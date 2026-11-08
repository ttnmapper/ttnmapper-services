package tms

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"ttnmapper-services/internal/tilemapserver/types"
)

// RunLocationOptimization performs location optimization for a gateway using
// radial reach matching, identical to the cmd/optimize-location binary.
// It runs maxTries iterations and saves results to gateway_params.json after
// each iteration. It is safe to call concurrently for different gateways.
func RunLocationOptimization(networkID, gatewayID string, maxTries int) error {

	dbLat, dbLon, dbAlt, err := GetGatewayLocation(networkID, gatewayID)
	if err != nil || (dbLat == 0 && dbLon == 0) {
		return fmt.Errorf("optimize-location: no location for %s/%s: %v", networkID, gatewayID, err)
	}

	params := types.GetGatewayParams(MyConfiguration.GatewayParamsFile, networkID, gatewayID, dbLat, dbLon, dbAlt)

	if params.LocationTriesDone == 0 && params.RadioTriesDone == 0 {
		params.HeightAgl = 10.0
		params.Clutter = 10.0
	}

	lat := params.Lat
	lon := params.Lon
	altM := dbAlt

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

	measuredSamples, err := GetGatewaySamplesInRange(networkID, gatewayID, xMin, yMin, xMax, yMax)
	if err != nil {
		return fmt.Errorf("optimize-location: failed to fetch measured data for %s/%s: %w", networkID, gatewayID, err)
	}
	if len(measuredSamples) == 0 {
		return fmt.Errorf("optimize-location: no measured data for %s/%s", networkID, gatewayID)
	}

	log.Printf("optimize-location: starting for %s/%s with %d measured samples, %d tries", networkID, gatewayID, len(measuredSamples), maxTries)

	currentParams := params

	losParams := currentParams
	losParams.ConstantLoss = 0
	losParams.Reliability = 50

	gatewayTileX, gatewayTileY := LatLonToZ19Tile(lat, lon)

	type radialReach struct {
		reaches [72]float64
	}

	getRadialReaches := func(samples []types.Sample, gX, gY int) map[int]radialReach {
		reachesPerBucket := make(map[int]radialReach)
		for _, s := range samples {
			if s.MaxBucketIndex == 12 {
				continue
			}
			dx := float64(s.X - gX)
			dy := float64(s.Y - gY)
			dist := math.Sqrt(dx*dx + dy*dy)
			angle := math.Atan2(dy, dx) * 180 / math.Pi
			if angle < 0 {
				angle += 360
			}
			sector := int(angle/5) % 72
			for b := s.MaxBucketIndex; b <= 11; b++ {
				r := reachesPerBucket[b]
				if dist > r.reaches[sector] {
					r.reaches[sector] = dist
					reachesPerBucket[b] = r
				}
			}
		}
		return reachesPerBucket
	}

	measuredReaches := getRadialReaches(measuredSamples, gatewayTileX, gatewayTileY)

	evaluateLOS := func(p types.SimulationParams) float64 {
		// Call WithParams directly to use the candidate parameters without
		// touching the shared gateway_params.json during evaluation.
		simSamples, tifPath, err := RunSignalServerSimulationWithParams(networkID, gatewayID, lat, lon, altM, radiusKm, p)
		if err != nil {
			return math.MaxFloat64
		}
		if tifPath != "" {
			defer os.RemoveAll(filepath.Dir(tifPath))
		}

		simGX, simGY := LatLonToZ19Tile(p.Lat, p.Lon)
		simReaches := getRadialReaches(simSamples, simGX, simGY)

		totalCost := 0.0
		for b := 0; b <= 11; b++ {
			mR, mExists := measuredReaches[b]
			sR, sExists := simReaches[b]
			if !mExists {
				continue
			}
			bucketCost := 0.0
			for i := range 72 {
				mReach := mR.reaches[i]
				sReach := 0.0
				if sExists {
					sReach = sR.reaches[i]
				}
				if mReach > 0 {
					diff := sReach - mReach
					if diff < 0 {
						bucketCost += math.Abs(diff) * 10.0
					} else {
						bucketCost += diff * 1.0
					}
				} else {
					bucketCost += sReach * 0.1
				}
			}
			totalCost += bucketCost
		}
		totalCost += p.HeightAgl * 1.0
		return totalCost
	}

	bestLat := currentParams.Lat
	bestLon := currentParams.Lon
	bestTxAlt := currentParams.HeightAgl
	bestClutter := currentParams.Clutter
	losParams.HeightAgl = bestTxAlt
	losParams.Clutter = bestClutter
	bestCost := evaluateLOS(losParams)

	maxRadiusDeg := 0.018
	totalTriesGoal := currentParams.LocationTriesDone + maxTries

	for try := range maxTries {
		currentTryNumber := currentParams.LocationTriesDone + try + 1
		progress := float64(currentTryNumber) / float64(totalTriesGoal)
		temp := math.Exp(-5.0 * progress)
		currentRadius := maxRadiusDeg * temp
		if currentRadius < 0.00001 {
			currentRadius = 0.00001
		}

		nextLOSParams := losParams
		if currentTryNumber%10 == 0 {
			nextLOSParams.Lat = lat + (rand.Float64()*2.0*maxRadiusDeg - maxRadiusDeg)
			nextLOSParams.Lon = lon + (rand.Float64()*2.0*maxRadiusDeg - maxRadiusDeg)
			nextLOSParams.HeightAgl = 0.5 + rand.Float64()*19.5
			nextLOSParams.Clutter = rand.Float64() * 30.0
		} else {
			nextLOSParams.Lat = bestLat + (rand.Float64()*2.0*currentRadius - currentRadius)
			nextLOSParams.Lon = bestLon + (rand.Float64()*2.0*currentRadius - currentRadius)
			nextLOSParams.HeightAgl = bestTxAlt + (rand.Float64()*4.0*temp - 2.0*temp)
			nextLOSParams.Clutter = bestClutter + (rand.Float64()*4.0*temp - 2.0*temp)
		}

		if nextLOSParams.HeightAgl < 0.5 {
			nextLOSParams.HeightAgl = 0.5
		}
		if nextLOSParams.HeightAgl > 40 {
			nextLOSParams.HeightAgl = 40
		}
		if nextLOSParams.Clutter < 0 {
			nextLOSParams.Clutter = 0
		}
		if nextLOSParams.Clutter > 30 {
			nextLOSParams.Clutter = 30
		}

		cost := evaluateLOS(nextLOSParams)
		if cost < bestCost {
			bestCost = cost
			bestLat = nextLOSParams.Lat
			bestLon = nextLOSParams.Lon
			bestTxAlt = nextLOSParams.HeightAgl
			bestClutter = nextLOSParams.Clutter
			currentParams.LocationBestRadius = currentRadius * 111000
		}

		currentParams.Lat = bestLat
		currentParams.Lon = bestLon
		currentParams.HeightAgl = bestTxAlt
		currentParams.Clutter = bestClutter
		currentParams.LocationLastRMSE = bestCost
		currentParams.LocationTriesDone = params.LocationTriesDone + try + 1
		types.SaveFinalParams(MyConfiguration.GatewayParamsFile, networkID, gatewayID, currentParams)
	}

	log.Printf("optimize-location: finished for %s/%s — best location: %.6f, %.6f, height: %.1fm, clutter: %.1f (cost: %.2f), total tries: %d",
		networkID, gatewayID, bestLat, bestLon, bestTxAlt, bestClutter, bestCost, currentParams.LocationTriesDone)

	return nil
}
