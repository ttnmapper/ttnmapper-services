package utils

import (
	"errors"
	"github.com/kellydunn/golang-geo"
	"log"
	"math"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

const (
	GatewayMaximumRangeKm = 200
)

func GetPointForNetworkGateway(networkId string, gatewayId string) (*geo.Point, error) {
	gatewayIndexer := database.GatewayIndexer{
		NetworkId: networkId,
		GatewayId: gatewayId,
	}
	gateway, err := database.GetGateway(gatewayIndexer)
	if err != nil {
		return nil, err
	}
	if gateway.Latitude == 0 && gateway.Longitude == 0 {
		return nil, errors.New("gateway location unknown")
	}
	gatewayPoint := geo.NewPoint(gateway.Latitude, gateway.Longitude)
	return gatewayPoint, nil
}

func CheckDistanceFromAntenna(antenna database.Antenna, packet database.Packet) bool {
	gatewayPoint, err := GetPointForNetworkGateway(antenna.NetworkId, antenna.GatewayId)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	measurementPoint := geo.NewPoint(packet.Latitude, packet.Longitude)
	km := gatewayPoint.GreatCircleDistance(measurementPoint)
	return CheckDistance(km)
}

func CheckDistanceFromGateway(gateway types.TtnMapperGateway, message types.TtnMapperUplinkMessage) bool {
	gatewayPoint, err := GetPointForNetworkGateway(gateway.NetworkId, gateway.GatewayId)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	measurementPoint := geo.NewPoint(message.Latitude, message.Longitude)
	km := gatewayPoint.GreatCircleDistance(measurementPoint)
	return CheckDistance(km)
}

func CheckDistance(km float64) bool {
	if km == 0 || km > GatewayMaximumRangeKm {
		return false
	} else {
		return true
	}
}

func GetDistanceLive(gateway types.TtnMapperGateway, message types.TtnMapperUplinkMessage) float64 {
	gatewayPoint, err := GetPointForNetworkGateway(gateway.NetworkId, gateway.GatewayId)
	if err != nil {
		log.Println(err.Error())
		return 0
	}
	measurementPoint := geo.NewPoint(message.Latitude, message.Longitude)
	return gatewayPoint.GreatCircleDistance(measurementPoint)
}

func GetDistanceDatabase(gatewayDb database.Gateway, packet database.Packet) float64 {
	gatewayPoint := geo.NewPoint(gatewayDb.Latitude, gatewayDb.Longitude)
	measurementPoint := geo.NewPoint(packet.Latitude, packet.Longitude)
	return gatewayPoint.GreatCircleDistance(measurementPoint)
}

func GetBearingDatabase(gatewayDb database.Gateway, packet database.Packet) uint {
	gatewayPoint := geo.NewPoint(gatewayDb.Latitude, gatewayDb.Longitude)
	measurementPoint := geo.NewPoint(packet.Latitude, packet.Longitude)
	return getBearingBetweenPoints(gatewayPoint, measurementPoint)
}

func GetBearingLive(gateway types.TtnMapperGateway, message types.TtnMapperUplinkMessage) uint {
	gatewayPoint := geo.NewPoint(gateway.Latitude, gateway.Longitude)
	measurementPoint := geo.NewPoint(message.Latitude, message.Longitude)
	return getBearingBetweenPoints(gatewayPoint, measurementPoint)
}

func getBearingBetweenPoints(from *geo.Point, to *geo.Point) uint {
	// Floor because all points between 0 an 1 falls in bucket index 0
	bearing := from.BearingTo(to)
	if bearing < 0 {
		bearing += 360
	}
	return uint(math.Floor(bearing))
}
