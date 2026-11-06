package main

import (
	"encoding/json"
	"ttnmapper-services/pkg/aggregations"
	"ttnmapper-services/pkg/types"
)

// A new live packet came in. Add it to the appropriate grid cell.
func processNewData() {
	for data := range newDataChannel {
		var message types.TtnMapperUplinkMessage
		if err := json.Unmarshal(data.Body, &message); err != nil {
			continue
		}

		// This aggregation does not use experiment data
		if message.Experiment != "" {
			continue
		}

		aggregations.AggregateNewData(message)
	}
}

// If a gateway moved, delete and rebuild all its grid cells
func processMovedGateway() {
	for data := range gatewayMovedChannel {
		var message types.TtnMapperGatewayMoved
		if err := json.Unmarshal(data.Body, &message); err != nil {
			continue
		}

		aggregations.AggregateMovedGateway(message)
	}
}
