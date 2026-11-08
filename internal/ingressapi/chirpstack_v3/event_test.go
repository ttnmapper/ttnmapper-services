package chirpstack_v3

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"ttnmapper-services/pkg/types"
)

func TestDecodeV3JSONFormats(t *testing.T) {
	fixture, err := os.ReadFile("chirp_v3_output.json")
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := decodeUplinkEvent(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(fixture, &fields); err != nil {
		t.Fatal(err)
	}
	fields["dev_eui"] = fields["devEUI"]
	delete(fields, "devEUI")
	fields["f_port"] = "1"
	delete(fields, "fPort")
	fields["f_cnt"] = "5"
	delete(fields, "fCnt")
	tx := fields["txInfo"].(map[string]any)
	tx["frequency"] = "868300000"
	tx["modulation"] = 0
	tx["lo_ra_modulation_info"] = tx["loRaModulationInfo"]
	delete(tx, "loRaModulationInfo")
	for _, item := range fields["rxInfo"].([]any) {
		gateway := item.(map[string]any)
		gateway["gateway_id"] = gateway["gatewayID"]
		delete(gateway, "gatewayID")
		gateway["fine_timestamp_type"] = 0
		delete(gateway, "fineTimestampType")
		gateway["location"].(map[string]any)["source"] = 0
	}
	fields["futureField"] = map[string]any{"ignored": true}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeUplinkEvent(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, baseline) {
		t.Fatalf("alternate webhook encoding changed decoded fields:\ngot %+v\nwant %+v", got, baseline)
	}
	if len(got.RxInfo) != 5 || got.RxInfo[0].Time == nil || got.RxInfo[0].Time.Format(time.RFC3339Nano) != "2021-04-20T12:41:05.733901733Z" {
		t.Fatal("gateway timestamps or receptions were lost")
	}
}

func TestDecodeV3FSKAndFineTimestamps(t *testing.T) {
	body := `{"devEUI":"_-4","fCnt":"4294967295","txInfo":{"modulation":"FSK","frequency":868300000,"fskModulationInfo":{"datarate":"50000"}},"rxInfo":[{"gatewayID":"AAEC","rssi":"-105","loRaSNR":"7.5","fineTimestampType":"ENCRYPTED","encryptedFineTimestamp":{"aesKeyIndex":"3","encryptedNS":"_-4="}},{"fineTimestampType":"PLAIN","plainFineTimestamp":{"time":"2021-04-20T12:41:05.123456789Z"}}]}`
	got, err := decodeUplinkEvent([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	var output types.TtnMapperUplinkMessage
	CopyChirpV3Fields(got, &output)
	if output.DevEui != "FFEE" || output.FCnt != 4294967295 || output.Modulation != "FSK" || output.Bitrate != 50000 || output.Frequency != 868300000 {
		t.Fatalf("FSK fields lost: %+v", output)
	}
	gateway := output.Gateways[0]
	if gateway.GatewayId != "eui-000102" || gateway.Rssi != -105 || gateway.Snr != 7.5 || gateway.FineTimestampEncryptedKeyId != "3" || !reflect.DeepEqual(gateway.FineTimestampEncrypted, []byte{255, 238}) {
		t.Fatalf("gateway fields lost: %+v", gateway)
	}
	if got.RxInfo[1].PlainFineTimestamp.Time.Nanosecond() != 123456789 {
		t.Fatal("plain timestamp lost precision")
	}
}

func TestInvalidV3WebhookIsRejectedWithoutPublishing(t *testing.T) {
	validTX := `"txInfo":{"modulation":"LORA","loRaModulationInfo":{"bandwidth":125,"spreadingFactor":7}}`
	for _, body := range []string{
		`null`, `[]`, `{`, `{}`, `{} {}`,
		`{"txInfo":null}`,
		`{"txInfo":{"modulation":"LORA"}}`,
		`{"txInfo":{"modulation":"FSK"}}`,
		`{"txInfo":{"modulation":"bogus"}}`,
		`{"txInfo":{"modulation":2}}`,
		`{` + validTX + `,"devEUI":"invalid!"}`,
		`{` + validTX + `,"fCnt":4294967296}`,
		`{` + validTX + `,"rxInfo":[null]}`,
		`{` + validTX + `,"rxInfo":[{"time":"invalid"}]}`,
		`{` + validTX + `,"rxInfo":[{"fineTimestampType":"PLAIN"}]}`,
		`{` + validTX + `,"rxInfo":[{"fineTimestampType":"ENCRYPTED"}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			messages := make(chan types.TtnMapperUplinkMessage, 1)
			handler := &Context{PublishChannel: messages}
			req := httptest.NewRequest(http.MethodPost, "/events?event=up", strings.NewReader(body))
			req.Header.Set("TTNMAPPERORG-NETWORK", "test.example")
			response := httptest.NewRecorder()
			handler.PostChirpV3Event(response, req)
			if response.Code != http.StatusBadRequest || len(messages) != 0 {
				t.Fatalf("status=%d published=%d", response.Code, len(messages))
			}
		})
	}
}
