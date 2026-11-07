package packet_broker

import (
	"net/http"
	"testing"
	"ttnmapper-services/internal/testutil"
	"ttnmapper-services/pkg/gateway-statuses/packet_broker/Openapi"
)

func TestFetchStatuses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{"page", `[{"id":"gateway","netID":"000013"}]`, 200, false},
		{"empty", `[]`, 200, true}, {"server error", `{}`, 500, true}, {"malformed", `{`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.HTTP(t, func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if q.Get("offset") != "2000" || q.Get("limit") != "1000" || q.Get("online") != "true" {
					t.Errorf("query = %s", req.URL.RawQuery)
				}
				res := testutil.JSON(tc.body)
				res.StatusCode = tc.status
				return res, nil
			})
			got, err := FetchStatuses(2)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if !tc.wantError && (len(got) != 1 || got[0].Id != "gateway") {
				t.Fatalf("gateways = %+v", got)
			}
		})
	}
}
func TestGatewayConversion(t *testing.T) {
	for _, tenant := range []string{"ttn", "ttnv2"} {
		t.Run(tenant, func(t *testing.T) {
			input := Openapi.Gateway{}
			input.TenantID = &tenant
			input.NetID = "000013"
			input.Id = "eui-001122334455aabb"
			got, err := PbGatewayToTtnMapperGateway(input)
			if err != nil {
				t.Fatal(err)
			}
			wantNetwork := "NS_TTS_V3://ttn@000013"
			if tenant == "ttnv2" {
				wantNetwork = "thethingsnetwork.org"
			}
			if got.NetworkId != wantNetwork || got.GatewayEui != "001122334455AABB" || got.Time != 0 {
				t.Fatalf("gateway = %+v", got)
			}
		})
	}
	if _, err := PbGatewayToTtnMapperGateway(Openapi.Gateway{}); err == nil {
		t.Fatal("missing tenant accepted")
	}
}
func TestTenantNetIdToNetworkID(t *testing.T) {
	for _, tc := range []struct {
		net          uint32
		tenant, want string
	}{
		{0x13, "ttn", "NS_TTS_V3://ttn@000013"}, {0xabcdef, "private", "NS_TTS_V3://private@ABCDEF"}, {0x13, "", "NS://000013"},
	} {
		if got := TenantNetIdToNetworkID(tc.net, tc.tenant); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
