package tms

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"

	"github.com/gorilla/mux"
)

type ComparisonPoint struct {
	Distance float64 `json:"distance"`
	Signal   int     `json:"signal"` // Bucket index
}

type AnalysisResponse struct {
	GatewayID       string   `json:"gateway_id"`
	CommonPoints    int      `json:"common_points"`
	MeanError       float64  `json:"mean_error_db"` // Positive means simulation is stronger than measured
	RMSE            float64  `json:"rmse_db"`
	Recommendations []string `json:"recommendations"`
}

type ComparisonResponse struct {
	GatewayID string            `json:"gateway_id"`
	Measured  []ComparisonPoint `json:"measured"`
	Simulated []ComparisonPoint `json:"simulated"`
}

func BucketIndexToDbm(idx int) float64 {
	if idx == 0 {
		return -97.5 // center of >-100
	}
	if idx >= 11 {
		return -152.5 // center of <-150
	}
	return -100.0 - float64(idx-1)*5.0 - 2.5
}

func GetComparisonData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	networkId, _ := url.QueryUnescape(vars["network_id"])
	gatewayId, _ := url.QueryUnescape(vars["gateway_id"])

	lat, lon, dbAlt, err := GetGatewayLocation(networkId, gatewayId)
	if err != nil {
		http.Error(w, "gateway not found", http.StatusNotFound)
		return
	}

	// Range for data: use simulation radius
	radiusKm := MyConfiguration.SimCovRadiusKm
	latDeg := float64(radiusKm) / 111.0
	lonDeg := float64(radiusKm) / (111.0 * math.Cos(lat*math.Pi/180.0))

	xMin, yMin := LatLonToZ19Tile(lat+latDeg, lon-lonDeg)
	xMax, yMax := LatLonToZ19Tile(lat-latDeg, lon+lonDeg)

	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}
	if yMin > yMax {
		yMin, yMax = yMax, yMin
	}

	// 1. Measured data
	measuredSamples, err := GetGatewaySamplesInRange(networkId, gatewayId, xMin, yMin, xMax, yMax)
	if err != nil {
		http.Error(w, "failed to fetch measured data", http.StatusInternalServerError)
		return
	}

	// 2. Simulated data. Route through ensureGatewaySimCov so this endpoint uses
	// exactly the same simulate/cache path as the tile handlers and does not
	// thrash the cache by disagreeing on the simulation center.
	ensureGatewaySimCov(networkId, gatewayId, lat, lon, dbAlt)
	simulatedSamples, _, _, _, ok := LoadSimCovCache(networkId, gatewayId)
	if !ok {
		log.Printf("Comparison: simulation failed or returned no data for %s/%s", networkId, gatewayId)
	} else {
		log.Printf("Comparison: loaded %d simulated samples", len(simulatedSamples))
	}

	response := ComparisonResponse{
		GatewayID: gatewayId,
		Measured:  make([]ComparisonPoint, 0, len(measuredSamples)),
		Simulated: make([]ComparisonPoint, 0, len(simulatedSamples)),
	}

	for _, s := range measuredSamples {
		if s.MaxBucketIndex == 12 {
			continue
		}
		sLat, sLon := TileCenterLatLon(s.X, s.Y, 19)
		dist := haversineDistanceM(lat, lon, sLat, sLon)
		response.Measured = append(response.Measured, ComparisonPoint{
			Distance: math.Round(dist*100) / 100,
			Signal:   s.MaxBucketIndex,
		})
	}

	for _, s := range simulatedSamples {
		if s.MaxBucketIndex == 12 {
			continue
		}
		sLat, sLon := TileCenterLatLon(s.X, s.Y, 19)
		dist := haversineDistanceM(lat, lon, sLat, sLon)
		response.Simulated = append(response.Simulated, ComparisonPoint{
			Distance: math.Round(dist*100) / 100,
			Signal:   s.MaxBucketIndex,
		})
	}

	// Sort by distance to make plotting easier if needed
	sort.Slice(response.Measured, func(i, j int) bool {
		return response.Measured[i].Distance < response.Measured[j].Distance
	})
	sort.Slice(response.Simulated, func(i, j int) bool {
		return response.Simulated[i].Distance < response.Simulated[j].Distance
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetAnalysisData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	networkId, _ := url.QueryUnescape(vars["network_id"])
	gatewayId, _ := url.QueryUnescape(vars["gateway_id"])

	lat, lon, dbAlt, err := GetGatewayLocation(networkId, gatewayId)
	if err != nil {
		http.Error(w, "gateway not found", http.StatusNotFound)
		return
	}

	// Range for data: use simulation radius
	radiusKm := MyConfiguration.SimCovRadiusKm
	latDeg := float64(radiusKm) / 111.0
	lonDeg := float64(radiusKm) / (111.0 * math.Cos(lat*math.Pi/180.0))

	xMin, yMin := LatLonToZ19Tile(lat+latDeg, lon-lonDeg)
	xMax, yMax := LatLonToZ19Tile(lat-latDeg, lon+lonDeg)

	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}
	if yMin > yMax {
		yMin, yMax = yMax, yMin
	}

	// 1. Measured data
	measuredSamples, err := GetGatewaySamplesInRange(networkId, gatewayId, xMin, yMin, xMax, yMax)
	if err != nil {
		http.Error(w, "failed to fetch measured data", http.StatusInternalServerError)
		return
	}

	// 2. Simulated data. Route through ensureGatewaySimCov for a consistent,
	// non-thrashing optimize/simulate/cache path (see GetComparisonData).
	ensureGatewaySimCov(networkId, gatewayId, lat, lon, dbAlt)
	simulatedSamples, _, _, _, ok := LoadSimCovCache(networkId, gatewayId)
	if !ok {
		log.Printf("Analysis: simulation failed or returned no data for %s/%s", networkId, gatewayId)
	}

	// Create a map for fast lookup of simulated samples by tile
	type tileKey struct{ X, Y int }
	simMap := make(map[tileKey]int)
	for _, s := range simulatedSamples {
		simMap[tileKey{s.X, s.Y}] = s.MaxBucketIndex
	}

	var totalError float64
	var totalSqError float64
	var count int

	for _, m := range measuredSamples {
		if m.MaxBucketIndex == 12 {
			continue
		}
		if simBucket, exists := simMap[tileKey{m.X, m.Y}]; exists && simBucket != 12 {
			mDbm := BucketIndexToDbm(m.MaxBucketIndex)
			sDbm := BucketIndexToDbm(simBucket)

			diff := sDbm - mDbm // Simulation - Measured
			totalError += diff
			totalSqError += diff * diff
			count++
		}
	}

	response := AnalysisResponse{
		GatewayID:    gatewayId,
		CommonPoints: count,
	}

	if count > 0 {
		response.MeanError = totalError / float64(count)
		response.RMSE = math.Sqrt(totalSqError / float64(count))

		// Diagnostic assessment only. Radio parameters are intentionally kept at
		// their defaults and the location is optimized offline, so these notes
		// describe the match quality rather than suggest manual parameter edits.
		if response.MeanError > 10 {
			response.Recommendations = append(response.Recommendations, "Simulation is significantly more optimistic than measurements (>10 dB bias).")
		} else if response.MeanError > 3 {
			response.Recommendations = append(response.Recommendations, "Simulation is slightly more optimistic than measurements.")
		} else if response.MeanError < -10 {
			response.Recommendations = append(response.Recommendations, "Simulation is significantly more pessimistic than measurements (>10 dB bias).")
		} else if response.MeanError < -3 {
			response.Recommendations = append(response.Recommendations, "Simulation is slightly more pessimistic than measurements.")
		} else {
			response.Recommendations = append(response.Recommendations, "Simulation signal levels match measurements well on average.")
		}

		if response.RMSE > 15 {
			response.Recommendations = append(response.Recommendations, "High RMSE indicates large local variations, common in urban/multipath environments.")
		}
	} else {
		response.Recommendations = append(response.Recommendations, "No overlapping measured and simulated points found for analysis.")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

