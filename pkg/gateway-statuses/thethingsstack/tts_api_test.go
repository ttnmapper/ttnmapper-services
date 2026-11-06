package thethingsstack

import (
	"encoding/json"
	"log"
	"os"
	"testing"
	"ttnmapper-services/pkg/utils"
)

func TestParseV3GatewayList(t *testing.T) {
	buf, err := os.ReadFile("gateway-list-example.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	var apiResponse V3Gateways
	err = json.Unmarshal(buf, &apiResponse)
	if err != nil {
		t.Fatalf(err.Error())
	}

	//for i, ttsGateway := range apiResponse.Gateways {
	//	log.Println(i)
	//	//log.Println(utils.PrettyPrint(ttsGateway))
	//	gateway, _ := TtsApiGatewayToTtnMapperGateway(ttsGateway)
	//	log.Println(utils.PrettyPrint(gateway))
	//}
}

func TestFetchTtsNetwork(t *testing.T) {

	//tenantId := "jpmeijers"
	//apiKey := "NNSXS."
	//tenantId := "packetworx"
	//apiKey := "NNSXS."
	tenantId := "redyteliot"
	apiKey := "NNSXS."

	gateways, err := FetchGateways(tenantId, apiKey)
	if err != nil {
		log.Println(err.Error())
	}

	// Fetch gateway statuses in batches
	gatewayCount := 0
	batchSize := 50
	for i := 0; i < len(gateways); i += batchSize {
		log.Printf("[TTS API] Fetching batch of %d gateway statuses", batchSize)
		endIndex := i + batchSize
		if len(gateways) < endIndex {
			endIndex = len(gateways)
		}
		currentlyFetchingGateways := gateways[i:endIndex]
		gatewayStatuses, err := FetchStatusesBatch(currentlyFetchingGateways, apiKey)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		if gatewayStatuses.Entries == nil {
			log.Println("[TTS API] Status Entries is nil")
		}
		// Iterate status responses
		for gatewayId, status := range gatewayStatuses.Entries {
			// Iterate fetched gateway list to find requested gateway's data
			for _, gateway := range currentlyFetchingGateways {
				// If we found the gateway, ie the id matches, update its status
				if gateway.Ids.GatewayId == gatewayId {
					log.Println(tenantId, gatewayId)
					log.Println(utils.PrettyPrint(gateway))
					ttnMapperGateway, err := TtsApiGatewayToTtnMapperGateway(tenantId, gateway, status)
					if err != nil {
						log.Println(err)
						continue
					}
					log.Println(utils.PrettyPrint(ttnMapperGateway))
					gatewayCount++
				}
			}
		}
	}
	log.Printf("[TTS API] Fetched %d gateway statuses for network %s", gatewayCount, tenantId)
}
