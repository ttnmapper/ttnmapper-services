package helium

import (
	"net/http"
	"testing"
	"time"
	"ttnmapper-services/internal/testutil"
)

func TestFetchStatuses(t *testing.T) {
	testutil.HTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("cursor") != "next/+=" {
			t.Errorf("cursor = %q", req.URL.Query().Get("cursor"))
		}
		return testutil.JSON(`{"cursor":"last","data":[{"address":"gateway","name":"test"}]}`), nil
	})
	got, err := FetchStatuses("next/+=")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cursor != "last" || len(got.Data) != 1 || got.Data[0].Address != "gateway" {
		t.Fatalf("response = %+v", got)
	}
}
func TestHotspotConversion(t *testing.T) {
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, online := range []string{"online", "offline", ""} {
		t.Run(online, func(t *testing.T) {
			h := Hotspot{Address: "gateway", Name: "test", Latitude: -33.9, Longitude: 18.8, Elevation: 10, Status: HotspotStatus{Timestamp: stamp, Online: online}}
			got, err := HeliumHotspotToTtnMapperGateway(h)
			if err != nil {
				t.Fatal(err)
			}
			wantTime := int64(0)
			if online == "online" {
				wantTime = stamp.UnixNano()
			}
			if got.NetworkId != "NS_HELIUM://000024" || got.GatewayId != "gateway" || got.Name != "test" || got.Time != wantTime || got.Latitude != -33.9 || got.Longitude != 18.8 || got.Altitude != 10 {
				t.Fatalf("gateway = %+v", got)
			}
		})
	}
}
