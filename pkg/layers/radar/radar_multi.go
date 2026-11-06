package radar

import (
	geo "github.com/kellydunn/golang-geo"
	"github.com/patrickmn/go-cache"
	"github.com/paulmach/go.geojson"
	"log"
	"sort"
	"time"
	"ttnmapper-services/pkg/aggregations/radar_beam"
	"ttnmapper-services/pkg/database"
)

var radarMultiCache *cache.Cache

func GenerateRadarMulti(networkId string, gatewayId string) []byte {
	cacheKey := networkId + "/" + gatewayId

	// Create cache if it does not exist
	if radarMultiCache == nil {
		radarMultiCache = cache.New(24*time.Hour, 10*time.Minute)
	}
	// Try to get radar from cache
	if x, found := radarMultiCache.Get(cacheKey); found {
		log.Println("From cache")
		geoJsonString := x.([]byte)
		return geoJsonString
	}
	log.Println("Generating new radar")

	gatewayBeams := make(map[int]map[uint]database.RadarBeam, 0)

	gateway, _ := database.GetGateway(database.GatewayIndexer{
		NetworkId: networkId,
		GatewayId: gatewayId,
	})
	if gateway.Latitude == 0 && gateway.Longitude == 0 {
		log.Println("gateway at null island")
		return nil
	}
	gatewayLocation := geo.NewPoint(gateway.Latitude, gateway.Longitude)

	antennas := database.GetAntennaForGateway(networkId, gatewayId)
	for _, antenna := range antennas {
		beams := database.GetRadarBeamsForAntenna(antenna)
		AddBeamsToGatewayMulti(gatewayBeams, beams)
	}
	//log.Println(gatewayBeams)

	FillZerosMulti(gatewayBeams)
	//log.Println(utils.PrettyPrint(gatewayBeams))
	geoJsonString := CreateGeoJsonMulti(gatewayLocation, gatewayBeams)
	radarMultiCache.Set(cacheKey, geoJsonString, cache.DefaultExpiration)
	return geoJsonString
}

func AddBeamsToGatewayMulti(gatewayBeams map[int]map[uint]database.RadarBeam, newBeams []database.RadarBeam) {
	for _, newBeam := range newBeams {

		if _, ok := gatewayBeams[newBeam.Level]; ok {
			// Level exist, find bearing
			if oldBeam, ok := gatewayBeams[newBeam.Level][newBeam.Bearing]; ok {
				// Bearing exists, update
				mergedBeam := MergeBeams(oldBeam, newBeam)
				gatewayBeams[newBeam.Level][newBeam.Bearing] = mergedBeam
			} else {
				// Bearing does not exist, just add
				gatewayBeams[newBeam.Level][newBeam.Bearing] = newBeam
			}
		} else {
			// Level does not exist, just add.
			gatewayBeams[newBeam.Level] = make(map[uint]database.RadarBeam)
			gatewayBeams[newBeam.Level][newBeam.Bearing] = newBeam
		}
	}
}

func MergeBeams(oldBeam database.RadarBeam, newBeam database.RadarBeam) database.RadarBeam {
	distances := []float64{oldBeam.Distance2nd, oldBeam.DistanceMax, newBeam.Distance2nd, newBeam.DistanceMax}
	sort.Float64s(distances)
	oldBeam.DistanceMax = distances[3]
	oldBeam.Distance2nd = distances[2]
	return oldBeam
}

func FillZerosMulti(gatewayBeams map[int]map[uint]database.RadarBeam) {

	// Fill levels
	for _, level := range radar_beam.Levels {
		if _, ok := gatewayBeams[level]; !ok {
			gatewayBeams[level] = make(map[uint]database.RadarBeam)
		}
	}

	// Fill bearings
	previousLevel := 0
	for _, level := range radar_beam.Levels {
		for bearing := 0; bearing < 360; bearing++ {
			if _, ok := gatewayBeams[level][uint(bearing)]; !ok {
				// Check if lower level has a beam
				lowerBeam, lowerHasBeam := gatewayBeams[previousLevel][uint(bearing)]
				if previousLevel != 0 && lowerHasBeam {
					gatewayBeams[level][uint(bearing)] = lowerBeam
				} else {
					// Otherwise add an empty beam
					newBeam := database.RadarBeam{
						Level:       level,
						Bearing:     uint(bearing),
						Samples:     0,
						DistanceMax: 0,
						Distance2nd: 0,
					}
					gatewayBeams[level][uint(bearing)] = newBeam
				}
			}
		}
		previousLevel = level
	}
}

func CreateGeoJsonMulti(gatewayLocation *geo.Point, gatewayBeams map[int]map[uint]database.RadarBeam) []byte {

	fc := geojson.NewFeatureCollection()

	for i := len(radar_beam.Levels); i > 0; i-- {
		level := radar_beam.Levels[i-1]
		points := make([][]float64, 0)

		/*
		   o  A linear ring MUST follow the right-hand rule with respect to the
		      area it bounds, i.e., exterior rings are counterclockwise, and
		      holes are clockwise.
		*/
		for bearing := 359; bearing > 0; bearing-- {
			pointA := gatewayLocation.PointAtDistanceAndBearing(gatewayBeams[level][uint(bearing)].DistanceMax/1000, float64(bearing+1))
			pointB := gatewayLocation.PointAtDistanceAndBearing(gatewayBeams[level][uint(bearing)].DistanceMax/1000, float64(bearing))
			points = append(points, []float64{pointA.Lng(), pointA.Lat()})
			points = append(points, []float64{pointB.Lng(), pointB.Lat()})
		}

		/*
		   o  The first and last positions are equivalent, and they MUST contain
		      identical values; their representation SHOULD also be identical.
		*/
		points = append(points, points[0])

		polygonOuterPoints := make([][][]float64, 0)
		polygonOuterPoints = append(polygonOuterPoints, points)
		polygonFeature := geojson.NewPolygonFeature(polygonOuterPoints)
		polygonFeature.SetProperty("rssi_avg", level)

		colour := "black"
		if level == -200 {
			colour = "blue"
		}
		if level == -120 {
			colour = "cyan"
		}
		if level == -115 {
			colour = "green"
		}
		if level == -110 {
			colour = "yellow"
		}
		if level == -105 {
			colour = "orange"
		}
		if level == -100 {
			colour = "red"
		}

		polygonFeature.SetProperty("fill", colour)
		//polygonFeature.SetProperty("fill-opacity", 0.5)
		//polygonFeature.SetProperty("stroke-width", 0)

		fc.AddFeature(polygonFeature)
	}

	rawJSON, err := fc.MarshalJSON()
	if err != nil {
		log.Println(err.Error())
	}
	//log.Println(string(rawJSON))
	return rawJSON
}
