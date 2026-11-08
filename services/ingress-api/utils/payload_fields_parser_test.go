package utils

import (
	"testing"
	"ttnmapper-services/services/ingress-api/types"
)

func TestParsePayloadCoordinates(t *testing.T) {
	var packet types.TtnMapperUplinkMessage
	err := ParsePayloadFields(1, map[string]interface{}{"latitude": -33.9, "longitude": 18.8, "altitude": 10.0}, &packet)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Latitude != -33.9 || packet.Longitude != 18.8 || packet.Altitude != 10 {
		t.Fatalf("coordinates = %+v", packet)
	}
}
