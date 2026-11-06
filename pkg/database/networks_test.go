package database

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/utils"
)

func TestGetOnlineGatewaysForNetwork(t *testing.T) {
	initDb()

	gateways := GetOnlineGatewaysForNetwork("NS_TTS_V3://packetworx@000013")
	log.Println(utils.PrettyPrint(gateways))
}

func TestGetNetworkSubscription(t *testing.T) {
	initDb()

	//AutoMigrate(&NetworkSubscription{})

	subscription := GetNetworkSubscription("NS_TTS_V3://swansea-bay@000013")
	log.Println(subscription)
}
