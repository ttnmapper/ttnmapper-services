package websocketslive

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"ttnmapper-services/pkg/config"
	"ttnmapper-services/pkg/queues"
	"ttnmapper-services/pkg/types"
	"ttnmapper-services/pkg/utils"
)

var messageChannel = make(chan types.TtnMapperUplinkMessage)

type Configuration struct {
	AmqpHost     string `env:"AMQP_HOST"`
	AmqpPort     string `env:"AMQP_PORT"`
	AmqpUser     string `env:"AMQP_USER"`
	AmqpPassword string `env:"AMQP_PASSWORD"`
	AmqpExchange string `env:"AMQP_EXHANGE"`
	AmqpQueue    string `env:"AMQP_QUEUE"`

	HttpListenAddress string `env:"HTTP_LISTEN_ADDRESS"`
}

var myConfiguration = Configuration{
	AmqpHost:     "localhost",
	AmqpPort:     "5672",
	AmqpUser:     "guest",
	AmqpPassword: "guest",
	AmqpExchange: "new_packets",
	AmqpQueue:    "websockets-live-data",

	HttpListenAddress: ":8080",
}

func Run() {

	err := config.Environment(&myConfiguration)
	if err != nil {
		log.Fatal("Invalid configuration: ", err)
	}

	log.Printf("[Configuration]\n%s\n", utils.PrettyPrint(myConfiguration)) // output: [UserA, UserB]

	go subscribeToRabbit()

	flag.Parse()
	hub := newHub()
	go hub.run()

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Route hit")
		serveExperiment(hub, w, r)
	})

	err = http.ListenAndServe(myConfiguration.HttpListenAddress, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	fmt.Fprintf(w, "TTN Mapper Websocket server")
}

func subscribeToRabbit() {
	options := queues.Subscription{QueueCredentials: queues.QueueCredentials{User: myConfiguration.AmqpUser, Password: myConfiguration.AmqpPassword, Host: myConfiguration.AmqpHost, Port: myConfiguration.AmqpPort}, Exchange: myConfiguration.AmqpExchange, Queue: myConfiguration.AmqpQueue, AutoAck: true, Prefetch: 10}
	err := queues.RunSubscription(options, func(delivery amqp.Delivery) error {
		var message types.TtnMapperUplinkMessage
		if err := json.Unmarshal(delivery.Body, &message); err != nil {
			log.Print(err)
			return nil
		}
		messageChannel <- message
		return nil
	})
	utils.FailOnError(err, "Subscribe channel closed")
}
