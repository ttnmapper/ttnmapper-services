package main

import (
	"encoding/json"
	"ttnmapper-services/pkg/types"
)

// Keep the existing public live-feed projection while consuming the shared
// internal contract. Explicit empty fields shadow the embedded optional fields.
type liveGateway struct {
	types.TtnMapperGateway
	NetworkId   string                 `json:"network_id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}
type liveUplink struct {
	types.TtnMapperUplinkMessage
	NetworkId      string        `json:"network_id,omitempty"`
	NetworkType    string        `json:"network_type,omitempty"`
	NetworkAddress string        `json:"network_address,omitempty"`
	Gateways       []liveGateway `json:"gateways"`
}

func marshalLiveMessage(message types.TtnMapperUplinkMessage) ([]byte, error) {
	wire := liveUplink{TtnMapperUplinkMessage: message}
	if message.Gateways != nil {
		wire.Gateways = make([]liveGateway, len(message.Gateways))
		for i, gateway := range message.Gateways {
			wire.Gateways[i] = liveGateway{TtnMapperGateway: gateway}
		}
	}
	return json.Marshal(wire)
}
