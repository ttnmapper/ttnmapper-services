package helium

import (
	"encoding/json"
	"testing"
)

func TestDisk91Conversion(t *testing.T) {
	var input Disk91Snapshot
	if err := json.Unmarshal([]byte(`{"hotspotId":"gateway","animalName":"test","lastSeen":1600000000123,"position":{"lat":-33.9,"lng":18.8,"alt":100,"gain":5.5}}`), &input); err != nil {
		t.Fatal(err)
	}
	got, err := Disk91SnapshotToTtnMapperGateway(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.GatewayId != "gateway" || got.Name != "test" || got.NetworkId != "NS_HELIUM://000024" || got.Time != 1600000000123000000 || got.Latitude != -33.9 || got.Longitude != 18.8 || got.Altitude != 100 || got.Attributes["gain"] != 5.5 {
		t.Fatalf("gateway = %+v", got)
	}
}
