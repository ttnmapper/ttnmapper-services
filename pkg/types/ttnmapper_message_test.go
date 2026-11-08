package types

import (
	"encoding/json"
	"testing"
)

func TestMessageWireContract(t *testing.T) {
	const fixture = `{"network_id":"NS_TTS_V3://ttn@000013","app_id":"app","dev_id":"device","time":1600000000123456789,"port":1,"frequency":868100000,"gateways":[{"network_id":"NS_TTS_V3://ttn@000013","gtw_id":"gateway","antenna_index":0,"description":"legacy description","name":"gateway name","attributes":{"source":"test"},"rssi":-100}]}`
	var message TtnMapperUplinkMessage
	if err := json.Unmarshal([]byte(fixture), &message); err != nil {
		t.Fatal(err)
	}
	if message.Time != 1600000000123456789 || message.NetworkId != "NS_TTS_V3://ttn@000013" || len(message.Gateways) != 1 || message.Gateways[0].NetworkId != message.NetworkId || message.Gateways[0].Description != "legacy description" {
		t.Fatalf("message=%+v", message)
	}
	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip TtnMapperUplinkMessage
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Time != message.Time || roundTrip.Gateways[0].Name != "gateway name" || roundTrip.Gateways[0].Attributes["source"] != "test" {
		t.Fatalf("round trip=%s", data)
	}
}
