package thethingsstack

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
	"ttnmapper-services/internal/testutil"
)

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type failingBody struct{ closed bool }

func (b *failingBody) Read([]byte) (int, error) { return 0, errors.New("body read failed") }
func (b *failingBody) Close() error             { b.closed = true; return nil }

func TestFetchGatewaysRequest(t *testing.T) {
	var body *trackedBody
	testutil.HTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "tenant.eu1.cloud.thethings.industries" || req.URL.Path != "/api/v3/gateways" {
			t.Errorf("request=%s %s", req.Method, req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("User-Agent") != "ttnmapper-update-gateway" {
			t.Errorf("headers=%v", req.Header)
		}
		if req.URL.Query().Get("field_mask") == "" {
			t.Error("missing field mask")
		}
		res := testutil.JSON(`{"gateways":[{"ids":{"gateway_id":"gateway","eui":"001122334455AABB"},"gateway_server_address":"cluster.example.invalid","name":"test"}]}`)
		body = &trackedBody{Reader: res.Body}
		res.Body = body
		return res, nil
	})
	got, err := FetchGateways("tenant", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Ids.GatewayId != "gateway" || got[0].GatewayServerAddress != "cluster.example.invalid" || got[0].Name != "test" {
		t.Fatalf("gateways=%+v", got)
	}
	if body == nil || !body.closed {
		t.Fatal("response body not closed")
	}
}

func TestFetchBatchRequest(t *testing.T) {
	gateways := []Gateway{
		{Ids: GatewayIds{GatewayId: "one"}, GatewayServerAddress: "cluster.example.invalid"},
		{Ids: GatewayIds{GatewayId: "two"}, GatewayServerAddress: "cluster.example.invalid"},
	}
	testutil.HTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Host != "cluster.example.invalid" || req.URL.Path != "/api/v3/gs/gateways/connection/stats" {
			t.Errorf("request=%s %s", req.Method, req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers=%v", req.Header)
		}
		var payload GatewayStatsBatchRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(payload.GatewayIds, []GatewayIds{gateways[0].Ids, gateways[1].Ids}) {
			t.Errorf("body=%+v", payload)
		}
		return testutil.JSON(`{"entries":{"one":{"last_uplink_received_at":"2020-01-02T03:04:05Z"},"two":{}}}`), nil
	})
	got, err := FetchStatusesBatch(gateways, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 2 || got.Entries["one"].GetLastUplinkReceivedAt().GetSeconds() != 1577934245 {
		t.Fatalf("stats=%+v", got.Entries)
	}
}

func TestFetchSingleStatusRequest(t *testing.T) {
	testutil.HTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "cluster.example.invalid" || req.URL.Path != "/api/v3/gs/gateways/gateway/connection/stats" || req.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request=%s %s", req.Method, req.URL)
		}
		return testutil.JSON(`{"last_uplink_received_at":"2020-01-02T03:04:05Z","protocol":"udp"}`), nil
	})
	got, err := FetchStatus(Gateway{Ids: GatewayIds{GatewayId: "gateway"}, GatewayServerAddress: "cluster.example.invalid"}, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastUplinkReceivedAt.Equal(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)) || got.Protocol != "udp" {
		t.Fatalf("status=%+v", got)
	}
}

func TestFetchFailures(t *testing.T) {
	gateway := Gateway{Ids: GatewayIds{GatewayId: "gateway"}, GatewayServerAddress: "cluster.example.invalid"}
	fetchers := []struct {
		name string
		call func() error
	}{
		{"gateways", func() error { _, err := FetchGateways("tenant", "test-key"); return err }},
		{"batch", func() error { _, err := FetchStatusesBatch([]Gateway{gateway}, "test-key"); return err }},
		{"single", func() error { _, err := FetchStatus(gateway, "test-key"); return err }},
	}
	for _, fetch := range fetchers {
		t.Run(fetch.name, func(t *testing.T) {
			for _, tc := range []struct {
				name                      string
				status                    int
				body                      string
				transportError, readError bool
			}{
				{name: "unauthorized", status: 401, body: `{}`}, {name: "server error", status: 503, body: `{}`},
				{name: "malformed JSON", status: 200, body: `{`}, {name: "transport error", transportError: true}, {name: "read failure", status: 200, readError: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var body *trackedBody
					var broken *failingBody
					testutil.HTTP(t, func(*http.Request) (*http.Response, error) {
						if tc.transportError {
							return nil, errors.New("offline")
						}
						res := testutil.JSON(tc.body)
						res.StatusCode = tc.status
						if tc.readError {
							broken = &failingBody{}
							res.Body = broken
						} else {
							body = &trackedBody{Reader: res.Body}
							res.Body = body
						}
						return res, nil
					})
					if err := fetch.call(); err == nil {
						t.Fatal("failure returned no error")
					}
					if body != nil && !body.closed {
						t.Error("response body not closed")
					}
					if broken != nil && !broken.closed {
						t.Error("failed response body not closed")
					}
				})
			}
		})
	}
}

func TestFetchEmptyBatch(t *testing.T) {
	testutil.HTTP(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("empty batch sent an HTTP request")
		return nil, nil
	})
	got, err := FetchStatusesBatch(nil, "test-key")
	if err == nil || len(got.Entries) != 0 {
		t.Fatalf("result=%+v error=%v", got, err)
	}
}
