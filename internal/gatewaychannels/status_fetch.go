package gatewaychannels

import (
	"github.com/streadway/amqp"
	"ttnmapper-services/pkg/queues"
	"ttnmapper-services/pkg/utils"
)

func subscribeToRabbitRaw() {
	go func() {
		options := queues.Subscription{QueueCredentials: queues.QueueCredentials{User: myConfiguration.AmqpUser, Password: myConfiguration.AmqpPassword, Host: myConfiguration.AmqpHost, Port: myConfiguration.AmqpPort}, Exchange: myConfiguration.AmqpExchangeInsertedData, Queue: myConfiguration.AmqpQueueInsertedData, AutoAck: true, Prefetch: 10}
		err := queues.RunSubscription(options, func(delivery amqp.Delivery) error { rawPacketsChannel <- delivery; return nil })
		utils.FailOnError(err, "Subscribe channel closed")
	}()
}
