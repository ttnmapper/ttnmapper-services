package tms

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"ttnmapper-tms/pkg/tms/types"
)

type rasterMetadata struct {
	Lat      float64
	Lon      float64
	Params   types.SimulationParams
	RadiusKm int
	Size     int64
	Modified time.Time
}

func rasterMetadataPath(networkID, gatewayID string) string {
	return simCovCacheTifPath(networkID, gatewayID) + ".json"
}

func ensureGatewayRaster(networkID, gatewayID string, lat, lon float64, alt int) error {
	lock := gatewayEnsureLock(networkID + "\x00" + gatewayID)
	lock.Lock()
	defer lock.Unlock()
	_, err := ensureGatewayRasterLocked(networkID, gatewayID, lat, lon, alt)
	return err
}

// Called with the gateway lock held. Both raster and legacy tile requests share
// the simulation; only legacy consumers subsequently construct sample cells.
func ensureGatewayRasterLocked(networkID, gatewayID string, lat, lon float64, alt int) (rasterMetadata, error) {
	params := getSimulationParams(lat, lon, networkID, gatewayID, alt)
	path := simCovCacheTifPath(networkID, gatewayID)
	info, statErr := os.Stat(path)
	var metadata rasterMetadata
	data, metaErr := os.ReadFile(rasterMetadataPath(networkID, gatewayID))
	if metaErr == nil {
		metaErr = json.Unmarshal(data, &metadata)
	} else if os.IsNotExist(metaErr) && statErr == nil {
		// Adopt existing simulations without recomputing them.
		if cachedLat, cachedLon, cachedParams, ok := loadSimCovMetadata(networkID, gatewayID); ok {
			metadata = rasterMetadata{cachedLat, cachedLon, cachedParams, MyConfiguration.SimCovRadiusKm, info.Size(), info.ModTime()}
			metaErr = nil
		}
	}
	if statErr == nil && metaErr == nil && metadata.Size == info.Size() && metadata.Modified.Equal(info.ModTime()) &&
		metadata.RadiusKm == MyConfiguration.SimCovRadiusKm && !IsSimCovCacheStale(networkID, gatewayID, metadata.Lat, metadata.Lon, metadata.Params, params.Lat, params.Lon, params) {
		return metadata, nil
	}
	tif, err := runSignalServerRasterWithParams(networkID, gatewayID, params.Lat, params.Lon, alt, MyConfiguration.SimCovRadiusKm, params)
	if err != nil {
		return rasterMetadata{}, err
	}
	defer os.RemoveAll(filepath.Dir(tif))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return rasterMetadata{}, err
	}
	if err := writeSimCovArtifact(path, func(w io.Writer) error {
		f, err := os.Open(tif)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}); err != nil {
		return rasterMetadata{}, err
	}
	info, err = os.Stat(path)
	if err != nil {
		return rasterMetadata{}, err
	}
	metadata = rasterMetadata{params.Lat, params.Lon, params, MyConfiguration.SimCovRadiusKm, info.Size(), info.ModTime()}
	if err := DeleteSimCovCache(networkID, gatewayID); err != nil {
		return rasterMetadata{}, err
	}
	simCovMetadataCache.Delete(simCovCacheFilePath(networkID, gatewayID))
	if err := writeSimCovArtifact(rasterMetadataPath(networkID, gatewayID), func(w io.Writer) error { return json.NewEncoder(w).Encode(metadata) }); err != nil {
		return rasterMetadata{}, err
	}
	return metadata, nil
}

// GetSimCovGatewayRaster delivers a complete native-grid GeoTIFF without
// projecting, colorizing, composing or expanding it into z19 samples.
func GetSimCovGatewayRaster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	networkID, _ := url.QueryUnescape(vars["network_id"])
	gatewayID, _ := url.QueryUnescape(vars["gateway_id"])
	lat, lon, alt, err := GetGatewayLocation(networkID, gatewayID)
	if err != nil || (lat == 0 && lon == 0) {
		http.Error(w, "gateway location unavailable", http.StatusNotFound)
		return
	}
	lock := gatewayEnsureLock(networkID + "\x00" + gatewayID)
	lock.Lock()
	if r.Context().Err() != nil {
		lock.Unlock()
		return
	}
	metadata, err := ensureGatewayRasterLocked(networkID, gatewayID, lat, lon, alt)
	if err != nil {
		lock.Unlock()
		log.Printf("SimCov raster %s/%s: %v", networkID, gatewayID, err)
		http.Error(w, "coverage simulation failed", http.StatusInternalServerError)
		return
	}
	// Open while holding the lock; ServeContent retains this generation even if
	// another request later publishes a replacement. It supports byte ranges.
	f, err := os.Open(simCovCacheTifPath(networkID, gatewayID))
	lock.Unlock()
	if err != nil {
		http.Error(w, "coverage raster unavailable", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	serveSimCovRaster(w, r, metadata, f)
}

func serveSimCovRaster(w http.ResponseWriter, r *http.Request, metadata rasterMetadata, f *os.File) {
	w.Header().Set("Content-Type", "image/tiff")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", fmt.Sprintf("\"%x-%x\"", metadata.Modified.UnixNano(), metadata.Size))
	http.ServeContent(w, r, "coverage.tif", metadata.Modified, f)
}

type coverageBounds struct{ West, South, East, North float64 }

func parseCoverageBounds(r *http.Request) (coverageBounds, error) {
	var bounds coverageBounds
	for name, target := range map[string]*float64{"west": &bounds.West, "south": &bounds.South, "east": &bounds.East, "north": &bounds.North} {
		value, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return bounds, fmt.Errorf("%s invalid", name)
		}
		*target = value
	}
	if bounds.South < -85.051129 || bounds.North > 85.051129 || bounds.South >= bounds.North || bounds.West < -180 || bounds.West > 180 || bounds.East < -180 || bounds.East > 180 || bounds.West == bounds.East {
		return bounds, fmt.Errorf("bounds invalid")
	}
	return bounds, nil
}

func gatewayIntersectsCoverageBounds(lat, lon float64, bounds coverageBounds, radiusM float64) bool {
	if bounds.West > bounds.East {
		return gatewayIntersectsCoverageBounds(lat, lon, coverageBounds{bounds.West, bounds.South, 180, bounds.North}, radiusM) || gatewayIntersectsCoverageBounds(lat, lon, coverageBounds{-180, bounds.South, bounds.East, bounds.North}, radiusM)
	}
	closestLat := max(bounds.South, min(bounds.North, lat))
	if lon >= bounds.West && lon <= bounds.East {
		return haversineDistanceM(lat, lon, closestLat, lon) <= radiusM
	}
	// Longitude is circular: a gateway just across the dateline may be nearer
	// the east edge even though its numeric longitude is less than the west edge.
	return min(haversineDistanceM(lat, lon, closestLat, bounds.West), haversineDistanceM(lat, lon, closestLat, bounds.East)) <= radiusM
}

type coverageRasterDescriptor struct {
	GatewayID string `json:"gateway_id"`
	URL       string `json:"url"`
	Cached    bool   `json:"cached"`
}

// This manifest performs no simulations. The client requests each raster
// independently, so one slow gateway never holds up other completed coverage.
func GetSimCovRasterManifest(w http.ResponseWriter, r *http.Request) {
	bounds, err := parseCoverageBounds(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	networkID, _ := url.QueryUnescape(mux.Vars(r)["network_id"])
	gateways, err := GetNetworkGateways(networkID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	descriptors := selectCoverageRasters(networkID, gateways, bounds, r.URL.Query().Get("gateway_id"))
	limit := MyConfiguration.SimCovMaxRasterGateways
	if limit <= 0 {
		limit = 128
	}
	if len(descriptors) > limit {
		http.Error(w, fmt.Sprintf("%d gateways intersect this view (limit %d); zoom in", len(descriptors), limit), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(struct {
		Gateways []coverageRasterDescriptor `json:"gateways"`
	}{descriptors})
}

func selectCoverageRasters(networkID string, gateways []types.Gateway, bounds coverageBounds, selectedGatewayID string) []coverageRasterDescriptor {
	descriptors := make([]coverageRasterDescriptor, 0)
	selected := selectedGatewayID
	cutoff := time.Now().AddDate(0, 0, -5)
	for _, gw := range gateways {
		if (selected != "" && gw.GatewayId != selected) || (selected == "" && gw.LastHeard.Before(cutoff)) || (gw.Latitude == 0 && gw.Longitude == 0) {
			continue
		}
		params := getGatewayParams(networkID, gw.GatewayId, gw.Latitude, gw.Longitude, int(gw.Altitude))
		if !gatewayIntersectsCoverageBounds(params.Lat, params.Lon, bounds, float64(MyConfiguration.SimCovRadiusKm)*1000) {
			continue
		}
		inputs, _ := json.Marshal(struct {
			Params   types.SimulationParams
			Altitude int32
			Radius   int
		}{params, gw.Altitude, MyConfiguration.SimCovRadiusKm})
		version := sha256.Sum256(inputs)
		_, rasterErr := os.Stat(simCovCacheTifPath(networkID, gw.GatewayId))
		descriptors = append(descriptors, coverageRasterDescriptor{GatewayID: gw.GatewayId, URL: fmt.Sprintf("/simcov/rasters/gateway/%s/%s?v=%x", url.QueryEscape(networkID), url.QueryEscape(gw.GatewayId), version[:12]), Cached: rasterErr == nil})
	}
	return descriptors
}
