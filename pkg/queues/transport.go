package queues

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/streadway/amqp"
	"net"
	"net/url"
)

func (credentials QueueCredentials) URL() string {
	return (&url.URL{Scheme: "amqp", User: url.UserPassword(credentials.User, credentials.Password), Host: net.JoinHostPort(credentials.Host, credentials.Port), Path: "/"}).String()
}

// Subscription preserves each service's existing queue policy explicitly.
// All existing exchanges are durable fanout exchanges; queues differ in lifetime
// and acknowledgement policy and must not silently inherit another service's.
type Subscription struct {
	QueueCredentials
	Exchange, Queue              string
	Durable, AutoDelete, AutoAck bool
	Prefetch                     int
}

type consumerChannel interface {
	ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
	Qos(int, int, bool) error
	QueueBind(string, string, string, bool, amqp.Table) error
	Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error)
}

func configureConsumer(channel consumerChannel, options Subscription) (<-chan amqp.Delivery, error) {
	if err := channel.ExchangeDeclare(options.Exchange, "fanout", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}
	queue, err := channel.QueueDeclare(options.Queue, options.Durable, options.AutoDelete, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("declare queue: %w", err)
	}
	if err := channel.Qos(options.Prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("configure QoS: %w", err)
	}
	if err := channel.QueueBind(queue.Name, "", options.Exchange, false, nil); err != nil {
		return nil, fmt.Errorf("bind queue: %w", err)
	}
	return channel.Consume(queue.Name, "", options.AutoAck, false, false, false, nil)
}
func dispatchDeliveries(deliveries <-chan amqp.Delivery, handle func(amqp.Delivery) error) error {
	for delivery := range deliveries {
		if err := handle(delivery); err != nil {
			return err
		}
	}
	return errors.New("RabbitMQ delivery channel closed")
}
func RunSubscription(options Subscription, handle func(amqp.Delivery) error) error {
	conn, err := amqp.Dial(options.QueueCredentials.URL())
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	defer conn.Close()
	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	deliveries, err := configureConsumer(channel, options)
	if err != nil {
		return err
	}
	return dispatchDeliveries(deliveries, handle)
}

type publishChannel interface {
	Publish(string, string, bool, bool, amqp.Publishing) error
}
type Publisher struct {
	connection   *amqp.Connection
	channel      publishChannel
	exchange     string
	closeChannel func() error
}

func NewPublisher(credentials QueueCredentials, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(credentials.URL())
	if err != nil {
		return nil, err
	}
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := channel.ExchangeDeclare(exchange, "fanout", true, false, false, false, nil); err != nil {
		channel.Close()
		conn.Close()
		return nil, err
	}
	return &Publisher{connection: conn, channel: channel, exchange: exchange, closeChannel: channel.Close}, nil
}
func (p *Publisher) PublishJSON(value interface{}, deliveryMode uint8) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return p.channel.Publish(p.exchange, "", false, false, amqp.Publishing{ContentType: "text/plain", DeliveryMode: deliveryMode, Body: body})
}
func (p *Publisher) Close() {
	if p.closeChannel != nil {
		_ = p.closeChannel()
	}
	if p.connection != nil {
		_ = p.connection.Close()
	}
}
