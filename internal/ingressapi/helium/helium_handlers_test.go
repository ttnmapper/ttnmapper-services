package helium

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ttnmapper-services/internal/ingressapi/types"
)

// All requests use synthetic payloads, an in-memory queue and a local validator.
func TestUplink(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		reject     bool
		status     int
		message    string
	}{
		{"accepted", `{"app_eui":"app","name":"device","port":1,"fcnt":42,"decoded":{"payload":{"latitude":-33.9,"longitude":18.8,"altitude":10}}}`, false, http.StatusAccepted, "New packet accepted into queue"},
		{"invalid json", `{`, false, http.StatusBadRequest, "Can not parse json body"},
		{"rejected user", `{`, true, http.StatusForbidden, "invalid test user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := make(chan types.TtnMapperUplinkMessage, 2)
			calls := 0
			ctx := &Context{PublishChannel: queue, ValidateEmail: func(email string) error {
				calls++
				if email != "mapper@example.invalid" {
					t.Errorf("email = %q", email)
				}
				if tc.reject {
					return errors.New("invalid test user")
				}
				return nil
			}}
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("TTNMAPPERORG-USER", "mapper@example.invalid")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-TTS-DOMAIN", "eu1.cloud.thethings.network")
			req.Header.Set("User-Agent", "unit-test")
			rr := httptest.NewRecorder()
			ctx.PostHelium(rr, req)
			if calls != 1 {
				t.Fatalf("validation calls = %d", calls)
			}
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", rr.Code, tc.status, rr.Body.String())
			}
			var response struct {
				Success bool
				Message string
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			accepted := tc.status == http.StatusAccepted
			if response.Success != accepted || response.Message != tc.message {
				t.Fatalf("response = %+v", response)
			}
			if !accepted {
				if len(queue) != 0 {
					t.Fatal("rejected request published a packet")
				}
				return
			}
			if len(queue) != 1 {
				t.Fatalf("published %d packets, want one", len(queue))
			}
			packet := <-queue
			if packet.AppID != "app" || packet.DevID != "device" || packet.FPort != 1 || packet.FCnt != 42 {
				t.Errorf("identity = %+v", packet)
			}
			if packet.Latitude != -33.9 || packet.Longitude != 18.8 || packet.Altitude != 10 {
				t.Errorf("location = %+v", packet)
			}
			if packet.UserId != "mapper@example.invalid" || packet.UserAgent != "unit-test" {
				t.Errorf("source = %+v", packet)
			}
		})
	}
}

func TestUplinkDefaultValidatorRejectsMissingUser(t *testing.T) {
	queue := make(chan types.TtnMapperUplinkMessage, 1)
	ctx := &Context{PublishChannel: queue}
	rr := httptest.NewRecorder()
	ctx.PostHelium(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)))
	if rr.Code != http.StatusForbidden || len(queue) != 0 {
		t.Fatalf("status=%d queued=%d", rr.Code, len(queue))
	}
}
