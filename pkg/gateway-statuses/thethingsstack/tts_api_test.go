package thethingsstack

import (
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.thethings.network/lorawan-stack/v3/pkg/ttnpb"
	"testing"
)

func TestGatewayConversion(t *testing.T) {
	gateway := Gateway{Ids: GatewayIds{GatewayId: "gateway", Eui: "001122334455AABB"}, Name: "test", Description: "description", GatewayServerAddress: "eu1"}
	if _, err := TtsApiGatewayToTtnMapperGateway("tenant", gateway, nil); err == nil {
		t.Fatal("missing status accepted")
	}
	for _, tc := range []struct {
		name                 string
		status, uplink, want int64
	}{
		{"status newer", 200, 100, 200}, {"uplink newer", 100, 200, 200}, {"same", 100, 100, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TtsApiGatewayToTtnMapperGateway("tenant", gateway, &ttnpb.GatewayConnectionStats{LastStatusReceivedAt: &timestamp.Timestamp{Seconds: tc.status}, LastUplinkReceivedAt: &timestamp.Timestamp{Seconds: tc.uplink}})
			if err != nil {
				t.Fatal(err)
			}
			if got.NetworkId != "NS_TTS_V3://tenant@000013" || got.GatewayId != "gateway" || got.GatewayEui != "001122334455AABB" || got.Time != tc.want*1000000000 || got.Attributes["description"] != "description" || got.Attributes["cluster_id"] != "eu1" {
				t.Fatalf("gateway = %+v", got)
			}
		})
	}
}
