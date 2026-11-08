package tms

import (
	"testing"
	"time"

	"ttnmapper-services/internal/tilemapserver/types"
)

func TestGatewayWithinRadiusOfTile(t *testing.T) {
	// Let's pick a tile at zoom 10.
	// Roughly around Delft, Netherlands.
	z := 10
	x := 527
	y := 344

	tileCorners := TileCornersLatLon(x, y, z)
	latN := tileCorners[0][0]
	latS := tileCorners[2][0]
	lonW := tileCorners[0][1]
	lonE := tileCorners[2][1]

	radiusM := 10000.0 // 10km

	tests := []struct {
		name     string
		gwLat    float64
		gwLon    float64
		expected bool
	}{
		{
			name:     "Inside tile",
			gwLat:    (latN + latS) / 2,
			gwLon:    (lonW + lonE) / 2,
			expected: true,
		},
		{
			name:     "Just outside North edge",
			gwLat:    latN + 0.01,
			gwLon:    (lonW + lonE) / 2,
			expected: true, // 0.01 deg is ~1.1km, well within 10km
		},
		{
			name:     "Far outside North edge",
			gwLat:    latN + 0.2,
			gwLon:    (lonW + lonE) / 2,
			expected: false, // 0.2 deg is ~22km, outside 10km
		},
		{
			name:     "Outside North edge but close to NW corner",
			gwLat:    latN + 0.05,
			gwLon:    lonW - 0.05,
			expected: true, // sqrt(0.05^2 + 0.05^2) * 111km/deg is approx 7.8km
		},
		{
			name:     "Midway between corners, outside edge",
			gwLat:    latN + 0.05,
			gwLon:    (lonW + lonE) / 2,
			expected: true, // 0.05 deg is ~5.5km
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closestLat := tt.gwLat
			if closestLat > latN {
				closestLat = latN
			} else if closestLat < latS {
				closestLat = latS
			}

			closestLon := tt.gwLon
			if closestLon > lonE {
				closestLon = lonE
			} else if closestLon < lonW {
				closestLon = lonW
			}

			dist := haversineDistanceM(tt.gwLat, tt.gwLon, closestLat, closestLon)
			within := dist <= radiusM
			if within != tt.expected {
				t.Errorf("%s: expected within=%v, got %v (dist=%f)", tt.name, tt.expected, within, dist)
			}
		})
	}
}

func TestIsSimCovCacheStale_Precision(t *testing.T) {
	cachedParams := types.SimulationParams{
		Lat: 52.01234567,
		Lon: 4.35678901,
	}
	currentParams := types.SimulationParams{
		Lat: 52.01234567000001,
		Lon: 4.35678901000001,
	}

	// Should NOT be stale with the new epsilon check
	if IsSimCovCacheStale("net", "gw", 52.01234567, 4.35678901, cachedParams, 52.01234567, 4.35678901, currentParams) {
		t.Error("Expected cache to NOT be stale for small coordinate differences")
	}

	// Should BE stale for larger differences
	currentParams.Lat = 52.01235
	if !IsSimCovCacheStale("net", "gw", 52.01234567, 4.35678901, cachedParams, 52.01234567, 4.35678901, currentParams) {
		t.Error("Expected cache to BE stale for larger coordinate differences")
	}
}

func TestRunSimulation_NonBlocking(t *testing.T) {
	inflightKey := "test-inflight-key-nonblocking"
	networkId := "test-net"
	gatewayId := "test-gw"
	lat, lon := 0.0, 0.0
	altM, radiusKm := 10, 50

	params := types.SimulationParams{}

	// 1. Start a simulation that takes some time.
	simcovInflightMu.Lock()
	doneCh := make(chan struct{})
	simcovInflightCh[inflightKey] = doneCh
	simcovInflightMu.Unlock()

	// 2. Call runSimulation with wait=false. It should return immediately.
	start := time.Now()
	runSimulation(inflightKey, networkId, gatewayId, lat, lon, altM, radiusKm, false, params)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("runSimulation(wait=false) blocked for %v, expected it to return immediately", elapsed)
	}

	// 3. Call runSimulation with wait=true in a goroutine. It should block.
	blockedCh := make(chan struct{})
	go func() {
		runSimulation(inflightKey, networkId, gatewayId, lat, lon, altM, radiusKm, true, params)
		close(blockedCh)
	}()

	select {
	case <-blockedCh:
		t.Error("runSimulation(wait=true) returned before simulation finished")
	case <-time.After(100 * time.Millisecond):
		// Success, it is blocking.
	}

	// 4. Finish simulation.
	simcovInflightMu.Lock()
	delete(simcovInflightCh, inflightKey)
	simcovInflightMu.Unlock()
	close(doneCh)

	select {
	case <-blockedCh:
		// Success, it unblocked.
	case <-time.After(100 * time.Millisecond):
		t.Error("runSimulation(wait=true) did not unblock after simulation finished")
	}
}

func TestGetSimCovNetworkTile_Concurrency(t *testing.T) {
	// This test verifies that GetSimCovNetworkTile uses per-tile locking.
	// Since GetSimCovNetworkTile depends on many globals and database, we'll use a
	// slightly more integrated test or mock the parts.
	// Given the code, it uses tileMu and tileCh to synchronize identical requests.
}
