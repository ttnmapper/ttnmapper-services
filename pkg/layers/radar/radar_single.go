package radar

import (
	geo "github.com/kellydunn/golang-geo"
	"github.com/patrickmn/go-cache"
	geojson "github.com/paulmach/go.geojson"
	"log"
	"time"
	"ttnmapper-services/pkg/database"
)

var radarSingleCache *cache.Cache

func GenerateRadarSingle(networkId string, gatewayId string) []byte {
	cacheKey := networkId + "/" + gatewayId

	// Create cache if it does not exist
	if radarSingleCache == nil {
		radarSingleCache = cache.New(24*time.Hour, 10*time.Minute)
	}
	// Try to get radar from cache
	if x, found := radarSingleCache.Get(cacheKey); found {
		log.Println("From cache")
		geoJsonString := x.([]byte)
		return geoJsonString
	}
	log.Println("Generating new radar")

	gatewayBeams := make(map[uint]database.RadarBeam, 0)

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
		AddBeamsToGatewaySingle(gatewayBeams, beams)
	}
	//log.Println(gatewayBeams)

	FillZerosSingle(gatewayBeams)
	//log.Println(utils.PrettyPrint(gatewayBeams))
	geoJsonString := CreateGeoJsonSingle(gatewayLocation, gatewayBeams)
	radarSingleCache.Set(cacheKey, geoJsonString, cache.DefaultExpiration)
	return geoJsonString
}

func AddBeamsToGatewaySingle(gatewayBeams map[uint]database.RadarBeam, newBeams []database.RadarBeam) {
	for _, newBeam := range newBeams {
		// Find bearing
		if oldBeam, ok := gatewayBeams[newBeam.Bearing]; ok {
			// Bearing exists, update
			mergedBeam := MergeBeams(oldBeam, newBeam)
			gatewayBeams[newBeam.Bearing] = mergedBeam
		} else {
			// Bearing does not exist, just add
			gatewayBeams[newBeam.Bearing] = newBeam
		}
	}
}

func FillZerosSingle(gatewayBeams map[uint]database.RadarBeam) {
	// Fill bearings
	for bearing := 0; bearing < 360; bearing++ {
		if _, ok := gatewayBeams[uint(bearing)]; !ok {
			// Add an empty beam
			newBeam := database.RadarBeam{
				Level:       0,
				Bearing:     uint(bearing),
				Samples:     0,
				DistanceMax: 0,
				Distance2nd: 0,
			}
			gatewayBeams[uint(bearing)] = newBeam
		}
	}
}

func CreateGeoJsonSingle(gatewayLocation *geo.Point, gatewayBeams map[uint]database.RadarBeam) []byte {

	fc := geojson.NewFeatureCollection()

	points := make([][]float64, 0)

	/*
		   o  A linear ring MUST follow the right-hand rule with respect to the
			  area it bounds, i.e., exterior rings are counterclockwise, and
			  holes are clockwise.
	*/
	for bearing := 359; bearing > 0; bearing-- {
		pointA := gatewayLocation.PointAtDistanceAndBearing(gatewayBeams[uint(bearing)].DistanceMax/1000, float64(bearing+1))
		pointB := gatewayLocation.PointAtDistanceAndBearing(gatewayBeams[uint(bearing)].DistanceMax/1000, float64(bearing))
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

	polygonFeature.SetProperty("fill", "blue")
	polygonFeature.SetProperty("fill-opacity", 0.5)
	polygonFeature.SetProperty("stroke-width", 0)

	fc.AddFeature(polygonFeature)

	rawJSON, err := fc.MarshalJSON()
	if err != nil {
		log.Println(err.Error())
	}
	//log.Println(string(rawJSON))
	return rawJSON
}
