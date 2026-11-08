package gatewayupdate

import (
	"log"
	"os"
	"time"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/gateway-statuses/helium"
	"ttnmapper-services/pkg/gateway-statuses/noc"
	"ttnmapper-services/pkg/gateway-statuses/packet_broker"
	"ttnmapper-services/pkg/gateway-statuses/thethingsstack"
	"ttnmapper-services/pkg/gateway-statuses/web"
	"ttnmapper-services/pkg/queues"
	"ttnmapper-services/pkg/utils"

	"github.com/streadway/amqp"
)

func subscribeToRabbitRaw() {
	go func() {
		options := queues.Subscription{QueueCredentials: queues.QueueCredentials{User: myConfiguration.AmqpUser, Password: myConfiguration.AmqpPassword, Host: myConfiguration.AmqpHost, Port: myConfiguration.AmqpPort}, Exchange: myConfiguration.AmqpExchangeRawPackets, Queue: myConfiguration.AmqpQueueRawPackets, AutoAck: true, Prefetch: 10}
		err := queues.RunSubscription(options, func(delivery amqp.Delivery) error { rawPacketsChannel <- delivery; return nil })
		utils.FailOnError(err, "Subscribe channel closed")
	}()
}

func startPeriodicFetchers() {
	//nocTicker := time.NewTicker(time.Duration(myConfiguration.FetchNocInterval) * time.Second)
	webTicker := time.NewTicker(time.Duration(myConfiguration.FetchWebInterval) * time.Second)
	pbTicker := time.NewTicker(time.Duration(myConfiguration.FetchPacketBrokerInterval) * time.Second)
	heliumTicker := time.NewTicker(time.Duration(myConfiguration.FetchHeliumInterval) * time.Second)
	heliumSnapshotTicker := time.NewTicker(time.Duration(myConfiguration.FetchHeliumSnapshotInterval) * time.Second)
	ttsTicker := time.NewTicker(time.Duration(myConfiguration.FetchTtsInterval) * time.Second)
	pbRoutingTicker := time.NewTicker(time.Duration(myConfiguration.FetchRoutingInterval) * time.Second)

	go func() {
		for {
			select {
			//case <-nocTicker.C:
			//	if myConfiguration.FetchNoc {
			//		go fetchNocStatuses()
			//	}
			case <-webTicker.C:
				if myConfiguration.FetchWeb {
					go fetchWebStatuses()
				}
			case <-pbTicker.C:
				if myConfiguration.FetchPacketBroker {
					go fetchPacketBrokerStatuses()
				}
			case <-heliumTicker.C:
				if myConfiguration.FetchHelium {
					go fetchHeliumStatuses()
				}
			case <-heliumSnapshotTicker.C:
				if myConfiguration.FetchHeliumSnapshot {
					go fetchHeliumSnapshot()
				}
			case <-ttsTicker.C:
				if myConfiguration.FetchTts {
					go fetchAllTtsStatuses()
				}
			case <-pbRoutingTicker.C:
				if myConfiguration.FetchRouting {
					go FetchPbRoutingPolicies()
				}
			}
		}
	}()
}

var busyFetchingNoc = false

func fetchNocStatuses() {
	if busyFetchingNoc {
		return
	}
	busyFetchingNoc = true

	gateways, err := noc.FetchNocStatuses()
	if err != nil {
		log.Println(err.Error())
	} else {
		for id, gateway := range gateways {
			ttnMapperGateway := noc.NocGatewayToTtnMapperGateway(id, gateway)
			log.Print("NOC ", "", "\t", ttnMapperGateway.GatewayId+"\t", ttnMapperGateway.Time)
			UpdateGateway(ttnMapperGateway)
		}
	}

	busyFetchingNoc = false
}

var busyFetchingWeb = false

func fetchWebStatuses() {
	if busyFetchingWeb {
		return
	}
	busyFetchingWeb = true

	gatewayCount := 0
	gateways, err := web.FetchWebStatuses()
	if err != nil {
		log.Println(err.Error())
	} else {
		for _, gateway := range gateways {
			gatewayCount++
			ttnMapperGateway := web.WebGatewayToTtnMapperGateway(*gateway)
			log.Print("WEB ", "", "\t", ttnMapperGateway.GatewayId+"\t", ttnMapperGateway.Time)
			UpdateGateway(ttnMapperGateway)
		}
	}

	log.Printf("Fetched %d gateways from TTN website", gatewayCount)
	busyFetchingWeb = false
}

/*
Fetching on 2021-11-17 took
real	4m5.426s
user	0m3.360s
sys	0m4.620s
*/
var busyFetchingPacketBroker = false

func fetchPacketBrokerStatuses() {
	if busyFetchingPacketBroker {
		return
	}
	busyFetchingPacketBroker = true

	gatewayCount := 0
	page := 0
	for {

		gateways, err := packet_broker.FetchStatuses(page)
		if err != nil {
			log.Println(err.Error())
			break
		} else {
			for _, gateway := range gateways {
				gatewayCount++
				ttnMapperGateway, err := packet_broker.PbGatewayToTtnMapperGateway(gateway)
				if err == nil {
					log.Print("PB ", "", "\t", ttnMapperGateway.GatewayId+"\t", ttnMapperGateway.Time)
					UpdateGateway(ttnMapperGateway)
				}
			}
		}
		time.Sleep(10 * time.Second) // need to delay to prevent hitting rate limit

		page++
	}

	log.Printf("Fetched %d gateways from Packet Broker", gatewayCount)
	busyFetchingPacketBroker = false
}

var busyFetchingHeliumSnapshot = false

func fetchHeliumSnapshot() {
	if busyFetchingHeliumSnapshot {
		return
	}
	busyFetchingHeliumSnapshot = true

	hotspotCount := 0
	hotspots, err := helium.FetchDisk91Snapshot() //helium.FetchSnapshot()
	if err != nil {
		log.Println(err.Error())
		return
	}

	for _, hotspot := range hotspots {
		hotspotCount++
		ttnMapperGateway, err := helium.Disk91SnapshotToTtnMapperGateway(hotspot)
		if err == nil {
			UpdateGateway(ttnMapperGateway)
		}
	}

	log.Printf("Fetched %d hotspots from Helium Snapshot", len(hotspots))

	busyFetchingHeliumSnapshot = false
}

/*
Fetching Helium on 2021-11-17 took
real    306m29.503s
user    1m47.352s
sys     3m13.356s
*/
var busyFetchingHelium = false

func fetchHeliumStatuses() {
	if busyFetchingHelium {
		return
	}
	busyFetchingHelium = true

	hotspotCount := 0
	cursor := ""
	for {
		response, err := helium.FetchStatuses(cursor)
		if err != nil {
			log.Println(err.Error())
			break
		}

		log.Printf("HELIUM %d hotspots\n", len(response.Data))

		for _, hotspot := range response.Data {
			hotspotCount++
			ttnMapperGateway, err := helium.HeliumHotspotToTtnMapperGateway(hotspot)
			if err == nil {
				//log.Print("HELIUM ", "", "\t", ttnMapperGateway.GatewayId+"\t", ttnMapperGateway.Time)
				UpdateGateway(ttnMapperGateway)
			}
		}

		cursor = response.Cursor
		if cursor == "" {
			log.Println("Cursor empty")
			break
		}
		if len(response.Data) == 0 {
			log.Println("No hotspots in response")
			break
		}
	}

	log.Printf("Fetched %d hotspots from Helium", hotspotCount)
	busyFetchingHelium = false
}

// Fetch statuses from private TTS instances

var busyFetchingTtsNetworkStatuses = false

func fetchAllTtsStatuses() {
	if busyFetchingTtsNetworkStatuses {
		return
	}
	busyFetchingTtsNetworkStatuses = true

	networks := database.GetAllTtsNetworksToFetch()

	for _, network := range networks {
		fetchTtsStatuses(network)
	}

	busyFetchingTtsNetworkStatuses = false
}

func fetchTtsStatuses(network database.TtsV3FetchStatus) {
	gatewayCount := 0
	gateways, err := thethingsstack.FetchGateways(network.TenantId, network.ApiKey)
	if err != nil {
		log.Println(err.Error())
	}

	log.Println(utils.PrettyPrint(gateways))

	// Fetch gateway statuses in batches
	batchSize := 50
	for i := 0; i < len(gateways); i += batchSize {
		log.Printf("[TTS API] Fetching batch of %d gateway statuses", batchSize)
		endIndex := i + batchSize
		if len(gateways) < endIndex {
			endIndex = len(gateways)
		}
		currentlyFetchingGateways := gateways[i:endIndex]
		gatewayStatuses, err := thethingsstack.FetchStatusesBatch(currentlyFetchingGateways, network.ApiKey)
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
					log.Println(network.TenantId, gatewayId)
					log.Println(utils.PrettyPrint(status))
					ttnMapperGateway, err := thethingsstack.TtsApiGatewayToTtnMapperGateway(network.TenantId, gateway, status)
					if err != nil {
						log.Println(err)
						continue
					}
					UpdateGateway(ttnMapperGateway)
					gatewayCount++
				}
			}
		}
	}
	log.Printf("[TTS API] Fetched %d gateway statuses for network %s", gatewayCount, network.TenantId)
}

// Fetch routing policies from Packet Broker

var busyFetchingRouting = false

func FetchPbRoutingPolicies() {
	if busyFetchingRouting {
		return
	}
	busyFetchingRouting = true

	var netId uint32 = 0x000013
	tenantId := "ttn"
	policies := packet_broker.FetchRoutingPolicies(netId, tenantId, os.Getenv("PB_API_KEY_ID"), os.Getenv("PB_API_KEY_SECRET"))

	//log.Println(utils.PrettyPrint(policies))
	for _, policy := range policies {
		// The results include wildcard policies. Replace empty wildcard fields with known network values.
		policy.HomeNetworkNetId = netId
		policy.HomeNetworkTenantId = tenantId

		dbPolicy := database.PacketBrokerRoutingPolicy{}
		packet_broker.RoutingPolicyToDbPolicy(policy, &dbPolicy)

		//log.Println(dbPolicy.HomeNetworkId, " - ", dbPolicy.ForwarderNetworkId)
		database.InsertOrUpdateRoutingPolicy(dbPolicy)
	}

	routingUpdates.Inc()
	busyFetchingRouting = false
}
