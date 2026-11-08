package tts

import (
	"github.com/golang/protobuf/proto"
	"go.thethings.network/lorawan-stack/v3/pkg/jsonpb"
	"go.thethings.network/lorawan-stack/v3/pkg/ttnpb"
	"testing"
	"time"
	"ttnmapper-services/internal/ingressapi/types"
)

// This complete synthetic uplink includes the network IDs absent from the old fixture.
const testUplink = `{"end_device_ids":{"device_id":"device","application_ids":{"application_id":"app"}},"received_at":"2026-11-06T12:00:00Z","uplink_message":{"f_port":1,"f_cnt":42,"decoded_payload":{"latitude":-33.9,"longitude":18.8,"altitude":10},"settings":{"frequency":"868100000","data_rate":{"lora":{"bandwidth":125000,"spreading_factor":7,"coding_rate":"4/5"}}},"network_ids":{"tenant_id":"ttn","net_id":"000013","cluster_id":"eu1"},"rx_metadata":[{"gateway_ids":{"gateway_id":"eui-001122334455aabb"},"rssi":-90,"snr":7.5},{"gateway_ids":{"gateway_id":"packetbroker"}}]}}`

func TestUplinkConversion(t *testing.T) {
	for _, encoding := range []string{"json", "protobuf"} {
		t.Run(encoding, func(t *testing.T) {
			var input ttnpb.ApplicationUp
			if err := jsonpb.TTN().Unmarshal([]byte(testUplink), &input); err != nil {
				t.Fatal(err)
			}
			if encoding == "protobuf" {
				data, err := proto.Marshal(&input)
				if err != nil {
					t.Fatal(err)
				}
				input.Reset()
				if err := proto.Unmarshal(data, &input); err != nil {
					t.Fatal(err)
				}
			}
			var got types.TtnMapperUplinkMessage
			if err := DecodeV3Payload(1, input, &got); err != nil {
				t.Fatal(err)
			}
			CopyV3Fields(input, &got)
			if got.AppID != "app" || got.DevID != "device" || got.NetworkId != types.NS_TTS_V3+"://ttn@000013" {
				t.Errorf("identity = %+v", got)
			}
			if got.Latitude != -33.9 || got.Longitude != 18.8 || got.Altitude != 10 {
				t.Errorf("location = %+v", got)
			}
			wantTime := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC).UnixNano()
			if got.Time != wantTime || got.FCnt != 42 || got.FPort != 1 {
				t.Errorf("packet metadata = %+v", got)
			}
			if got.Frequency != 868100000 || got.SpreadingFactor != 7 || got.Bandwidth != 125000 || got.CodingRate != "4/5" {
				t.Errorf("radio = %+v", got)
			}
			if len(got.Gateways) != 1 {
				t.Fatalf("gateways = %+v", got.Gateways)
			}
			gw := got.Gateways[0]
			if gw.GatewayEui != "001122334455AABB" || gw.NetworkId != got.NetworkId || gw.Rssi != -90 || gw.Snr != 7.5 || gw.Attributes["cluster_id"] != "eu1" {
				t.Errorf("gateway = %+v", gw)
			}
		})
	}
}
