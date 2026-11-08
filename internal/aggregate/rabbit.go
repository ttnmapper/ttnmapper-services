package aggregate

import (
	"github.com/streadway/amqp"
	"ttnmapper-services/pkg/queues"
	"ttnmapper-services/pkg/utils"
)

var (
	newDataChannel      = make(chan amqp.Delivery)
	gatewayMovedChannel = make(chan amqp.Delivery)
)

func subscribeToRabbitNewData() {
	options := queues.Subscription{QueueCredentials: queues.QueueCredentials{User: myConfiguration.AmqpUser, Password: myConfiguration.AmqpPassword, Host: myConfiguration.AmqpHost, Port: myConfiguration.AmqpPort}, Exchange: myConfiguration.AmqpExchangeInsertedData, Queue: myConfiguration.AmqpQueueInsertedData, AutoAck: true, Prefetch: 10}
	err := queues.RunSubscription(options, func(delivery amqp.Delivery) error { newDataChannel <- delivery; return nil })
	utils.FailOnError(err, "Subscribe channel closed")
}
func subscribeToRabbitMovedGateway() {
	options := queues.Subscription{QueueCredentials: queues.QueueCredentials{User: myConfiguration.AmqpUser, Password: myConfiguration.AmqpPassword, Host: myConfiguration.AmqpHost, Port: myConfiguration.AmqpPort}, Exchange: myConfiguration.AmqpExchangeGatewayMoved, Queue: myConfiguration.AmqpQueueGatewayMoved, AutoAck: true, Prefetch: 10}
	err := queues.RunSubscription(options, func(delivery amqp.Delivery) error { gatewayMovedChannel <- delivery; return nil })
	utils.FailOnError(err, "Subscribe channel closed")
}
