package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"ttnmapper-services/pkg/types"
)

func TestLiveWireProjectionRemainsCompatible(t *testing.T) {
	message := types.TtnMapperUplinkMessage{NetworkId: "internal-network", NetworkType: "legacy", NetworkAddress: "legacy-host", AppID: "app", DevID: "device", Time: 123, Gateways: []types.TtnMapperGateway{{NetworkId: "internal-network", GatewayId: "gateway", Name: "name", Description: "description", Attributes: map[string]interface{}{"private": "attribute"}, Rssi: -100}}}
	data, err := marshalLiveMessage(message)
	if err != nil {
		t.Fatal(err)
	}
	var got, want interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"app_id":"app","dev_id":"device","time":123,"gateways":[{"gtw_id":"gateway","antenna_index":0,"rssi":-100}]}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("live wire contract changed: %s", data)
	}
}
