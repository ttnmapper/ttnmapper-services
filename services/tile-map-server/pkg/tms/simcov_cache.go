package tms

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/patrickmn/go-cache"
	"ttnmapper-services/services/tile-map-server/pkg/tms/types"
)

// simCovCacheEntry is the on-disk representation of a per-gateway simulation result.
type simCovCacheEntry struct {
	Lat     float64
	Lon     float64
	Params  types.SimulationParams
	Samples []types.Sample
}

func simCovCacheFilePath(networkId, gatewayId string) string {
	return filepath.Join(
		MyConfiguration.CacheDirSimCov,
		url.QueryEscape(networkId),
		url.QueryEscape(gatewayId)+".gob",
	)
}

func simCovCacheTifPath(networkId, gatewayId string) string {
	return filepath.Join(
		MyConfiguration.CacheDirSimCov,
		url.QueryEscape(networkId),
		url.QueryEscape(gatewayId)+".tif",
	)
}

// simCovMetadata omits Samples so gob can skip that field without allocating
// the potentially huge slice. Repeated checks reuse metadata while the file's
// identity, modification time and size remain unchanged.
type simCovMetadata struct {
	Lat    float64
	Lon    float64
	Params types.SimulationParams
}

type cachedSimCovMetadata struct {
	metadata simCovMetadata
	info     os.FileInfo
}

var simCovMetadataCache = cache.New(5*time.Minute, time.Minute)

func loadSimCovMetadata(networkID, gatewayID string) (float64, float64, types.SimulationParams, bool) {
	path := simCovCacheFilePath(networkID, gatewayID)
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, types.SimulationParams{}, false
	}
	if value, ok := simCovMetadataCache.Get(path); ok {
		cached := value.(cachedSimCovMetadata)
		if os.SameFile(info, cached.info) && info.Size() == cached.info.Size() && info.ModTime().Equal(cached.info.ModTime()) {
			m := cached.metadata
			return m.Lat, m.Lon, m.Params, true
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, types.SimulationParams{}, false
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return 0, 0, types.SimulationParams{}, false
	}
	var metadata simCovMetadata
	if err := gob.NewDecoder(f).Decode(&metadata); err != nil {
		return 0, 0, types.SimulationParams{}, false
	}
	simCovMetadataCache.SetDefault(path, cachedSimCovMetadata{metadata, info})
	return metadata.Lat, metadata.Lon, metadata.Params, true
}

// writeSimCovArtifact writes to a temporary file and publishes it only once
// complete, so concurrent readers cannot decode a partially written artifact.
func writeSimCovArtifact(path string, write func(io.Writer) error) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".simcov-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// SaveSimCovCache persists simulation results for a gateway to disk.
func SaveSimCovCache(networkId, gatewayId string, samples []types.Sample, lat, lon float64, params types.SimulationParams, tifPath string) {
	filePath := simCovCacheFilePath(networkId, gatewayId)
	CreateDirIfNotExist(filepath.Dir(filePath))
	if tifPath != "" {
		err := writeSimCovArtifact(simCovCacheTifPath(networkId, gatewayId), func(w io.Writer) error {
			f, err := os.Open(tifPath)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(w, f)
			return err
		})
		if err != nil {
			log.Printf("simcov_cache: failed to copy tif for %s/%s: %v", networkId, gatewayId, err)
			return
		}
	}
	entry := simCovCacheEntry{Lat: lat, Lon: lon, Params: params, Samples: samples}
	if err := writeSimCovArtifact(filePath, func(w io.Writer) error { return gob.NewEncoder(w).Encode(entry) }); err != nil {
		log.Printf("simcov_cache: failed to encode cache for %s/%s: %v", networkId, gatewayId, err)
		return
	}
	if info, err := os.Stat(filePath); err == nil {
		simCovMetadataCache.SetDefault(filePath, cachedSimCovMetadata{simCovMetadata{lat, lon, params}, info})
	}
}

// LoadSimCovCache loads previously saved simulation results for a gateway.
// Returns ok=false if no cache file exists.
func LoadSimCovCache(networkId, gatewayId string) (samples []types.Sample, lat, lon float64, params types.SimulationParams, ok bool) {
	filePath := simCovCacheFilePath(networkId, gatewayId)

	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, 0, types.SimulationParams{}, false
	}
	defer f.Close()

	var entry simCovCacheEntry
	if err := gob.NewDecoder(f).Decode(&entry); err != nil {
		log.Printf("simcov_cache: failed to decode cache for %s/%s: %v", networkId, gatewayId, err)
		return nil, 0, 0, types.SimulationParams{}, false
	}

	return entry.Samples, entry.Lat, entry.Lon, entry.Params, true
}

// IsSimCovCacheStale returns true when the gateway has moved more than ~10 m
// since the cache was written (compares stored lat/lon against current values),
// or when the simulation parameters have changed.
func IsSimCovCacheStale(networkId, gatewayId string, cachedLat, cachedLon float64, cachedParams types.SimulationParams, currentLat, currentLon float64, currentParams types.SimulationParams) bool {
	if haversineDistanceM(cachedLat, cachedLon, currentLat, currentLon) > 10.0 {
		log.Printf("SimCov: cache for %s/%s is stale due to location change (>10m)", networkId, gatewayId)
		return true
	}
	// Check if parameters changed. We ignore TriesDone and LastRMSE.
	if cachedParams.Climate != currentParams.Climate ||
		cachedParams.Clutter != currentParams.Clutter ||
		cachedParams.Conductivity != currentParams.Conductivity ||
		cachedParams.Dielectric != currentParams.Dielectric ||
		cachedParams.Reliability != currentParams.Reliability ||
		cachedParams.ConstantLoss != currentParams.ConstantLoss ||
		cachedParams.HeightAgl != currentParams.HeightAgl ||
		math.Abs(cachedParams.Lat-currentParams.Lat) > 0.000001 ||
		math.Abs(cachedParams.Lon-currentParams.Lon) > 0.000001 {
		log.Printf("SimCov: cache for %s/%s is stale due to parameter change", networkId, gatewayId)
		return true
	}
	return false
}

// haversineDistanceM returns the great-circle distance in metres between two
// geographic coordinates.
func haversineDistanceM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6_371_000.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// DeleteSimCovCache removes the on-disk cache file for a gateway.
func DeleteSimCovCache(networkId, gatewayId string) error {
	filePath := simCovCacheFilePath(networkId, gatewayId)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("simcov_cache: failed to delete cache file %s: %w", filePath, err)
	}
	return nil
}
