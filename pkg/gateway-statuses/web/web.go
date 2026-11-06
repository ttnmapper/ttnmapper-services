package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ttnmapper-services/pkg/types"
)

var Url = "https://www.thethingsnetwork.org/gateway-data/"

func FetchWebStatuses() (map[string]*WebGateway, error) {
	log.Println("Fetching web statuses")

	httpClient := http.Client{
		Timeout: time.Second * 60, // Maximum of 1 minute
	}

	req, err := http.NewRequest(http.MethodGet, Url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ttnmapper-update-gateway")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	webData := map[string]*WebGateway{}
	err = json.NewDecoder(res.Body).Decode(&webData)
	if err != nil {
		return nil, err
	}
	return webData, nil
}

func WebGatewayToTtnMapperGateway(gatewayIn WebGateway) types.TtnMapperGateway {
	gatewayOut := types.TtnMapperGateway{}

	// Website lists only TTN gateways
	if gatewayIn.Network == "ttnv2" {
		gatewayOut.NetworkId = "thethingsnetwork.org"
	} else if gatewayIn.Network == "ttn" {
		gatewayOut.NetworkId = "NS_TTS_V3://ttn@000013"
	} else {
		log.Println("Unknown network " + gatewayIn.Network)
	}

	gatewayOut.GatewayId = gatewayIn.ID

	// V2 has LastSeen, V3 has Online
	if gatewayIn.LastSeen != nil {
		gatewayOut.Time = gatewayIn.LastSeen.UnixNano()
	} else {
		if gatewayIn.Online == true {
			gatewayOut.Time = time.Now().UnixNano()
		}
	}

	// eui-c0ee40ffff29618d
	if len(gatewayIn.ID) == 20 && strings.HasPrefix(gatewayIn.ID, "eui-") {
		gatewayOut.GatewayEui = strings.ToUpper(strings.TrimPrefix(gatewayIn.ID, "eui-"))
	}
	// 00800000a000222e
	_, err := strconv.ParseUint(gatewayIn.ID, 16, 64)
	if err == nil {
		// Is a valid hex number
		if len(gatewayIn.ID) == 16 {
			gatewayOut.GatewayEui = strings.ToUpper(gatewayIn.ID)
		}
	}

	// Only use location obtained from web api
	gatewayOut.Latitude = gatewayIn.Location.Latitude
	gatewayOut.Longitude = gatewayIn.Location.Longitude
	gatewayOut.Altitude = int32(gatewayIn.Location.Altitude)

	// V3 has name and description, V2 has description
	gatewayOut.Name = gatewayIn.Name
	gatewayOut.Attributes = make(map[string]interface{}, 0)
	if gatewayIn.Description != "" {
		gatewayOut.Attributes["description"] = gatewayIn.Description
	}

	return gatewayOut
}
