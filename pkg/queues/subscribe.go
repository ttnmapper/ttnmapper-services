package queues

import (
	"github.com/streadway/amqp"
	"ttnmapper-services/pkg/utils"
)

func (context *SubscribeContext) Subscribe() {
	options := Subscription{QueueCredentials: context.QueueCredentials, Exchange: context.Exchange, Queue: context.Queue, Durable: true, AutoAck: false, Prefetch: 10}
	err := RunSubscription(options, func(delivery amqp.Delivery) error { context.RxChannel <- delivery; return nil })
	utils.FailOnError(err, "Subscribe channel closed")
}
