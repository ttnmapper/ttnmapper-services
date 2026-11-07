package noc

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
		{"valid", `{"statuses":{"gateway":{"timestamp":"2020-01-02T03:04:05Z"}}}`, false, false}, {"bad json", `{`, false, true}, {"transport failure", "", true, true},
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
			got, err := FetchNocStatuses()
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

func TestConversionIgnoresNocLocation(t *testing.T) {
	input := NocGateway{}
	input.Timestamp = time.Unix(1600000000, 123)
	input.Location.Latitude = 52
	input.Location.Longitude = 4
	got := NocGatewayToTtnMapperGateway("gateway", input)
	if got.NetworkId != "thethingsnetwork.org" || got.GatewayId != "gateway" || got.Time != 1600000000000000123 || got.Latitude != 0 || got.Longitude != 0 {
		t.Fatalf("gateway=%+v", got)
	}
}
