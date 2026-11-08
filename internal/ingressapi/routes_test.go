package ingressapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/chirpstack/chirpstack/api/go/v4/common"
	"github.com/chirpstack/chirpstack/api/go/v4/gw"
	"github.com/chirpstack/chirpstack/api/go/v4/integration"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"ttnmapper-services/pkg/types"
)

// Loading this package imports every ingress provider. With the default protobuf
// conflict policy, reintroducing incompatible descriptors fails before tests run.
func TestChirpStackVersionsShareIngress(t *testing.T) {
	v3, err := os.ReadFile("chirpstack_v3/chirp_v3_output.json")
	if err != nil {
		t.Fatal(err)
	}
	object, err := structpb.NewStruct(map[string]any{"latitude": -33.9, "longitude": 18.8, "altitude": 10})
	if err != nil {
		t.Fatal(err)
	}
	event := &integration.UplinkEvent{
		DeviceInfo: &integration.DeviceInfo{ApplicationName: "app", DeviceName: "device", DevEui: "0001020304050607"},
		FPort:      1, FCnt: 42, Object: object,
		TxInfo: &gw.UplinkTxInfo{Frequency: 868300000, Modulation: &gw.Modulation{Parameters: &gw.Modulation_Lora{Lora: &gw.LoraModulationInfo{Bandwidth: 125000, SpreadingFactor: 7, CodeRate: gw.CodeRate_CR_4_5}}}},
		RxInfo: []*gw.UplinkRxInfo{{GatewayId: "0011223344556677", Rssi: -80, Snr: 5, Location: &common.Location{Latitude: -33.8, Longitude: 18.7}}},
	}
	v4JSON, err := protojson.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	v4Proto, err := proto.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan types.TtnMapperUplinkMessage, 1)
	router := Routes(messages)
	for _, test := range []struct {
		name, path, contentType, device string
		body                            []byte
	}{
		{"v3 JSON", "/chirp/v3/events?event=up", "application/json", "Adeunis-Fieldtester-1", v3},
		{"v4 JSON", "/chirp/v4/events?event=up", "application/json", "device", v4JSON},
		{"v4 protobuf", "/chirp/v4/events?event=up", "application/octet-stream", "device", v4Proto},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, test.path, bytes.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			req.Header.Set("TTNMAPPERORG-NETWORK", "test.example")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusAccepted {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			select {
			case message := <-messages:
				if message.DevID != test.device || len(message.Gateways) == 0 || message.NetworkId != types.NS_CHIRP+"://test.example" {
					t.Fatalf("unexpected mapped uplink: %+v", message)
				}
			default:
				t.Fatal("accepted request did not publish")
			}
		})
	}
}
