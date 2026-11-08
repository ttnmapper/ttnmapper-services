package rabbit

import (
	"github.com/streadway/amqp"
	"strconv"
	"ttnmapper-services/pkg/queues"
	"ttnmapper-services/pkg/utils"
)

type Credentials struct {
	Host     string
	Port     int
	User     string
	Password string
	Exchange string
	Queue    string
	AutoAck  bool

	MessagesChannel chan amqp.Delivery
}

func (credentials *Credentials) SubscribeToRabbit() {
	options := queues.Subscription{QueueCredentials: queues.QueueCredentials{Host: credentials.Host, Port: strconv.Itoa(credentials.Port), User: credentials.User, Password: credentials.Password}, Exchange: credentials.Exchange, Queue: credentials.Queue, AutoAck: credentials.AutoAck, AutoDelete: true, Prefetch: 10}
	err := queues.RunSubscription(options, func(delivery amqp.Delivery) error { credentials.MessagesChannel <- delivery; return nil })
	utils.FailOnError(err, "Subscribe channel closed")
}
