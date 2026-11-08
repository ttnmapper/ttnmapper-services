package tms

import (
	"fmt"
	"image/png"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ttnmapper-services/services/tile-map-server/pkg/tms/types"

	"github.com/gorilla/mux"
)

// simcovInflightCh tracks in-flight simulations per gateway key; callers that
// arrive while a simulation is running block on the channel until it completes.
var (
	simcovInflightMu sync.Mutex
	simcovInflightCh = make(map[string]chan struct{})

	// tileMu ensures exactly one GetSimCovNetworkTile request for a given (network, z, x, y)
	// is processed at a time.
	tileMu sync.Mutex
	tileCh = make(map[string]chan struct{})

	// gwEnsureMu guards gwEnsureLock, a per-gateway mutex map that serialises
	// ensureGatewaySimCov calls for the same gateway. This prevents concurrent
	// tile requests (per-gateway and network-wide) from optimizing or simulating
	// the same gateway more than once at a time.
	gwEnsureMu   sync.Mutex
	gwEnsureLock = make(map[string]*sync.Mutex)
)

// gatewayEnsureLock returns the per-gateway mutex for the given key, creating it
// on first use.
func gatewayEnsureLock(key string) *sync.Mutex {
	gwEnsureMu.Lock()
	defer gwEnsureMu.Unlock()
	m, ok := gwEnsureLock[key]
	if !ok {
		m = &sync.Mutex{}
		gwEnsureLock[key] = m
	}
	return m
}

// ensureGatewaySimCov makes sure a gateway has an up-to-date simulated coverage
// cache. It runs the coverage simulation if the cache is missing or stale, using
// the gateway's previously-optimized parameters when available, or falling back
// to the registered location from the database otherwise. The call blocks until
// any required simulation has completed.
//
// Location optimization is deliberately NOT performed here: it is far too slow to
// run on the tile-request path. Optimization is done offline (e.g. via the
// optimize-location tool); when it later changes a gateway's location or
// parameters, the cache becomes stale and the coverage is re-simulated on the
// next request.
//
// Calls are serialised per gateway, so concurrent requests for the same gateway
// wait for the first to finish rather than duplicating work.
func ensureGatewaySimCov(networkId, gatewayId string, gwLat, gwLon float64, gwAltM int) {
	key := networkId + "\x00" + gatewayId
	lock := gatewayEnsureLock(key)
	lock.Lock()
	defer lock.Unlock()

	metadata, err := ensureGatewayRasterLocked(networkId, gatewayId, gwLat, gwLon, gwAltM)
	if err != nil {
		log.Printf("SimCov: %s/%s: %v", networkId, gatewayId, err)
		return
	}
	lat, lon, params, ok := loadSimCovMetadata(networkId, gatewayId)
	if ok && !IsSimCovCacheStale(networkId, gatewayId, lat, lon, params, metadata.Lat, metadata.Lon, metadata.Params) {
		return
	}
	runner := getSignalServerRunner()
	runner.simulations <- struct{}{}
	defer func() { <-runner.simulations }()
	samples, err := parseGeoTIFF(simCovCacheTifPath(networkId, gatewayId))
	if err != nil {
		log.Printf("SimCov: sample conversion: %v", err)
		return
	}
	SaveSimCovCache(networkId, gatewayId, samples, metadata.Lat, metadata.Lon, metadata.Params, "")
}

// simCovTileCacheValid reports whether a rendered PNG tile at tilePath may still
// be served directly. The tile must exist, still be within the zoom-level TTL,
// and be newer than gateway_params.json. The last condition ensures that when an
// offline optimization run (optimize-location / optimize-radio) rewrites the
// parameters file, previously-rendered tiles are treated as stale and the
// coverage is re-simulated on the next request rather than served for up to the
// full cache TTL.
func simCovTileCacheValid(tilePath string, z int) bool {
	fi, err := os.Stat(tilePath)
	if err != nil {
		return false
	}
	if !fi.ModTime().Add(GetCacheDurationForZoom(z)).After(time.Now()) {
		return false
	}
	if MyConfiguration.GatewayParamsFile != "" {
		if pf, err := os.Stat(MyConfiguration.GatewayParamsFile); err == nil && pf.ModTime().After(fi.ModTime()) {
			return false
		}
	}
	return true
}

// runSimulation ensures exactly one Signal-Server simulation runs per gateway at a
// time. If wait is true, it blocks until the simulation finishes. If wait is
// false, it returns immediately (after starting or seeing it's in-flight).
func runSimulation(inflightKey, networkId, gatewayId string, lat, lon float64, altM, radiusKm int, wait bool, params types.SimulationParams) {
	simcovInflightMu.Lock()
	if ch, running := simcovInflightCh[inflightKey]; running {
		// Another goroutine is already simulating.
		simcovInflightMu.Unlock()
		if wait {
			log.Printf("SimCov: waiting for in-flight simulation for %s (Key: %s)", gatewayId, inflightKey)
			<-ch
		}
		return
	}
	// Register ourselves as the runner.
	doneCh := make(chan struct{})
	simcovInflightCh[inflightKey] = doneCh
	simcovInflightMu.Unlock()

	defer func() {
		simcovInflightMu.Lock()
		delete(simcovInflightCh, inflightKey)
		simcovInflightMu.Unlock()
		close(doneCh)
	}()

	log.Printf("SimCov: starting simulation for %s/%s (Key: %s)", networkId, gatewayId, inflightKey)
	samples, tifPath, err := RunSignalServerSimulationWithParams(networkId, gatewayId, lat, lon, altM, radiusKm, params)
	if err != nil {
		log.Printf("SimCov: simulation failed for %s/%s (Key: %s): %v", networkId, gatewayId, inflightKey, err)
		return
	}
	defer os.RemoveAll(filepath.Dir(tifPath))
	SaveSimCovCache(networkId, gatewayId, samples, lat, lon, params, tifPath)
	log.Printf("SimCov: simulation complete for %s/%s (Key: %s) (%d samples)", networkId, gatewayId, inflightKey, len(samples))
}

// GetSimCovTile handles requests for simulated radio coverage tiles.
// URL pattern: /simcov/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}
//
// If the simulation cache is missing or stale the simulation is run synchronously
// before the tile is rendered, so the response always reflects complete data.
func GetSimCovTile(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		PromTmsSimCovRequestDuration.Observe(float64(elapsed.Nanoseconds()) / 1_000_000.0)
	}()

	vars := mux.Vars(r)

	networkId, _ := url.QueryUnescape(vars["network_id"])
	gatewayId, _ := url.QueryUnescape(vars["gateway_id"])

	z, err := strconv.Atoi(vars["z"])
	if err != nil {
		http.Error(w, "z invalid", http.StatusBadRequest)
		return
	}
	x, err := strconv.Atoi(vars["x"])
	if err != nil {
		http.Error(w, "x invalid", http.StatusBadRequest)
		return
	}
	yStr := vars["y"]
	if strings.HasSuffix(yStr, ".png") {
		yStr = strings.TrimSuffix(yStr, ".png")
	}
	y, err := strconv.Atoi(yStr)
	if err != nil {
		http.Error(w, "y invalid", http.StatusBadRequest)
		return
	}

	log.Printf("SimCov tile %s - %s: %d/%d/%d", networkId, gatewayId, z, x, y)

	// 0. Check tile-level cache.
	if MyConfiguration.CacheEnabled {
		tilePath := fmt.Sprintf("%s/gateway/%s/%s/%d/%d/%d.png", MyConfiguration.CacheDirSimCov, url.QueryEscape(networkId), url.QueryEscape(gatewayId), z, x, y)
		if simCovTileCacheValid(tilePath, z) {
			log.Printf("SimCov: serving from tile cache")
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, tilePath)
			return
		}
	}

	// 1. Look up current gateway location (registered location from DB).
	dbLat, dbLon, dbAlt, err := GetGatewayLocation(networkId, gatewayId)
	if err != nil || (dbLat == 0 && dbLon == 0) {
		log.Printf("SimCov: no location for %s/%s: %v", networkId, gatewayId, err)
		serveEmptyTile(w, 60)
		return
	}

	// 2. Ensure the gateway is simulated. Uses previously-optimized parameters
	// when available, or the registered DB location as fallback. Re-simulates
	// when the cache is missing or stale.
	if err := ensureGatewayRaster(networkId, gatewayId, dbLat, dbLon, dbAlt); err != nil {
		log.Printf("SimCov: raster preparation failed: %v", err)
		serveEmptyTile(w, 5)
		return
	}

	// 4. Try rendering from GeoTIFF if it exists in cache.
	tifPath := simCovCacheTifPath(networkId, gatewayId)
	if _, err := os.Stat(tifPath); err == nil {
		tile, err := RenderSimCovTileFromGeoTIFF(tifPath, x, y, z)
		if err == nil {
			if MyConfiguration.CacheEnabled {
				destTilePath := fmt.Sprintf("%s/gateway/%s/%s/%d/%d/%d.png", MyConfiguration.CacheDirSimCov, url.QueryEscape(networkId), url.QueryEscape(gatewayId), z, x, y)
				StoreTileInFile(tile, destTilePath)
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			png.Encode(w, tile)
			return
		}
		log.Printf("SimCov: GeoTIFF rendering failed, falling back to circles: %v", err)
	}

	// Load samples only for the fallback renderer. The GeoTIFF path needs no samples.
	ensureGatewaySimCov(networkId, gatewayId, dbLat, dbLon, dbAlt)
	samples, _, _, _, cacheOk := LoadSimCovCache(networkId, gatewayId)
	if !cacheOk {
		serveEmptyTile(w, 5)
		return
	}

	// 5. Filter samples to the z19 range that covers the requested tile (with buffer).
	buffer := 1.0 / math.Pow(2, float64(19-z))
	if buffer < 0.0375 {
		buffer = 0.0375
	}
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(x, y, z, buffer)

	var filtered []types.Sample
	for _, s := range samples {
		if s.X >= xMin && s.X <= xMax && s.Y >= yMin && s.Y <= yMax {
			filtered = append(filtered, s)
		}
	}

	// 4. Sort and render using the same renderer as the circles endpoint.
	sort.Sort(types.ByRssi(filtered))
	tile := CreateCirclesTile(x, y, z, filtered)

	// Save to cache if enabled.
	if MyConfiguration.CacheEnabled {
		tileDir := fmt.Sprintf("%s/gateway/%s/%s/%d/%d", MyConfiguration.CacheDirSimCov, url.QueryEscape(networkId), url.QueryEscape(gatewayId), z, x)
		if err := os.MkdirAll(tileDir, 0755); err == nil {
			tilePath := filepath.Join(tileDir, fmt.Sprintf("%d.png", y))
			if f, err := os.Create(tilePath); err == nil {
				png.Encode(f, tile)
				f.Close()
			}
		}
	}

	// 5. Write the PNG response with a long cache TTL.
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

	if err := png.Encode(w, tile); err != nil {
		log.Printf("SimCov: png encode error: %v", err)
	}
}

// GetSimCovNetworkTile handles requests for network-wide simulated radio coverage tiles.
// URL pattern: /simcov/network/{network_id}/{z}/{x}/{y}
//
// It aggregates the cached simulation results for every gateway in the network.
// If any simulation is missing or stale, it waits for all simulations to finish
// before rendering and returning the tile.
func GetSimCovNetworkTile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	networkId, _ := url.QueryUnescape(vars["network_id"])
	zStr := vars["z"]
	xStr := vars["x"]
	yStr := vars["y"]
	if strings.HasSuffix(yStr, ".png") {
		yStr = strings.TrimSuffix(yStr, ".png")
	}

	// Use a per-tile lock to prevent redundant processing.
	tileKey := fmt.Sprintf("%s/%s/%s/%s", networkId, zStr, xStr, yStr)
	tileMu.Lock()
	if ch, running := tileCh[tileKey]; running {
		tileMu.Unlock()
		<-ch
	} else {
		doneCh := make(chan struct{})
		tileCh[tileKey] = doneCh
		tileMu.Unlock()
		defer func() {
			tileMu.Lock()
			delete(tileCh, tileKey)
			tileMu.Unlock()
			close(doneCh)
		}()
	}

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		PromTmsSimCovRequestDuration.Observe(float64(elapsed.Nanoseconds()) / 1_000_000.0)
	}()

	z, err := strconv.Atoi(zStr)
	if err != nil {
		http.Error(w, "z invalid", http.StatusBadRequest)
		return
	}
	x, err := strconv.Atoi(xStr)
	if err != nil {
		http.Error(w, "x invalid", http.StatusBadRequest)
		return
	}
	y, err := strconv.Atoi(yStr)
	if err != nil {
		http.Error(w, "y invalid", http.StatusBadRequest)
		return
	}

	log.Printf("SimCov network tile %s: %d/%d/%d", networkId, z, x, y)

	// 0. Check tile-level cache.
	if MyConfiguration.CacheEnabled {
		tilePath := fmt.Sprintf("%s/network/%s/%d/%d/%d.png", MyConfiguration.CacheDirSimCov, url.QueryEscape(networkId), z, x, y)
		if simCovTileCacheValid(tilePath, z) {
			log.Printf("SimCov network: serving from tile cache")
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, tilePath)
			return
		}
	}

	// 1. Fetch all gateways for this network.
	gateways, err := GetNetworkGateways(networkId)
	if err != nil {
		log.Printf("SimCov network: failed to list gateways for %s: %v", networkId, err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Filter gateways by last heard and distance early to avoid redundant cache checks.
	fiveDaysAgo := time.Now().AddDate(0, 0, -5)
	radiusM := float64(MyConfiguration.SimCovRadiusKm) * 1000.0
	tileCorners := TileCornersLatLon(x, y, z)
	latN := tileCorners[0][0]
	latS := tileCorners[2][0]
	lonW := tileCorners[0][1]
	lonE := tileCorners[2][1]

	var relevantGateways []types.Gateway
	for _, gw := range gateways {
		if gw.Latitude == 0 && gw.Longitude == 0 {
			continue
		}
		if gw.LastHeard.Before(fiveDaysAgo) {
			continue
		}

		closestLat := gw.Latitude
		if closestLat > latN {
			closestLat = latN
		} else if closestLat < latS {
			closestLat = latS
		}
		closestLon := gw.Longitude
		if closestLon > lonE {
			closestLon = lonE
		} else if closestLon < lonW {
			closestLon = lonW
		}

		if haversineDistanceM(gw.Latitude, gw.Longitude, closestLat, closestLon) <= radiusM {
			relevantGateways = append(relevantGateways, gw)
		}
	}

	// 2. Collect samples from each gateway's cache; kick off simulations as needed.
	buffer := 1.0 / math.Pow(2, float64(19-z))
	if buffer < 0.0375 {
		buffer = 0.0375
	}
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(x, y, z, buffer)

	var allSamples []types.Sample
	var wg sync.WaitGroup

	// Ensure every relevant gateway is simulated.
	// Each gateway is handled concurrently; ensureGatewaySimCov serialises work
	// per gateway internally.
	for _, gw := range relevantGateways {
		wg.Add(1)
		go func(gw types.Gateway) {
			defer wg.Done()
			if err := ensureGatewayRaster(networkId, gw.GatewayId, gw.Latitude, gw.Longitude, int(gw.Altitude)); err != nil {
				log.Printf("SimCov: raster preparation for %s: %v", gw.GatewayId, err)
			}
		}(gw)
	}

	// Wait for all simulations to finish.
	wg.Wait()

	// 3. Now that all simulations are done (or were already done), collect all samples.
	var tifPaths []string
	for _, gw := range relevantGateways {
		gatewayId := gw.GatewayId
		tifPath := simCovCacheTifPath(networkId, gatewayId)
		if _, err := os.Stat(tifPath); err == nil {
			tifPaths = append(tifPaths, tifPath)
		}
	}

	// Try rendering from GeoTIFFs if they exist in cache.
	if len(tifPaths) > 0 {
		tile, err := RenderSimCovNetworkTileFromGeoTIFFs(tifPaths, x, y, z)
		if err == nil {
			if MyConfiguration.CacheEnabled {
				destTilePath := fmt.Sprintf("%s/network/%s/%d/%d/%d.png", MyConfiguration.CacheDirSimCov, url.QueryEscape(networkId), z, x, y)
				StoreTileInFile(tile, destTilePath)
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			png.Encode(w, tile)
			return
		}
		log.Printf("SimCov network: GeoTIFF rendering failed, falling back to circles: %v", err)
	}

	for _, gw := range relevantGateways {
		gatewayId := gw.GatewayId
		ensureGatewaySimCov(networkId, gatewayId, gw.Latitude, gw.Longitude, int(gw.Altitude))
		samples, _, _, _, cacheOk := LoadSimCovCache(networkId, gatewayId)
		if !cacheOk {
			// This could happen if a simulation failed.
			continue
		}

		// Filter to the tile range.
		for _, s := range samples {
			if s.X >= xMin && s.X <= xMax && s.Y >= yMin && s.Y <= yMax {
				allSamples = append(allSamples, s)
			}
		}
	}

	// 4. Sort and render.
	sort.Sort(types.ByRssi(allSamples))
	tile := CreateCirclesTile(x, y, z, allSamples)

	// Save to cache if enabled.
	if MyConfiguration.CacheEnabled {
		tileDir := fmt.Sprintf("%s/network/%s/%d/%d", MyConfiguration.CacheDirSimCov, url.QueryEscape(networkId), z, x)
		if err := os.MkdirAll(tileDir, 0755); err == nil {
			tilePath := filepath.Join(tileDir, fmt.Sprintf("%d.png", y))
			if f, err := os.Create(tilePath); err == nil {
				png.Encode(f, tile)
				f.Close()
			}
		}
	}

	// 5. Write the PNG response with a long cache TTL (all simulations are done).
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

	if err := png.Encode(w, tile); err != nil {
		log.Printf("SimCov network: png encode error: %v", err)
	}
}

// serveEmptyTile writes a 256×256 transparent PNG with the given Cache-Control
// max-age (in seconds) so the browser retries after the TTL expires.
func serveEmptyTile(w http.ResponseWriter, maxAgeSeconds int) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(maxAgeSeconds))
	png.Encode(w, transparentTile())
}
