package tms

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// lookupGroup shares the result of concurrent identical lookups. Entries exist
// only while a lookup is running, including failed lookups.
type lookupCall struct {
	done  chan struct{}
	value any
	err   error
}

type lookupGroup struct {
	mu    sync.Mutex
	calls map[string]*lookupCall
}

func (group *lookupGroup) Do(key string, fn func() (any, error)) (any, error) {
	group.mu.Lock()
	if call, ok := group.calls[key]; ok {
		group.mu.Unlock()
		<-call.done
		return call.value, call.err
	}
	call := &lookupCall{done: make(chan struct{})}
	if group.calls == nil {
		group.calls = make(map[string]*lookupCall)
	}
	group.calls[key] = call
	group.mu.Unlock()
	defer func() {
		group.mu.Lock()
		delete(group.calls, key)
		close(call.done)
		group.mu.Unlock()
	}()
	call.value, call.err = fn()
	return call.value, call.err
}

var demHTTPClient = &http.Client{Timeout: 2 * time.Minute}
var demDownloads lookupGroup
var terrainQueries lookupGroup
var terrainElevations = cache.New(10*time.Minute, time.Minute)

func downloadCopernicusTile(demDir string, latTile, lonTile int) error {
	key := fmt.Sprintf("%s/%d/%d", demDir, latTile, lonTile)
	_, err := demDownloads.Do(key, func() (any, error) {
		return nil, downloadCopernicusTileOnce(demDir, latTile, lonTile)
	})
	return err
}

// Elevation is reused only while the underlying DEM tile is unchanged. Failed
// queries are never cached, allowing subsequent requests to retry downloads.
func queryTerrainElevation(lat, lon float64, demDir string) (float64, error) {
	latTile, lonTile := int(math.Floor(lat)), int(math.Floor(lon))
	latPrefix, lonPrefix := "N", "E"
	if latTile < 0 {
		latPrefix, latTile = "S", -latTile
	}
	if lonTile < 0 {
		lonPrefix, lonTile = "W", -lonTile
	}
	demPath := filepath.Join(demDir, fmt.Sprintf("Copernicus_DSM_COG_30_%s%02d_00_%s%03d_00_DEM.tif", latPrefix, latTile, lonPrefix, lonTile))
	info, statErr := os.Stat(demPath)
	key := fmt.Sprintf("%s/%s/%.6f/%.6f", MyConfiguration.SignalServerBin, demPath, lat, lon)
	if statErr == nil {
		key += fmt.Sprintf("/%d/%d", info.ModTime().UnixNano(), info.Size())
		if value, ok := terrainElevations.Get(key); ok {
			return value.(float64), nil
		}
	}
	value, err := terrainQueries.Do(key, func() (any, error) {
		if statErr == nil {
			if value, ok := terrainElevations.Get(key); ok {
				return value, nil
			}
		}
		elevation, err := queryTerrainElevationUncached(lat, lon, demDir)
		if err == nil && statErr == nil {
			terrainElevations.SetDefault(key, elevation)
		}
		return elevation, err
	})
	if err != nil {
		return 0, err
	}
	return value.(float64), nil
}
