package main

import (
	"github.com/streadway/amqp"
	"log"
)

func subscribeToRabbitRaw() {
	// Start thread that listens for new amqp messages
	go func() {
		conn, err := amqp.Dial("amqp://" + myConfiguration.AmqpUser + ":" + myConfiguration.AmqpPassword + "@" + myConfiguration.AmqpHost + ":" + myConfiguration.AmqpPort + "/")
		failOnError(err, "Failed to connect to RabbitMQ")
		defer conn.Close()

		// Create a channel for errors
		notify := conn.NotifyClose(make(chan *amqp.Error)) //error channel

		ch, err := conn.Channel()
		failOnError(err, "Failed to open a channel")
		defer ch.Close()

		err = ch.ExchangeDeclare(
			myConfiguration.AmqpExchangeInsertedData, // name
			"fanout",                                 // type
			true,                                     // durable
			false,                                    // auto-deleted
			false,                                    // internal
			false,                                    // no-wait
			nil,                                      // arguments
		)
		failOnError(err, "Failed to declare an exchange")

		q, err := ch.QueueDeclare(
			myConfiguration.AmqpQueueInsertedData, // name
			false,                                 // durable
			false,                                 // delete when unused
			false,                                 // exclusive
			false,                                 // no-wait
			nil,                                   // arguments
		)
		failOnError(err, "Failed to declare a queue")

		err = ch.Qos(
			10,    // prefetch count
			0,     // prefetch size
			false, // global
		)
		failOnError(err, "Failed to set queue QoS")

		err = ch.QueueBind(
			q.Name,                                   // queue name
			"",                                       // routing key
			myConfiguration.AmqpExchangeInsertedData, // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")

		msgs, err := ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			false,  // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // args
		)
		failOnError(err, "Failed to register a consumer")

		log.Println("AMQP started")

	waitForMessages:
		for {
			select {
			case err := <-notify:
				if err != nil {
					log.Println(err.Error())
				}
				break waitForMessages
			case d := <-msgs:
				log.Printf(" [a] Packet received")
				rawPacketsChannel <- d
			}
		}

		log.Fatal("Subscribe channel closed")
	}()

	// Start the thread that processes new amqp messages
	go func() {
		processRawPackets()
	}()
}
