package utils

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func TestGetBearing(t *testing.T) {
	gateway := types.TtnMapperGateway{Latitude: -33.936644228282525, Longitude: 18.87102216482163}
	message := types.TtnMapperUplinkMessage{Latitude: -33.944952, Longitude: 18.861567}
	bearing := GetBearingLive(gateway, message)
	log.Println(bearing)
}

func TestGetBearingFromPacket(t *testing.T) {
	gateway := database.Gateway{Latitude: -33.936644228282525, Longitude: 18.87102216482163}
	packet := database.Packet{Latitude: -33.944952, Longitude: 18.861567}
	bearing := GetBearingDatabase(gateway, packet)
	log.Println(bearing)
}

func TestGetDistanceFromGateway(t *testing.T) {
	gateway := types.TtnMapperGateway{Latitude: -33.93648714508494, Longitude: 18.868361593713505}
	message := types.TtnMapperUplinkMessage{Latitude: -33.93623234324361, Longitude: 18.871634206336243}
	distance := GetDistanceLive(gateway, message)
	log.Println(distance)
}
