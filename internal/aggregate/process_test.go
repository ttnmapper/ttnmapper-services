package aggregate

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"reflect"
	"testing"
	"time"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func TestProcessNewData(t *testing.T) {
	input := make(chan amqp.Delivery, 4)
	input <- amqp.Delivery{Body: []byte(`{`)}
	input <- amqp.Delivery{Body: []byte(`{"experiment":"excluded"}`)}
	msg := types.TtnMapperUplinkMessage{NetworkId: "network", AppID: "app", DevID: "device", Latitude: 2, Longitude: 1, Gateways: []types.TtnMapperGateway{{GatewayId: "gateway", Rssi: -100}}}
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	input <- amqp.Delivery{Body: body}
	input <- amqp.Delivery{Body: body}
	close(input)
	var got []types.TtnMapperUplinkMessage
	processNewDataMessages(input, func(msg types.TtnMapperUplinkMessage) { got = append(got, msg) })
	if !reflect.DeepEqual(got, []types.TtnMapperUplinkMessage{msg, msg}) {
		t.Fatalf("messages=%+v", got)
	}
}
func TestMovedGatewayRoutesAllAntennasSinceLastMove(t *testing.T) {
	input := make(chan amqp.Delivery, 3)
	input <- amqp.Delivery{Body: []byte(`{`)}
	input <- amqp.Delivery{Body: []byte(`{"network_id":"network","gtw_id":"gateway"}`)}
	close(input)
	moved := time.Unix(1600000000, 0)
	var rebuilt []uint
	routes := 0
	processMovedMessages(input, func(message types.TtnMapperGatewayMoved) {
		routes++
		if message.NetworkId != "network" || message.GatewayId != "gateway" {
			t.Fatalf("message=%+v", message)
		}
		reprocessGateway(database.Gateway{NetworkId: message.NetworkId, GatewayId: message.GatewayId},
			func(network, gateway string) []database.Antenna {
				if network != "network" || gateway != "gateway" {
					t.Fatal("wrong lookup identity")
				}
				return []database.Antenna{{ID: 7}, {ID: 8}}
			},
			func(network, gateway string) time.Time {
				if network != "network" || gateway != "gateway" {
					t.Fatal("wrong move lookup")
				}
				return moved
			},
			func(antenna database.Antenna, since time.Time) {
				if !since.Equal(moved) {
					t.Fatal("wrong replay cutoff")
				}
				rebuilt = append(rebuilt, antenna.ID)
			})
	})
	if routes != 1 || !reflect.DeepEqual(rebuilt, []uint{7, 8}) {
		t.Fatalf("routes=%d antennas=%v", routes, rebuilt)
	}
}
