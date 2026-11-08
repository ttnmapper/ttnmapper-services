package gatewayupdate

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/utils"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streadway/amqp"
)

type Configuration struct {
	AmqpHost                 string `envconfig:"AMQP_HOST"`
	AmqpPort                 string `envconfig:"AMQP_PORT"`
	AmqpUser                 string `envconfig:"AMQP_USER"`
	AmqpPassword             string `envconfig:"AMQP_PASSWORD"`
	AmqpExchangeRawPackets   string `envconfig:"AMQP_EXHANGE_RAW"`
	AmqpQueueRawPackets      string `envconfig:"AMQP_QUEUE"`
	AmqpExchangeGatewayMoved string `envconfig:"AMQP_EXCHANGE_GATEWAY_MOVED"`

	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresPort     string `envconfig:"POSTGRES_PORT"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDatabase string `envconfig:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `envconfig:"POSTGRES_DEBUG_LOG"`

	PrometheusPort string `envconfig:"PROMETHEUS_PORT"`

	FetchAmqp bool `envconfig:"FETCH_AMQP"` // Should we subscribe to the amqp queue to process live data
	//FetchNoc          bool `env:"FETCH_NOC"`  // Should we periodically fetch gateway statuses from the NOC (TTNv2)

	FetchWeb         bool `envconfig:"FETCH_WEB"`          // Should we periodically fetch gateway statuses from the TTN website (TTNv2 and v3)
	FetchWebInterval int  `envconfig:"FETCH_WEB_INTERVAL"` // How often in seconds should we fetch gateway statuses from the TTN Website

	FetchPacketBroker         bool `envconfig:"FETCH_PACKET_BROKER"`
	FetchPacketBrokerInterval int  `envconfig:"FETCH_PACKET_BROKER_INTERVAL"`

	FetchRouting         bool `envconfig:"FETCH_ROUTING"`
	FetchRoutingInterval int  `envconfig:"FETCH_ROUTING_INTERVAL"`

	FetchHelium         bool `envconfig:"FETCH_HELIUM"`
	FetchHeliumInterval int  `envconfig:"FETCH_HELIUM_INTERVAL"`

	FetchHeliumSnapshot         bool `envconfig:"FETCH_HELIUM_SNAPSHOT"`
	FetchHeliumSnapshotInterval int  `envconfig:"FETCH_HELIUM_SNAPSHOT_INTERVAL"`

	FetchTts         bool `envconfig:"FETCH_TTS"`
	FetchTtsInterval int  `envconfig:"FETCH_TTS_INTERVAL"`
}

var myConfiguration = Configuration{
	AmqpHost:                 "localhost",
	AmqpPort:                 "5672",
	AmqpUser:                 "user",
	AmqpPassword:             "password",
	AmqpExchangeRawPackets:   "new_packets",
	AmqpQueueRawPackets:      "gateway_updates_raw",
	AmqpExchangeGatewayMoved: "gateway_moved",

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	PrometheusPort: "9100",

	FetchAmqp: false,
	//FetchNoc:          false,
	FetchWeb:                    false,
	FetchWebInterval:            3600,
	FetchPacketBroker:           false,
	FetchPacketBrokerInterval:   3600,
	FetchRouting:                false,
	FetchRoutingInterval:        86400,
	FetchHelium:                 false,
	FetchHeliumInterval:         86400,
	FetchHeliumSnapshot:         false,
	FetchHeliumSnapshotInterval: 43200, // twice per day in case we tried fetching between 00:00 and 01:00 while the snapshot is not yet available
	FetchTts:                    false,
	FetchTtsInterval:            3600,
}

var (
	// Prometheus stats
	processedGateways = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ttnmapper_gateway_processed_count",
		Help: "The total number of gateway updates processed",
	})
	updatedGateways = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ttnmapper_gateway_updated_count",
		Help: "The total number of gateways updated",
	})
	newGateways = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ttnmapper_gateway_new_count",
		Help: "The total number of new gateways seen",
	})
	movedGateways = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ttnmapper_gateway_moved_count",
		Help: "The total number of gateways that moved",
	})

	insertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_gateway_processed_duration",
		Help:    "How long the processing of a gateway status took",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})

	routingUpdates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ttnmapper_packet_broker_routes_update",
		Help: "Count how many times we've updated the routing table",
	})

	// Other global vars
	rawPacketsChannel = make(chan amqp.Delivery)
)

func Run() {
	reprocess := flag.Bool("reprocess", false, "Reprocess by fetching gateway statuses from specific endpoints")
	flag.Parse()
	reprocessApis := flag.Args()

	err := envconfig.Process("", &myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("[Configuration]\n%s\n", utils.PrettyPrint(myConfiguration)) // output: [UserA, UserB]

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe("0.0.0.0:"+myConfiguration.PrometheusPort, nil)
		if err != nil {
			log.Print(err.Error())
		}
	}()

	log.Println("Init database")
	databaseContext := database.Context{
		Host:     myConfiguration.PostgresHost,
		Port:     myConfiguration.PostgresPort,
		User:     myConfiguration.PostgresUser,
		Database: myConfiguration.PostgresDatabase,
		Password: myConfiguration.PostgresPassword,
		DebugLog: myConfiguration.PostgresDebugLog,
	}
	databaseContext.Init()

	if *reprocess {
		for _, service := range reprocessApis {
			if service == "web" {
				log.Println("Fetching web gateway statuses")
				fetchWebStatuses()
			}
			if service == "packetbroker" {
				log.Println("Fetching Packet Broker gateway statuses")
				fetchPacketBrokerStatuses()
			}
			if service == "routing" {
				log.Println("Fetching Packet Broker routing policies")
				FetchPbRoutingPolicies()
			}
			if service == "helium" {
				log.Println("Fetching Helium hotspot statuses")
				fetchHeliumStatuses()
			}
			if service == "helium-snapshot" {
				log.Println("Fetching Snapshot of Helium hotspot statuses")
				fetchHeliumSnapshot()
			}
			if service == "tts" {
				log.Println("Fetching TTS network statuses")
				fetchAllTtsStatuses()
			}
		}
	} else {
		// Start amqp listener on this thread - blocking function
		if myConfiguration.FetchAmqp {
			log.Println("Starting AMQP thread")
			subscribeToRabbitRaw()
		}

		// Periodic status fetchers
		startPeriodicFetchers()

		log.Printf("Init Complete")
		forever := make(chan bool)
		<-forever
	}
}
