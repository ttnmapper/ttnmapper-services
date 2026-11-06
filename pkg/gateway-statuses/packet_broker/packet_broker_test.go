package packet_broker

import (
	"log"
	"os"
	"testing"
	"ttnmapper-services/pkg/database"
)

func TestFetchStatusesPage(t *testing.T) {
	gateways, err := FetchStatuses(0)
	if err != nil {
		t.Fatalf(err.Error())
	}

	//for _, gateway := range gateways {
	//	log.Println(utils.PrettyPrint(gateway))
	//	ttnMapperGw, err := PbGatewayToTtnMapperGateway(gateway)
	//	if err != nil {
	//		t.Fatalf(err.Error())
	//	}
	//	log.Println(utils.PrettyPrint(ttnMapperGw))
	//}

	log.Printf("Fetched %d gateways", len(gateways))
}

func TestFetchStatusesAll(t *testing.T) {
	page := 0
	for {
		gateways, err := FetchStatuses(page)
		if err != nil {
			log.Println(err.Error())
			break
		} else {
			log.Printf("Fetched %d statuses for page %d", len(gateways), page)
		}
		page++
	}
}

func TestFetchRoutingPolicies(t *testing.T) {
	var netId uint32 = 0x000013
	tenantId := "ttn"
	policies := FetchRoutingPolicies(netId, tenantId, os.Getenv("PB_API_KEY_ID"), os.Getenv("PB_API_KEY_SECRET"))

	//log.Println(utils.PrettyPrint(policies))
	for _, policy := range policies {
		// The results include wildcard policies. Replace empty wildcard fields with known network values.
		policy.HomeNetworkNetId = netId
		policy.HomeNetworkTenantId = tenantId

		dbPolicy := database.PacketBrokerRoutingPolicy{}
		RoutingPolicyToDbPolicy(policy, &dbPolicy)

		log.Println(dbPolicy.HomeNetworkId, " - ", dbPolicy.ForwarderNetworkId)
		//InsertOrUpdateRoutingPolicy(policy)
	}
}
