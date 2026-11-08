package gatewaychannels

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streadway/amqp"
	"gorm.io/gorm"
	"log"
	"net/http"
	"sync"
	"ttnmapper-services/internal/gatewaychannels/types"
	"ttnmapper-services/pkg/config"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/utils"
)

type Configuration struct {
	AmqpHost                 string `env:"AMQP_HOST"`
	AmqpPort                 string `env:"AMQP_PORT"`
	AmqpUser                 string `env:"AMQP_USER"`
	AmqpPassword             string `env:"AMQP_PASSWORD"`
	AmqpExchangeInsertedData string `env:"AMQP_EXCHANGE_INSERTED"`
	AmqpQueueInsertedData    string `env:"AMQP_QUEUE_INSERTED"`

	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     string `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`

	PrometheusPort string `env:"PROMETHEUS_PORT"`
}

var myConfiguration = Configuration{
	AmqpHost:                 "localhost",
	AmqpPort:                 "5672",
	AmqpUser:                 "user",
	AmqpPassword:             "password",
	AmqpExchangeInsertedData: "inserted_data",
	AmqpQueueInsertedData:    "inserted_data_gateway_channels",

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	PrometheusPort: "9100",
}

var (
	// Prometheus stats
	processedGateways = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ttnmapper_gateway_frequency_processed_count",
		Help: "The total number of gateway frequency updates processed",
	})

	insertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_gateway_frequency_processed_duration",
		Help:    "How long the processing of a gateway frequency took",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})

	// Other global vars
	antennaCache      sync.Map
	frequencyCache    sync.Map
	rawPacketsChannel = make(chan amqp.Delivery)
	db                *gorm.DB
)

func Run() {

	err := config.Environment(&myConfiguration)
	if err != nil {
		log.Fatal("Invalid configuration: ", err)
	}

	log.Printf("[Configuration]\n%s\n", utils.PrettyPrint(myConfiguration)) // output: [UserA, UserB]

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe("0.0.0.0:"+myConfiguration.PrometheusPort, nil)
		if err != nil {
			log.Print(err.Error())
		}
	}()

	var dbErr error
	db, dbErr = database.OpenPostgres(database.PostgresOptions{Host: myConfiguration.PostgresHost, Port: myConfiguration.PostgresPort, User: myConfiguration.PostgresUser, Password: myConfiguration.PostgresPassword, Database: myConfiguration.PostgresDatabase, DebugLog: myConfiguration.PostgresDebugLog})
	if dbErr != nil {
		panic(dbErr)
	}
	conn, dbErr := db.DB()
	if dbErr != nil {
		panic(dbErr)
	}
	defer conn.Close()

	// Create tables if they do not exist
	log.Println("Performing auto migrate")
	db.AutoMigrate(
		&types.AntennaFrequency{},
	)

	// Start amqp listener on this thread - blocking function
	log.Println("Starting AMQP thread")
	subscribeToRabbitRaw()

	log.Printf("Init Complete")
	forever := make(chan bool)
	<-forever
}
