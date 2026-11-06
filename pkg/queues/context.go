package queues

import (
	"github.com/streadway/amqp"
	"ttnmapper-services/pkg/types"
)

type QueueCredentials struct {
	User     string
	Password string
	Host     string
	Port     string
}

type SubscribeContext struct {
	QueueCredentials
	Exchange  string
	Queue     string
	RxChannel chan amqp.Delivery
}

type PublishContext struct {
	QueueCredentials
	Exchange  string
	TxChannel chan types.TtnMapperUplinkMessage
}
