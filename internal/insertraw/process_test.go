package insertraw

import (
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	"reflect"
	"testing"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

type acknowledger struct {
	calls  int
	err    error
	events *[]string
}

func (a *acknowledger) Ack(tag uint64, multiple bool) error {
	a.calls++
	if a.events != nil {
		*a.events = append(*a.events, "ack")
	}
	if tag != 7 || multiple {
		return errors.New("wrong acknowledgement")
	}
	return a.err
}
func (*acknowledger) Nack(uint64, bool, bool) error { panic("unexpected nack") }
func (*acknowledger) Reject(uint64, bool) error     { panic("unexpected reject") }
func makeDelivery(t *testing.T, message types.TtnMapperUplinkMessage, ack *acknowledger) amqp.Delivery {
	t.Helper()
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	return amqp.Delivery{Body: body, DeliveryTag: 7, Acknowledger: ack}
}
func TestDeliveryFiltersAndForwardsStoredGateways(t *testing.T) {
	var events []string
	ack := &acknowledger{events: &events}
	msg := types.TtnMapperUplinkMessage{Latitude: -33.9, Longitude: 18.8, Experiment: "test", Gateways: []types.TtnMapperGateway{{GatewayId: "boundary", Rssi: -50}, {GatewayId: "strong", Rssi: -49.9}, {GatewayId: "weak", Rssi: -120}}}
	store := packetStore{
		convert: func(message types.TtnMapperUplinkMessage, gw types.TtnMapperGateway) (database.Packet, error) {
			if !reflect.DeepEqual(message, msg) {
				t.Error("conversion message changed")
			}
			events = append(events, "convert:"+gw.GatewayId)
			return database.Packet{Rssi: gw.Rssi}, nil
		},
		insert: func(packet *database.Packet) error {
			events = append(events, "insert")
			if packet.Rssi > -50 {
				t.Error("strong packet inserted")
			}
			return nil
		},
	}
	published := 0
	err := processDelivery(makeDelivery(t, msg, ack), store, func(got types.TtnMapperUplinkMessage) {
		events = append(events, "publish")
		published++
		want := msg
		want.Gateways = []types.TtnMapperGateway{msg.Gateways[0], msg.Gateways[2]}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("published=%+v", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"convert:boundary", "insert", "convert:weak", "insert", "ack", "publish"}
	if published != 1 || ack.calls != 1 || !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%v", events)
	}
}
func TestDiscardedDeliveriesAreAcknowledgedWithoutPublishing(t *testing.T) {
	for _, tc := range []struct {
		name      string
		message   types.TtnMapperUplinkMessage
		malformed bool
	}{
		{name: "malformed", malformed: true},
		{name: "null island", message: types.TtnMapperUplinkMessage{Experiment: "test", Gateways: []types.TtnMapperGateway{{Rssi: -100}}}},
		{name: "no gateways", message: types.TtnMapperUplinkMessage{Latitude: 1, Longitude: 2}},
		{name: "all filtered", message: types.TtnMapperUplinkMessage{Latitude: 1, Longitude: 2, Gateways: []types.TtnMapperGateway{{Rssi: 2}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ack := &acknowledger{}
			delivery := makeDelivery(t, tc.message, ack)
			if tc.malformed {
				delivery.Body = []byte(`{`)
			}
			if err := processDelivery(delivery, packetStore{}, func(types.TtnMapperUplinkMessage) { t.Fatal("discarded message published") }); err != nil {
				t.Fatal(err)
			}
			if ack.calls != 1 {
				t.Fatalf("acknowledgements=%d", ack.calls)
			}
		})
	}
}
func TestDeliveryFailuresDoNotPublish(t *testing.T) {
	failure := errors.New("failed")
	for _, stage := range []string{"convert first", "insert first", "convert second", "insert second", "ack"} {
		t.Run(stage, func(t *testing.T) {
			ack := &acknowledger{}
			if stage == "ack" {
				ack.err = failure
			}
			msg := types.TtnMapperUplinkMessage{Latitude: 1, Longitude: 2, Gateways: []types.TtnMapperGateway{{GatewayId: "first", Rssi: -100}, {GatewayId: "second", Rssi: -100}}}
			converts, inserts := 0, 0
			store := packetStore{
				convert: func(_ types.TtnMapperUplinkMessage, gw types.TtnMapperGateway) (database.Packet, error) {
					converts++
					if stage == "convert "+gw.GatewayId {
						return database.Packet{}, failure
					}
					return database.Packet{ID: uint(converts)}, nil
				},
				insert: func(packet *database.Packet) error {
					inserts++
					if stage == "insert first" && packet.ID == 1 || stage == "insert second" && packet.ID == 2 {
						return failure
					}
					return nil
				},
			}
			err := processDelivery(makeDelivery(t, msg, ack), store, func(types.TtnMapperUplinkMessage) { t.Fatal("failed delivery published") })
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			if stage != "ack" && ack.calls != 0 {
				t.Fatal("failed write acknowledged")
			}
			if stage == "convert first" && inserts != 0 {
				t.Fatal("conversion failure inserted a row")
			}
			if stage == "insert first" && converts != 1 {
				t.Fatal("processing continued after insertion failure")
			}
		})
	}
}
