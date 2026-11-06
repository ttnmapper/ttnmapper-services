package noc

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
	"ttnmapper-services/pkg/types"
)

var Url = "http://noc.thethingsnetwork.org:8085/api/v2/gateways"

func FetchNocStatuses() (map[string]NocGateway, error) {
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

	nocData := NocStatus{}
	err = json.NewDecoder(res.Body).Decode(&nocData)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return nocData.Statuses, nil
}

func NocGatewayToTtnMapperGateway(gatewayId string, gatewayIn NocGateway) types.TtnMapperGateway {
	var gatewayOut types.TtnMapperGateway

	// Assume NOC lists only TTN gateways. Need to check this as a private V2 network can also have a NOC
	gatewayOut.NetworkId = "thethingsnetwork.org"

	gatewayOut.GatewayId = gatewayId
	gatewayOut.Time = gatewayIn.Timestamp.UnixNano()
	//ttnMapperGateway.Latitude = gateway.Location.Latitude
	//ttnMapperGateway.Longitude = gateway.Location.Longitude
	//ttnMapperGateway.Altitude = int32(gateway.Location.Altitude)
	//ttnMapperGateway.Description = gateway.Description

	// Ignore locations obtained from NOC
	gatewayOut.Latitude = 0
	gatewayOut.Longitude = 0
	gatewayOut.Altitude = 0

	return gatewayOut
}
