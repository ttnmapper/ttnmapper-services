package aggregate

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"ttnmapper-services/pkg/aggregations"
	"ttnmapper-services/pkg/types"
)

// A new live packet came in. Add it to the appropriate grid cell.
func processNewData() {
	processNewDataMessages(newDataChannel, aggregations.AggregateNewData)
}
func processNewDataMessages(input <-chan amqp.Delivery, aggregate func(types.TtnMapperUplinkMessage)) {
	for data := range input {
		var message types.TtnMapperUplinkMessage
		if err := json.Unmarshal(data.Body, &message); err != nil {
			continue
		}

		// This aggregation does not use experiment data
		if message.Experiment != "" {
			continue
		}

		aggregate(message)
	}
}

// If a gateway moved, delete and rebuild all its grid cells
func processMovedGateway() {
	processMovedMessages(gatewayMovedChannel, aggregations.AggregateMovedGateway)
}
func processMovedMessages(input <-chan amqp.Delivery, aggregate func(types.TtnMapperGatewayMoved)) {
	for data := range input {
		var message types.TtnMapperGatewayMoved
		if err := json.Unmarshal(data.Body, &message); err != nil {
			continue
		}

		aggregate(message)
	}
}
