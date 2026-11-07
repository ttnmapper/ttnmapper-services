package main

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"time"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

type packetStore struct {
	convert func(types.TtnMapperUplinkMessage, types.TtnMapperGateway) (database.Packet, error)
	insert  func(*database.Packet) error
}

// A delivery is acknowledged only after all eligible gateways are stored.
// Conversion/write failures leave it unacknowledged for the existing worker
// failure/restart policy. Earlier writes are not rolled back.
func processDelivery(delivery amqp.Delivery, store packetStore, publish func(types.TtnMapperUplinkMessage)) error {
	var message types.TtnMapperUplinkMessage
	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		return delivery.Ack(false)
	}
	if message.Latitude == 0 && message.Longitude == 0 {
		invalidLocations.Inc()
		return delivery.Ack(false)
	}
	accepted := make([]types.TtnMapperGateway, 0, len(message.Gateways))
	for _, gateway := range message.Gateways {
		if gateway.Rssi > -50 {
			continue
		}
		start := time.Now()
		entry, err := store.convert(message, gateway)
		if err != nil {
			return fmt.Errorf("convert gateway %s: %w", gateway.GatewayId, err)
		}
		if err := store.insert(&entry); err != nil {
			return fmt.Errorf("insert gateway %s: %w", gateway.GatewayId, err)
		}
		dbInserts.Inc()
		insertDuration.Observe(float64(time.Since(start)) / float64(time.Millisecond))
		accepted = append(accepted, gateway)
	}
	if err := delivery.Ack(false); err != nil {
		return fmt.Errorf("acknowledge delivery: %w", err)
	}
	if len(accepted) > 0 {
		message.Gateways = accepted
		publish(message)
	}
	return nil
}
