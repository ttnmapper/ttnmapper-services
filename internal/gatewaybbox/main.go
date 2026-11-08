package gatewaybbox

import (
	"flag"
	"log"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/utils"

	"github.com/streadway/amqp"
	"gorm.io/gorm/clause"
	"ttnmapper-services/pkg/config"
)

type Configuration struct {
	AmqpHost     string `env:"AMQP_HOST"`
	AmqpPort     string `env:"AMQP_PORT"`
	AmqpUser     string `env:"AMQP_USER"`
	AmqpPassword string `env:"AMQP_PASSWORD"`
	AmqpExchange string `env:"AMQP_EXCHANGE_INSERTED"`
	AmqpQueue    string `env:"AMQP_QUEUE"`

	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     string `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`

	PrometheusPort string `env:"PROMETHEUS_PORT"`
}

var myConfiguration = Configuration{
	AmqpHost:     "localhost",
	AmqpPort:     "5672",
	AmqpUser:     "user",
	AmqpPassword: "password",
	AmqpExchange: "inserted_data",
	AmqpQueue:    "inserted_data_gateway_bbox",

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	PrometheusPort: "9100",
}

var rawPacketsChannel = make(chan amqp.Delivery)

func Run() {
	reprocess := flag.Bool("reprocess", false, "a bool")
	flag.Parse()
	reprocessGateways := flag.Args()

	err := config.Environment(&myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("[Configuration]\n%s\n", utils.PrettyPrint(myConfiguration)) // output: [UserA, UserB]

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
			ReprocessAll()
		}

	} else {
		// Start amqp listener on this thread - blocking function
		log.Println("Starting AMQP thread")
		// todo go subscribeToRabbitGatewayMoved()
		go subscribeToRabbitRaw()
		go processMessages()

		log.Printf("Init Complete")
		forever := make(chan bool)
		<-forever
	}
}

func ReprocessAll() {
	log.Println("All gateways")

	// Get all records
	var gateways []database.Gateway
	database.Db.Find(&gateways)

	for i, gateway := range gateways {
		log.Println(i, "/", len(gateways), " ", gateway.NetworkId, " - ", gateway.GatewayId)
		ReprocessSingleGateway(gateway)
	}
}

func ReprocessGateways(gateways []string) {
	for _, gatewayId := range gateways {
		// The same gateway_id can exist in multiple networks, so iterate them all
		var gateways []database.Gateway
		database.Db.Where("gateway_id = ?", gatewayId).Find(&gateways)

		for i, gateway := range gateways {
			log.Println(i, "/", len(gateways), " ", gateway.NetworkId, " - ", gateway.GatewayId)
			ReprocessSingleGateway(gateway)
		}
	}
}

func ReprocessSingleGateway(gateway database.Gateway) {
	/*
		1. Find all antennas with same network and gateway id
		2. All packets for antennas find min and max lat and lon
		3. Gateway location
	*/
	var antennas []database.Antenna
	database.Db.Where("network_id = ? and gateway_id = ?", gateway.NetworkId, gateway.GatewayId).Find(&antennas)

	var antennaIds []uint
	for _, antenna := range antennas {
		antennaIds = append(antennaIds, antenna.ID)
	}

	log.Println("Antenna IDs: ", antennaIds)

	var result database.GatewayBoundingBox
	result.NetworkId = gateway.NetworkId
	result.GatewayId = gateway.GatewayId

	if len(antennaIds) > 0 {
		database.Db.Raw(`
			SELECT max(latitude) as north, min(latitude) as south FROM
			(
			   SELECT *
			   FROM packets
			   WHERE antenna_id IN ?
			) t
			WHERE latitude != 0 AND experiment_id IS NULL
		`, antennaIds).Scan(&result)
		database.Db.Raw(`
			SELECT max(longitude) as east, min(longitude) as west FROM
			(
			   SELECT *
			   FROM packets
			   WHERE antenna_id IN ?
			) t
			WHERE longitude != 0 AND experiment_id IS NULL
		`, antennaIds).Scan(&result)
	}

	// Take gateway location also into account
	if gateway.Latitude == 0 && gateway.Longitude == 0 {
		// Gateway location not set
	} else {
		log.Println("Gateway location:", gateway.Latitude, gateway.Longitude)

		if result.North == 0 || gateway.Latitude > result.North {
			result.North = gateway.Latitude
		}
		if result.South == 0 || gateway.Latitude < result.South {
			result.South = gateway.Latitude
		}
		if result.East == 0 || gateway.Longitude > result.East {
			result.East = gateway.Longitude
		}
		if result.West == 0 || gateway.Longitude < result.West {
			result.West = gateway.Longitude
		}

		log.Println(utils.PrettyPrint(result))
	}

	if result.North == 0 && result.South == 0 && result.East == 0 && result.West == 0 {
		log.Println("Bounds zero, not updating")
	} else {
		database.Db.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&result)
	}
}
