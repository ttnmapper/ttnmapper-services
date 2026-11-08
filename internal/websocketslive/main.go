package websocketslive

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"ttnmapper-services/internal/websocketslive/types"
	"ttnmapper-services/pkg/config"
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

	err := config.Load("conf.json", &myConfiguration)
	if err != nil {
		fmt.Println(err)
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
	conn, err := amqp.Dial("amqp://" + myConfiguration.AmqpUser + ":" + myConfiguration.AmqpPassword + "@" + myConfiguration.AmqpHost + ":" + myConfiguration.AmqpPort + "/")
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// Create a channel for errors
	notify := conn.NotifyClose(make(chan *amqp.Error)) //error channel

	ch, err := conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		myConfiguration.AmqpExchange, // name
		"fanout",                     // type
		true,                         // durable
		false,                        // auto-deleted
		false,                        // internal
		false,                        // no-wait
		nil,                          // arguments
	)
	utils.FailOnError(err, "Failed to declare an exchange")

	q, err := ch.QueueDeclare(
		myConfiguration.AmqpQueue, // name
		false,                     // durable
		false,                     // delete when usused
		false,                     // exclusive
		false,                     // no-wait
		nil,                       // arguments
	)
	utils.FailOnError(err, "Failed to declare a queue")

	err = ch.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	utils.FailOnError(err, "Failed to set queue QoS")

	err = ch.QueueBind(
		q.Name,                       // queue name
		"",                           // routing key
		myConfiguration.AmqpExchange, // exchange
		false,
		nil)
	utils.FailOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	utils.FailOnError(err, "Failed to register a consumer")

waitForMessages:
	for {
		select {
		case err := <-notify:
			if err != nil {
				log.Println(err.Error())
			}
			break waitForMessages
		case d := <-msgs:
			log.Print(" [a] Packet received")

			//log.Printf(" [a] %s", d.Body)
			var message types.TtnMapperUplinkMessage
			if err := json.Unmarshal(d.Body, &message); err != nil {
				log.Print(" [a] " + err.Error())
				continue
			}

			messageChannel <- message
		}
	}
	log.Fatal("Subscribe channel closed")
}
