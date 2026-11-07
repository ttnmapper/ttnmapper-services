package helium

import (
	"testing"
	"time"
)

func TestSnapshotConversion(t *testing.T) {
	stamp := time.Unix(1600000000, 123)
	for _, online := range []string{"online", "offline", "unknown"} {
		t.Run(online, func(t *testing.T) {
			got, err := HotspotSnapshotToTtnMapperGateway(stamp, HotspotSnapshot{Address: "gateway", Name: "test", Online: online, Latitude: -33.9, Longitude: 18.8, Mode: "full"})
			if err != nil {
				t.Fatal(err)
			}
			wantTime := int64(0)
			if online == "online" {
				wantTime = stamp.UnixNano()
			}
			if got.NetworkId != "NS_HELIUM://000024" || got.GatewayId != "gateway" || got.Time != wantTime || got.Latitude != -33.9 || got.Longitude != 18.8 || got.Attributes["mode"] != "full" {
				t.Fatalf("gateway = %+v", got)
			}
		})
	}
}
