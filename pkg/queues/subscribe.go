package queues

import (
	"github.com/streadway/amqp"
	"log"
	"ttnmapper-services/pkg/utils"
)

func (queueContext *SubscribeContext) Subscribe() {
	conn, err := amqp.Dial("amqp://" + queueContext.User + ":" + queueContext.Password + "@" + queueContext.Host + ":" + queueContext.Port + "/")
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// Create a channel for errors
	notify := conn.NotifyClose(make(chan *amqp.Error)) //error channel

	ch, err := conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		queueContext.Exchange, // name
		"fanout",              // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	utils.FailOnError(err, "Failed to declare an exchange")

	q, err := ch.QueueDeclare(
		queueContext.Queue, // name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	utils.FailOnError(err, "Failed to declare a queue")

	err = ch.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	utils.FailOnError(err, "Failed to set queue QoS")

	err = ch.QueueBind(
		q.Name,                // queue name
		"",                    // routing key
		queueContext.Exchange, // exchange
		false,
		nil)
	utils.FailOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
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
			log.Printf(" [a] Packet received")
			queueContext.RxChannel <- d
		}
	}

	log.Fatal("Subscribe channel closed")

}
