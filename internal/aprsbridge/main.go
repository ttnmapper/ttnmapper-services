package aprsbridge

import (
	"encoding/json"
	"fmt"
	"github.com/ebarkie/aprs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streadway/amqp"
	"github.com/tkanos/gonfig"
	"log"
	"net/http"
	"ttnmapper-services/internal/aprsbridge/rabbit"
	"ttnmapper-services/internal/aprsbridge/sqldb"
	"ttnmapper-services/internal/aprsbridge/types"
	"ttnmapper-services/internal/aprsbridge/utils"
)

type Configuration struct {
	AmqpHost                 string `env:"AMQP_HOST"`
	AmqpPort                 int    `env:"AMQP_PORT"`
	AmqpUser                 string `env:"AMQP_USER"`
	AmqpPassword             string `env:"AMQP_PASSWORD"`
	AmqpExchangeRawData      string `env:"AMQP_EXCHANGE_RAW"`
	AmqpQueue                string `env:"AMQP_QUEUE"`
	AmqpExchangeInsertedData string `env:"AMQP_EXCHANGE_INSERTED"`

	PostgresHost          string `env:"POSTGRES_HOST"`
	PostgresPort          int    `env:"POSTGRES_PORT"`
	PostgresUser          string `env:"POSTGRES_USER"`
	PostgresPassword      string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase      string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog      bool   `env:"POSTGRES_DEBUG_LOG"`
	PostgresInsertThreads int    `env:"POSTGRES_INSERT_THREADS"`

	AprsServer        string `env:"APRS_SERVER"`
	AprsRelayUser     string `env:"APRS_USER"`
	AprsRelayPassword int    `env:"APRS_PASSWORD"`

	PrometheusPort string `env:"PROMETHEUS_PORT"`
}

var myConfiguration = Configuration{
	AmqpHost:                 "localhost",
	AmqpPort:                 5672,
	AmqpUser:                 "user",
	AmqpPassword:             "password",
	AmqpExchangeRawData:      "new_packets",
	AmqpQueue:                "aprs_bridge",
	AmqpExchangeInsertedData: "inserted_data",

	PostgresHost:          "localhost",
	PostgresPort:          5432,
	PostgresUser:          "username",
	PostgresPassword:      "password",
	PostgresDatabase:      "database",
	PostgresDebugLog:      false,
	PostgresInsertThreads: 1,

	AprsServer:        "",
	AprsRelayUser:     "",
	AprsRelayPassword: 0,

	PrometheusPort: "9100",
}

func Run() {

	err := gonfig.GetConf("conf.json", &myConfiguration)
	if err != nil {
		log.Println(err)
	}

	log.Printf("[Configuration]\n%s\n", utils.PrettyPrint(myConfiguration)) // output: [UserA, UserB]

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe("0.0.0.0:"+myConfiguration.PrometheusPort, nil)
		if err != nil {
			log.Print(err.Error())
		}
	}()

	messageChannel := make(chan amqp.Delivery)

	// Connect to database
	log.Println("Initialising database")
	sqlCredentials := sqldb.Credentials{
		Host:     myConfiguration.PostgresHost,
		Port:     myConfiguration.PostgresPort,
		User:     myConfiguration.PostgresUser,
		Password: myConfiguration.PostgresPassword,
		Database: myConfiguration.PostgresDatabase,

		DebugLog: myConfiguration.PostgresDebugLog,
	}
	sqlCredentials.Init()

	// Start amqp listener on this thread - blocking function
	log.Println("Starting AMQP thread")
	rabbitCredentials := rabbit.Credentials{
		Host:     myConfiguration.AmqpHost,
		Port:     myConfiguration.AmqpPort,
		User:     myConfiguration.AmqpUser,
		Password: myConfiguration.AmqpPassword,
		Exchange: myConfiguration.AmqpExchangeRawData,
		Queue:    myConfiguration.AmqpQueue,
		AutoAck:  true,

		MessagesChannel: messageChannel,
	}
	go rabbitCredentials.SubscribeToRabbit()

	aprsFramesChannel := make(chan aprs.Frame)
	go aprs.SendTCPFromChannel(myConfiguration.AprsServer,
		aprs.Addr{Call: myConfiguration.AprsRelayUser},
		myConfiguration.AprsRelayPassword, aprsFramesChannel)

	ProcessMessages(messageChannel, aprsFramesChannel)
}

func ProcessMessages(messageChannel chan amqp.Delivery, aprsFramesChannel chan aprs.Frame) {
	processMessages(messageChannel, aprsFramesChannel, sqldb.IsAprsDevice, sqldb.GetAprsDevice)
	log.Fatal("Messages channel closed")
}

// processMessages consumes a finite stream; database access is supplied by the caller.
func processMessages(messageChannel <-chan amqp.Delivery, aprsFramesChannel chan<- aprs.Frame,
	isDevice func(string, string, string) bool,
	getDevice func(string, string, string) (sqldb.AprsDevice, error)) {
	for d := range messageChannel {
		//log.Printf(" [p] Processing packet")

		// The message from amqp is a json string. Unmarshal to ttnmapper uplink struct
		var message types.TtnMapperUplinkMessage
		if err := json.Unmarshal(d.Body, &message); err != nil {
			log.Print(" [p] ", err)
			continue
		}

		if !isDevice(message.NetworkId, message.AppID, message.DevID) {
			//log.Println(" [p] Not APRS device")
			continue
		}
		log.Println(" [p] Is APRS device")

		if !MinimumReportDurationPassed(message) {
			log.Println("Skipping message, as it's too soon after previous")
			continue
		}

		aprsDevice, err := getDevice(message.NetworkId, message.AppID, message.DevID)
		if err != nil {
			continue
		}

		position := aprs.Position{
			Latitude:    message.Latitude,
			Longitude:   message.Longitude,
			Altitude:    message.Altitude,
			SymbolTable: aprsDevice.SymbolTable,
			Symbol:      aprsDevice.Symbol,
			Message:     "",
		}

		frame := aprs.Frame{
			Src:  aprs.Addr{Call: aprsDevice.Callsign, SSID: aprsDevice.Ssid, Repeated: false},
			Dst:  aprs.Addr{Call: "APRS"},
			Path: aprs.Path{aprs.Addr{Call: "TCPIP", Repeated: true}},
			Text: fmt.Sprintf("%s", position),
		}

		aprsFramesChannel <- frame

		CacheReportedMessage(message)
	}

}
