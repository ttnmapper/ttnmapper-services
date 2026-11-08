package aggregate

import (
	"flag"
	"log"
	"net/http"
	"time"
	"ttnmapper-services/pkg/aggregations"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"ttnmapper-services/pkg/config"
)

type Configuration struct {
	AmqpHost                 string `env:"AMQP_HOST"`
	AmqpPort                 string `env:"AMQP_PORT"`
	AmqpUser                 string `env:"AMQP_USER"`
	AmqpPassword             string `env:"AMQP_PASSWORD"`
	AmqpExchangeInsertedData string `env:"AMQP_EXCHANGE_INSERTED"`
	AmqpQueueInsertedData    string `env:"AMQP_QUEUE_INSERTED"`
	AmqpExchangeGatewayMoved string `env:"AMQP_EXCHANGE_GATEWAY_MOVED"`
	AmqpQueueGatewayMoved    string `env:"AMQP_QUEUE_GATEWAY_MOVED"`

	PostgresHost          string `env:"POSTGRES_HOST"`
	PostgresPort          string `env:"POSTGRES_PORT"`
	PostgresUser          string `env:"POSTGRES_USER"`
	PostgresPassword      string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase      string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog      bool   `env:"POSTGRES_DEBUG_LOG"`
	PostgresInsertThreads int    `env:"POSTGRES_INSERT_THREADS"`

	PrometheusPort string `env:"PROMETHEUS_PORT"`
}

var myConfiguration = Configuration{
	AmqpHost:                 "localhost",
	AmqpPort:                 "5672",
	AmqpUser:                 "user",
	AmqpPassword:             "password",
	AmqpExchangeInsertedData: "inserted_data",
	AmqpQueueInsertedData:    "inserted_data_gridcell",
	AmqpExchangeGatewayMoved: "gateway_moved",
	AmqpQueueGatewayMoved:    "gateway_moved_gridcell",

	PostgresHost:          "localhost",
	PostgresPort:          "5432",
	PostgresUser:          "username",
	PostgresPassword:      "password",
	PostgresDatabase:      "database",
	PostgresDebugLog:      false,
	PostgresInsertThreads: 1,

	PrometheusPort: "9100",
}

func Run() {

	reprocess := flag.Bool("reprocess", false, "Reprocess all or specific gateways")
	offset := flag.Int("offset", 0, "Skip this number of gateways when reprocessing all")
	//aggregationType := flag.String("type", "", "Which type of aggregation to reprocess: radar, gridcell. Defaults to all.")
	network := flag.String("network", "", "When specified reprocess only gateways for this network")
	flag.Parse()
	reprocessGateways := flag.Args()

	err := config.Environment(&myConfiguration)
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

	// Should we reprocess or listen for live data?
	if *reprocess {
		log.Println("Reprocessing")

		if len(reprocessGateways) > 0 {
			ReprocessGateways(reprocessGateways)
		} else {
			ReprocessAll(*network, *offset)
		}

	} else {
		// Start amqp listener threads
		log.Println("Starting AMQP thread")
		go subscribeToRabbitNewData()
		go subscribeToRabbitMovedGateway()

		// Starting processing threads
		go processNewData()
		go processMovedGateway()

		log.Printf("Init Complete")
		forever := make(chan bool)
		<-forever
	}
}

func ReprocessAll(network string, offset int) {

	var gateways []database.Gateway
	slow := false
	if network != "" {
		log.Println("All gateways for network", network)
		gateways = database.GetAllGatewaysForNetwork(network)
	} else {
		log.Println("All gateways")
		gateways = database.GetAllGateways()
		slow = true // if we process everything, do it slowly
	}

	for i, gateway := range gateways {
		// Use offset to skip a certain number of gateways
		if i < offset {
			continue
		}
		log.Println(i, "/", len(gateways), " ", gateway.NetworkId, " - ", gateway.GatewayId)
		ReprocessSingleGateway(gateway)
		if slow {
			time.Sleep(1 * time.Second)
		}
	}
}

func ReprocessGateways(gatewayIds []string) {
	for _, gatewayId := range gatewayIds {
		// The same gateway_id can exist in multiple networks, so iterate them all
		gateways := database.GetGatewaysWithId(gatewayId)

		for i, gateway := range gateways {
			log.Println(i, "/", len(gateways), " ", gateway.NetworkId, " - ", gateway.GatewayId)
			ReprocessSingleGateway(gateway)
		}
	}
}

func ReprocessSingleGateway(gateway database.Gateway) {
	reprocessGateway(gateway, database.GetAntennaForGateway, database.GetGatewayLastMovedTime, aggregations.ReprocessAntenna)
}
func reprocessGateway(gateway database.Gateway, antennas func(string, string) []database.Antenna, lastMoved func(string, string) time.Time, reprocess func(database.Antenna, time.Time)) {
	moved := lastMoved(gateway.NetworkId, gateway.GatewayId)
	for _, antenna := range antennas(gateway.NetworkId, gateway.GatewayId) {
		reprocess(antenna, moved)
	}
}
