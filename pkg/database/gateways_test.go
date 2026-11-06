package database

import (
	"log"
	"testing"
	"time"
	"ttnmapper-services/pkg/utils"
)

func TestGetGatewaysByNameOrId(t *testing.T) {
	initDb()

	gateways := GetGatewaysByNameOrId("Xf1p4e7QPx2Kb1lqLq4ql/H9zF+2jH0lLpMGg+w4W7o=")
	log.Println(utils.PrettyPrint(gateways))
}

func TestGetPacketsForGateway(t *testing.T) {
	initDb()

	packets, err := GetPacketsForGateway("NS_TTS_V3://deutschebahn@000013", "Xf1p4e7QPx2Kb1lqLq4ql/H9zF+2jH0lLpMGg+w4W7o=", time.Unix(0, 0), time.Now(), 10000)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// This should return data, but it does not
	log.Println(utils.PrettyPrint(packets))
}

func TestGetOnlineGatewaysForNetworkInBbox(t *testing.T) {
	initDb()

	gateways := GetOnlineGatewaysForNetwork("NS_TTS_V3://ttn@000013")
	log.Println(len(gateways), "gateways")
	gateways = GetOnlineGatewaysForNetwork("NS_TTS_V3://ttn@000013")
	log.Println(len(gateways), "gateways")
}
