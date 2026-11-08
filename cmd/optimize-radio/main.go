package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
	"ttnmapper-services/internal/tilemapserver"
	"ttnmapper-services/internal/tilemapserver/types"

	"strconv"
	"ttnmapper-services/pkg/database"
)

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	gatewayID := flag.String("gateway", "", "Gateway ID (required)")
	networkID := flag.String("network", "NS_TTS_V3://ttn@000013", "Network ID")
	maxTries := flag.Int("tries", 100, "Maximum number of optimization tries")
	flag.Parse()

	if *gatewayID == "" {
		fmt.Println("Error: -gateway is required")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	err := tms.LoadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	tms.DB, err = database.OpenPostgres(database.PostgresOptions{Host: tms.MyConfiguration.PostgresHost, Port: strconv.Itoa(tms.MyConfiguration.PostgresPort), User: tms.MyConfiguration.PostgresUser, Password: tms.MyConfiguration.PostgresPassword, Database: tms.MyConfiguration.PostgresDatabase, ApplicationName: "optimize-radio"})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Get gateway location (original registered location from DB)
	dbLat, dbLon, dbAlt, err := tms.GetGatewayLocation(*networkID, *gatewayID)
	if err != nil {
		log.Fatalf("Gateway not found in database: %v", err)
	}

	// Get initial parameters from gateway_params.json
	// If missing, GetGatewayParams will use the provided DB values as defaults.
	params := types.GetGatewayParams(tms.MyConfiguration.GatewayParamsFile, *networkID, *gatewayID, dbLat, dbLon, dbAlt)

	// Initial run detected. Use provided defaults.
	if params.LocationTriesDone == 0 && params.RadioTriesDone == 0 {
		// Start with HeightAgl = 2.0 or convert from DB ASL.
		// GetGatewayParams already handled this.
	}

	fmt.Printf("Starting radio parameter optimization for gateway %s\n", *gatewayID)
	fmt.Printf("Using location: %.6f, %.6f, Height: %.1fm\n", params.Lat, params.Lon, params.HeightAgl)

	// Get measured data around the location we are using
	radiusKm := tms.MyConfiguration.SimCovRadiusKm
	latDeg := float64(radiusKm) / 111.0
	lonDeg := float64(radiusKm) / (111.0 * math.Cos(params.Lat*math.Pi/180.0))

	xMin, yMin := tms.LatLonToZ19Tile(params.Lat+latDeg, params.Lon-lonDeg)
	xMax, yMax := tms.LatLonToZ19Tile(params.Lat-latDeg, params.Lon+lonDeg)

	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}
	if yMin > yMax {
		yMin, yMax = yMax, yMin
	}

	measuredSamples, err := tms.GetGatewaySamplesInRange(*networkID, *gatewayID, xMin, yMin, xMax, yMax)
	if err != nil {
		log.Fatalf("Failed to fetch measured data: %v", err)
	}
	// Shuffle measured samples for training/validation split
	rand.Shuffle(len(measuredSamples), func(i, j int) {
		measuredSamples[i], measuredSamples[j] = measuredSamples[j], measuredSamples[i]
	})

	trainRatio := 0.8
	trainCount := int(float64(len(measuredSamples)) * trainRatio)
	if trainCount == 0 {
		trainCount = len(measuredSamples)
	}
	trainSamples := measuredSamples[:trainCount]
	valSamples := measuredSamples[trainCount:]

	fmt.Printf("Found %d measured samples (Training: %d, Validation: %d)\n", len(measuredSamples), len(trainSamples), len(valSamples))

	bestParams := params
	bestParams.Reliability = 99.0 // Fixed reliability as per requirement
	bestCost := math.MaxFloat64

	gatewayTileX, gatewayTileY := tms.LatLonToZ19Tile(params.Lat, params.Lon)

	type radialReach struct {
		reaches [72]float64 // 5 degree sectors
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

	// Objective function
	evaluate := func(p types.SimulationParams) (cost float64) {
		types.SaveTemporaryParams(tms.MyConfiguration.GatewayParamsFile, *networkID, *gatewayID, p)
		defer os.Remove(tms.MyConfiguration.GatewayParamsFile + ".tmp")

		// Simulation still takes the "original" location as center but uses params in gateway_params.json
		// for offsets.
		simSamples, tifPath, err := tms.RunSignalServerSimulation(*networkID, *gatewayID, dbLat, dbLon, dbAlt, radiusKm)
		if err != nil {
			return math.MaxFloat64
		}
		if tifPath != "" {
			defer os.RemoveAll(filepath.Dir(tifPath))
		}

		simGX, simGY := tms.LatLonToZ19Tile(p.Lat, p.Lon)
		simReaches := getRadialReaches(simSamples, simGX, simGY)

		totalCost := 0.0

		for b := 0; b <= 11; b++ {
			mR, mExists := measuredReaches[b]
			sR, sExists := simReaches[b]
			if !mExists {
				continue
			}

			bucketCost := 0.0
			for i := 0; i < 72; i++ {
				mReach := mR.reaches[i]
				sReach := 0.0
				if sExists {
					sReach = sR.reaches[i]
				}

				if mReach > 0 {
					diff := sReach - mReach
					if diff < 0 {
						// Simulation shorter than measured - BAD
						bucketCost += math.Abs(diff) * 10.0
					} else {
						// Simulation further than measured - Acceptable but minimize
						bucketCost += diff * 1.0
					}
				} else {
					// No measurement in this sector. Simulation should be minimal.
					bucketCost += sReach * 0.1
				}
			}
			totalCost += bucketCost
		}

		return totalCost
	}

	// Initial evaluation using training set
	bestCost = evaluate(bestParams)

	fmt.Println("\nOptimizing radio parameter (Constant Loss)...")
	fmt.Printf("%-10s | %-13s | %-8s | %-15s\n", "Try", "Constant Loss", "Cost", "Status")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-10s | %-13.1f | %-8.2f | %s\n", "Initial", bestParams.ConstantLoss, bestCost, "Initial state")

	currentParams := bestParams
	totalTriesGoal := params.RadioTriesDone + *maxTries

	for tries := 1; tries <= *maxTries; tries++ {
		currentTryNumber := params.RadioTriesDone + tries
		progress := float64(currentTryNumber) / float64(totalTriesGoal)
		temp := math.Exp(-5.0 * progress)

		nextParams := currentParams
		nextParams.Reliability = 99.0 // Keep reliability static

		if currentTryNumber%10 == 0 {
			// Random jump
			nextParams.ConstantLoss = rand.Float64() * 200.0 // 0 to 200dB
		} else {
			// Local perturbation
			maxDelta := 5.0 + 20.0*temp
			delta := (rand.Float64()*2.0 - 1.0) * maxDelta

			nextParams.ConstantLoss += delta
			if nextParams.ConstantLoss < 0 {
				nextParams.ConstantLoss = 0
			}
			if nextParams.ConstantLoss > 200.0 {
				nextParams.ConstantLoss = 200.0
			}
		}

		cost := evaluate(nextParams)
		status := ""

		// Acceptance criteria: better Cost
		if cost < bestCost {
			bestCost = cost
			bestParams = nextParams
			currentParams = nextParams
			status = "New Best"
		}

		fmt.Printf("%-10d | %-13.1f | %-8.2f | %s\n",
			tries, nextParams.ConstantLoss, cost, status)

		if tries%50 == 0 && tries < *maxTries {
			fmt.Printf("\n%-10s | %-13s | %-8s | %-15s\n", "Try", "Constant Loss", "Cost", "Status")
			fmt.Println(strings.Repeat("-", 60))
		}

		// Save progress. bestParams already carries the best-found ConstantLoss
		// (it is only assigned inside the acceptance block above), so we must not
		// overwrite it with the current (possibly rejected) candidate here.
		bestParams.RadioLastRMSE = bestCost
		bestParams.RadioTriesDone = currentTryNumber
		types.SaveFinalParams(tms.MyConfiguration.GatewayParamsFile, *networkID, *gatewayID, bestParams)

		if bestCost < 10 {
			fmt.Printf("\nTarget reached! (Cost < 10)\n")
			break
		}
	}

	fmt.Println(strings.Repeat("-", 88))
	fmt.Printf("\nRadio parameter optimization finished.\nBest Parameters: ConstantLoss=%.1fdB (Reliability=%.1f%%, Cost: %.2f)\n",
		bestParams.ConstantLoss, bestParams.Reliability, bestCost)
}
