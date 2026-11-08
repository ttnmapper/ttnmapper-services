package queues

import (
	"errors"
	"github.com/streadway/amqp"
	"math"
	"reflect"
	"testing"
)

var transportFailure = errors.New("injected transport failure")

type fakeConsumer struct {
	t          *testing.T
	options    Subscription
	fail       string
	calls      []string
	deliveries chan amqp.Delivery
}

func (f *fakeConsumer) step(name string) error {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return transportFailure
	}
	return nil
}
func (f *fakeConsumer) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	if name != f.options.Exchange || kind != "fanout" || !durable || autoDelete || internal || noWait || args != nil {
		f.t.Error("exchange policy changed")
	}
	return f.step("exchange")
}
func (f *fakeConsumer) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if name != f.options.Queue || durable != f.options.Durable || autoDelete != f.options.AutoDelete || exclusive || noWait || args != nil {
		f.t.Error("queue policy changed")
	}
	return amqp.Queue{Name: "server-assigned-name"}, f.step("queue")
}
func (f *fakeConsumer) Qos(count, size int, global bool) error {
	if count != 10 || size != 0 || global {
		f.t.Error("prefetch changed")
	}
	return f.step("qos")
}
func (f *fakeConsumer) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	if name != "server-assigned-name" || key != "" || exchange != f.options.Exchange || noWait || args != nil {
		f.t.Error("binding changed")
	}
	return f.step("bind")
}
func (f *fakeConsumer) Consume(name, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if name != "server-assigned-name" || consumer != "" || autoAck != f.options.AutoAck || exclusive || noLocal || noWait || args != nil {
		f.t.Error("acknowledgement policy changed")
	}
	return f.deliveries, f.step("consume")
}
func TestConsumerPoliciesAndSetupErrors(t *testing.T) {
	for _, policy := range []struct {
		name                         string
		durable, autoDelete, autoAck bool
	}{
		{"raw insertion", true, false, false}, {"status consumers", false, false, true}, {"APRS", false, true, true},
	} {
		t.Run(policy.name, func(t *testing.T) {
			for _, failure := range []string{"", "exchange", "queue", "qos", "bind", "consume"} {
				t.Run("failure="+failure, func(t *testing.T) {
					options := Subscription{Exchange: "exchange", Queue: "queue", Durable: policy.durable, AutoDelete: policy.autoDelete, AutoAck: policy.autoAck, Prefetch: 10}
					fake := &fakeConsumer{t: t, options: options, fail: failure, deliveries: make(chan amqp.Delivery)}
					got, err := configureConsumer(fake, options)
					if failure == "" {
						if err != nil || got != fake.deliveries {
							t.Fatalf("setup error=%v", err)
						}
					} else if !errors.Is(err, transportFailure) {
						t.Fatalf("error=%v", err)
					}
					want := []string{"exchange", "queue", "qos", "bind", "consume"}
					if failure != "" {
						for i, name := range want {
							if name == failure {
								want = want[:i+1]
								break
							}
						}
					}
					if !reflect.DeepEqual(fake.calls, want) {
						t.Fatalf("setup calls=%v want=%v", fake.calls, want)
					}
				})
			}
		})
	}
}
func TestDispatchStopsAtCallbackFailureAndDoesNotFabricateClosedDeliveries(t *testing.T) {
	input := make(chan amqp.Delivery, 3)
	input <- amqp.Delivery{DeliveryTag: 1}
	input <- amqp.Delivery{DeliveryTag: 2}
	input <- amqp.Delivery{DeliveryTag: 3}
	close(input)
	calls := 0
	err := dispatchDeliveries(input, func(d amqp.Delivery) error {
		calls++
		if d.DeliveryTag == 2 {
			return transportFailure
		}
		return nil
	})
	if !errors.Is(err, transportFailure) || calls != 2 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
	remaining := 0
	err = dispatchDeliveries(input, func(d amqp.Delivery) error {
		remaining++
		if d.DeliveryTag != 3 {
			t.Fatal("fabricated delivery")
		}
		return nil
	})
	if err == nil || remaining != 1 {
		t.Fatalf("remaining=%d error=%v", remaining, err)
	}
}

type fakePublisher struct {
	calls         int
	message       amqp.Publishing
	exchange, key string
	err           error
	t             *testing.T
}

func (f *fakePublisher) Publish(exchange, key string, mandatory, immediate bool, message amqp.Publishing) error {
	f.calls++
	f.exchange = exchange
	f.key = key
	f.message = message
	if mandatory || immediate {
		f.t.Error("publish flags changed")
	}
	return f.err
}
func TestPublisherPreservesWireAndPersistencePolicy(t *testing.T) {
	for _, mode := range []uint8{0, amqp.Persistent} {
		fake := &fakePublisher{t: t}
		publisher := Publisher{channel: fake, exchange: "events"}
		if err := publisher.PublishJSON(struct {
			Time int64 `json:"time"`
		}{1600000000123456789}, mode); err != nil {
			t.Fatal(err)
		}
		if fake.exchange != "events" || fake.key != "" || fake.message.ContentType != "text/plain" || fake.message.DeliveryMode != mode || string(fake.message.Body) != `{"time":1600000000123456789}` {
			t.Fatalf("message=%+v", fake.message)
		}
		fake.err = transportFailure
		if err := publisher.PublishJSON("test", mode); !errors.Is(err, transportFailure) {
			t.Fatalf("publish error=%v", err)
		}
		calls := fake.calls
		if err := publisher.PublishJSON(math.NaN(), mode); err == nil || fake.calls != calls {
			t.Fatal("unencodable value reached transport")
		}
	}
}
func TestCredentialEscapingPreservesUserPasswordAndVhost(t *testing.T) {
	options := QueueCredentials{Host: "localhost", Port: "5672", User: "user@name", Password: "p:/?# word"}
	parsed, err := amqp.ParseURI(options.URL())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Username != options.User || parsed.Password != options.Password || parsed.Host != "localhost" || parsed.Port != 5672 || parsed.Vhost != "/" {
		t.Fatalf("credentials changed during escaping")
	}
}
