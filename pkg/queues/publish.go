package queues

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"log"
	"ttnmapper-services/pkg/utils"
)

func (queueContext *PublishContext) Publish() {
	conn, err := amqp.Dial("amqp://" + queueContext.User + ":" + queueContext.Password + "@" + queueContext.Host + ":" + queueContext.Port + "/")
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	newDataAmqpChannel, err := conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer newDataAmqpChannel.Close()

	err = newDataAmqpChannel.ExchangeDeclare(
		queueContext.Exchange, // name
		"fanout",              // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	utils.FailOnError(err, "Failed to declare an exchange")

	for message := range queueContext.TxChannel {

		messageJsonData, err := json.Marshal(message)
		if err != nil {
			log.Println("\t\tCan't marshal message to json")
			return
		}

		err = newDataAmqpChannel.Publish(
			queueContext.Exchange, // exchange
			"",                    // routing key
			false,                 // mandatory
			false,                 // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        messageJsonData,
			})
		utils.FailOnError(err, "Failed to publish a message")

		log.Printf("[I] Published inserted to AMQP exchange")
	}

	log.Fatal("Publish channel closed")
}
