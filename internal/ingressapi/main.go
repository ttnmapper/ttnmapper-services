package ingressapi

import (
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"ttnmapper-services/internal/ingressapi/types"
	"ttnmapper-services/pkg/config"
	"ttnmapper-services/pkg/queues"
	sharedutil "ttnmapper-services/pkg/utils"
)
import "github.com/go-chi/chi"

var publishChannel = make(chan types.TtnMapperUplinkMessage, 100)

type Configuration struct {
	AmqpHost     string `env:"AMQP_HOST"`
	AmqpPort     string `env:"AMQP_PORT"`
	AmqpUser     string `env:"AMQP_USER"`
	AmqpPassword string `env:"AMQP_PASSWORD"`

	HttpListenAddress string `env:"HTTP_LISTEN_ADDRESS"`
}

var myConfiguration = Configuration{
	AmqpHost:     "localhost",
	AmqpPort:     "5672",
	AmqpUser:     "guest",
	AmqpPassword: "guest",

	HttpListenAddress: ":8080",
}

func Run() {

	err := config.Environment(&myConfiguration)
	if err != nil {
		log.Fatal("Invalid configuration: ", err)
	}

	router := Routes(publishChannel)

	log.Print("[Routes]")
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("%s %s\n", method, route)
		return nil
	}
	if err := chi.Walk(router, walkFunc); err != nil {
		log.Panicf("Logging err: %s\n", err.Error())
	}

	// Start the thread that process the queue
	go publishFromChannel()

	// Start the http endpoint
	log.Fatal(http.ListenAndServe(myConfiguration.HttpListenAddress, router))
}

func publishFromChannel() {
	publisher, err := queues.NewPublisher(queues.QueueCredentials{User: myConfiguration.AmqpUser, Password: myConfiguration.AmqpPassword, Host: myConfiguration.AmqpHost, Port: myConfiguration.AmqpPort}, "new_packets")
	sharedutil.FailOnError(err, "Connect publisher")
	defer publisher.Close()
	for message := range publishChannel {
		sharedutil.FailOnError(publisher.PublishJSON(message, amqp.Persistent), "Publish raw packet")
	}
	log.Fatal("Publish channel closed")
}
