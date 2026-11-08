package aprsbridge

import (
	"encoding/json"
	"errors"
	"github.com/ebarkie/aprs"
	"github.com/streadway/amqp"
	"testing"
	"time"
	"ttnmapper-services/internal/aprsbridge/sqldb"
	"ttnmapper-services/pkg/types"
)

func TestReportInterval(t *testing.T) {
	message := types.TtnMapperUplinkMessage{NetworkId: "network", AppID: "app", DevID: t.Name(), Time: time.Second.Nanoseconds()}
	key := GetLastReportCacheKey(message)
	lastReportCache.Delete(key)
	t.Cleanup(func() { lastReportCache.Delete(key) })
	if !MinimumReportDurationPassed(message) {
		t.Fatal("first report rejected")
	}
	CacheReportedMessage(message)
	for _, tc := range []struct {
		name  string
		delta time.Duration
		want  bool
	}{
		{"duplicate", 0, false}, {"older", -time.Second, false}, {"too soon", 29 * time.Second, false}, {"boundary", 30 * time.Second, false}, {"after", 30*time.Second + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := message
			current.Time += tc.delta.Nanoseconds()
			if got := MinimumReportDurationPassed(current); got != tc.want {
				t.Errorf("allowed=%v, want %v", got, tc.want)
			}
		})
	}
	for _, field := range []string{"network", "app", "device"} {
		other := message
		switch field {
		case "network":
			other.NetworkId = "other"
		case "app":
			other.AppID = "other"
		case "device":
			other.DevID = "other"
		}
		if !MinimumReportDurationPassed(other) {
			t.Errorf("%s not isolated", field)
		}
	}
}
func TestProcessMessages(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		registered, lookupError bool
		wantFrames              int
	}{
		{"registered", true, false, 1}, {"unregistered", false, false, 0}, {"lookup failure", true, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := types.TtnMapperUplinkMessage{NetworkId: "network", AppID: "app", DevID: t.Name(), Latitude: -33.9, Longitude: 18.8, Altitude: 10, Time: time.Second.Nanoseconds()}
			key := GetLastReportCacheKey(message)
			lastReportCache.Delete(key)
			t.Cleanup(func() { lastReportCache.Delete(key) })
			body, err := json.Marshal(message)
			if err != nil {
				t.Fatal(err)
			}
			input := make(chan amqp.Delivery, 3)
			input <- amqp.Delivery{Body: []byte(`{`)} // malformed messages must not reach database callbacks
			input <- amqp.Delivery{Body: body}
			input <- amqp.Delivery{Body: body}
			close(input)
			output := make(chan aprs.Frame, 3)
			lookups := 0
			processMessages(input, output, func(network, app, device string) bool {
				if network != message.NetworkId || app != message.AppID || device != message.DevID {
					t.Fatal("incorrect device identity")
				}
				return tc.registered
			}, func(string, string, string) (sqldb.AprsDevice, error) {
				lookups++
				if tc.lookupError {
					return sqldb.AprsDevice{}, errors.New("lookup failed")
				}
				return sqldb.AprsDevice{Callsign: "N0CALL", Ssid: 7, SymbolTable: "/", Symbol: ">"}, nil
			})
			if len(output) != tc.wantFrames {
				t.Fatalf("frames=%d, want %d", len(output), tc.wantFrames)
			}
			if !tc.registered && lookups != 0 {
				t.Fatal("unregistered device looked up")
			}
			_, cached := lastReportCache.Load(key)
			if cached != (tc.wantFrames > 0) {
				t.Fatalf("cached=%v", cached)
			}
			if tc.wantFrames > 0 {
				f := <-output
				if f.Src.Call != "N0CALL" || f.Src.SSID != 7 || f.Dst.Call != "APRS" || len(f.Path) != 1 || f.Path[0].Call != "TCPIP" || !f.Path[0].Repeated {
					t.Fatalf("frame = %+v", f)
				}
				want := aprs.Position{Latitude: -33.9, Longitude: 18.8, Altitude: 10, SymbolTable: "/", Symbol: ">"}
				if f.Text != want.String() {
					t.Fatalf("position=%q, want %q", f.Text, want.String())
				}
				if lookups != 1 {
					t.Fatalf("duplicate packet triggered %d lookups", lookups)
				}
			}
		})
	}
}
