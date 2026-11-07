package web

import (
	"errors"
	"net/http"
	"testing"
	"time"
	"ttnmapper-services/internal/testutil"
)

func TestFetchStatuses(t *testing.T) {
	for _, tc := range []struct {
		name, body     string
		transportError bool
		wantError      bool
	}{
		{"valid", `{"gateway":{"id":"gateway","network":"ttn"}}`, false, false}, {"bad json", `{`, false, true}, {"transport failure", "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.HTTP(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.Header.Get("User-Agent") != "ttnmapper-update-gateway" {
					t.Errorf("unexpected request: %v", req)
				}
				if tc.transportError {
					return nil, errors.New("offline")
				}
				return testutil.JSON(tc.body), nil
			})
			got, err := FetchWebStatuses()
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if !tc.wantError {
				if _, ok := got["gateway"]; !ok {
					t.Fatalf("gateways = %+v", got)
				}
			}
		})
	}
}

func TestWebGatewayConversion(t *testing.T) {
	stamp := time.Unix(1600000000, 123)
	for _, tc := range []struct{ network, id, wantNetwork, eui string }{
		{"ttn", "eui-001122334455aabb", "NS_TTS_V3://ttn@000013", "001122334455AABB"},
		{"ttnv2", "001122334455aabb", "thethingsnetwork.org", "001122334455AABB"},
		{"ttn", "gateway", "NS_TTS_V3://ttn@000013", ""},
	} {
		input := WebGateway{Network: tc.network, ID: tc.id, Name: "test", Description: "description", LastSeen: &stamp}
		input.Location.Latitude = -33.9
		input.Location.Longitude = 18.8
		input.Location.Altitude = 100
		got := WebGatewayToTtnMapperGateway(input)
		if got.NetworkId != tc.wantNetwork || got.GatewayEui != tc.eui || got.Time != stamp.UnixNano() || got.Latitude != -33.9 || got.Longitude != 18.8 || got.Altitude != 100 || got.Attributes["description"] != "description" {
			t.Fatalf("gateway=%+v", got)
		}
	}
}
