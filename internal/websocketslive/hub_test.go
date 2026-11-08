package websocketslive

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
	"ttnmapper-services/pkg/types"
)

func TestClientFilters(t *testing.T) {
	msg := types.TtnMapperUplinkMessage{AppID: "app", DevID: "device", UserId: "user", Experiment: "experiment"}
	for _, tc := range []struct {
		name   string
		client Client
		want   bool
	}{
		{"no filters", Client{}, false},
		{"application", Client{applicationId: "app"}, true}, {"wrong application", Client{applicationId: "other"}, false},
		{"device", Client{deviceId: "device"}, true}, {"wrong device", Client{deviceId: "other"}, false},
		{"user", Client{userId: "user"}, true}, {"wrong user", Client{userId: "other"}, false},
		{"experiment", Client{experiment: "experiment"}, true}, {"wrong experiment", Client{experiment: "other"}, false},
		{"all match", Client{applicationId: "app", deviceId: "device", userId: "user", experiment: "experiment"}, true},
		{"all but application", Client{applicationId: "other", deviceId: "device", userId: "user", experiment: "experiment"}, false},
		{"all but device", Client{applicationId: "app", deviceId: "other", userId: "user", experiment: "experiment"}, false},
		{"all but user", Client{applicationId: "app", deviceId: "device", userId: "other", experiment: "experiment"}, false},
		{"all but experiment", Client{applicationId: "app", deviceId: "device", userId: "user", experiment: "other"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.client.matches(msg); got != tc.want {
				t.Fatalf("matches=%v want=%v", got, tc.want)
			}
		})
	}
}
func TestDelivery(t *testing.T) {
	h := newHub()
	matching := &Client{applicationId: "app", send: make(chan []byte, 2)}
	experiment := &Client{experiment: "experiment", send: make(chan []byte, 2)}
	other := &Client{applicationId: "other", send: make(chan []byte, 2)}
	unfiltered := &Client{send: make(chan []byte, 2)}
	slow := &Client{applicationId: "app", send: make(chan []byte, 1)}
	slow.send <- []byte("queued")
	for _, c := range []*Client{matching, experiment, other, unfiltered, slow} {
		h.clients[c] = true
	}
	msg := types.TtnMapperUplinkMessage{AppID: "app", DevID: "device", Experiment: "experiment", Latitude: -33.9, Longitude: 18.8}
	h.deliver(msg)
	for _, c := range []*Client{matching, experiment} {
		select {
		case data := <-c.send:
			var got types.TtnMapperUplinkMessage
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, msg) {
				t.Fatalf("packet=%+v", got)
			}
		default:
			t.Fatal("matching client received nothing")
		}
	}
	if len(other.send) != 0 || len(unfiltered.send) != 0 {
		t.Fatal("packet leaked to unrelated client")
	}
	if h.clients[slow] {
		t.Fatal("slow client remains registered")
	}
	if string(<-slow.send) != "queued" {
		t.Fatal("queued packet changed")
	}
	if _, ok := <-slow.send; ok {
		t.Fatal("slow channel not closed")
	}
	// A subsequent disconnect must not close the channel twice.
	h.removeClient(slow)
	h.removeClient(matching)
	h.removeClient(matching)
	if _, ok := <-matching.send; ok {
		t.Fatal("unregistered channel not closed")
	}
	h.deliver(msg)
	if len(experiment.send) != 1 {
		t.Fatal("remaining client stopped receiving")
	}
}
func TestInvalidPacketDoesNotDisconnectClients(t *testing.T) {
	h := newHub()
	c := &Client{applicationId: "app", send: make(chan []byte, 1)}
	h.clients[c] = true
	h.deliver(types.TtnMapperUplinkMessage{AppID: "app", Latitude: math.NaN()})
	if !h.clients[c] || len(c.send) != 0 {
		t.Fatal("invalid packet changed clients")
	}
	h.deliver(types.TtnMapperUplinkMessage{AppID: "app"})
	if len(c.send) != 1 {
		t.Fatal("valid packet after invalid packet was lost")
	}
}
func TestHubLifecycle(t *testing.T) {
	h := newHub()
	messages := make(chan types.TtnMapperUplinkMessage)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { defer close(done); h.runMessages(messages, stop) }()
	t.Cleanup(func() {
		close(stop)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("hub did not stop")
		}
	})
	client := &Client{applicationId: "app", send: make(chan []byte, 1)}
	select {
	case h.register <- client:
	case <-time.After(time.Second):
		t.Fatal("registration blocked")
	}
	select {
	case messages <- types.TtnMapperUplinkMessage{AppID: "app"}:
	case <-time.After(time.Second):
		t.Fatal("delivery blocked")
	}
	select {
	case <-client.send:
	case <-time.After(time.Second):
		t.Fatal("no broadcast")
	}
	select {
	case h.unregister <- client:
	case <-time.After(time.Second):
		t.Fatal("unregister blocked")
	}
	select {
	case _, ok := <-client.send:
		if ok {
			t.Fatal("channel not closed")
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect blocked")
	}
	select {
	case h.unregister <- client:
	case <-time.After(time.Second):
		t.Fatal("repeat unregister blocked")
	}
}
func TestHubClosesClientsWhenInputEnds(t *testing.T) {
	h := newHub()
	messages := make(chan types.TtnMapperUplinkMessage)
	done := make(chan struct{})
	client := &Client{applicationId: "app", send: make(chan []byte, 1)}
	h.clients[client] = true
	go func() { defer close(done); h.runMessages(messages, nil) }()
	close(messages)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("hub did not exit")
	}
	if len(h.clients) != 0 {
		t.Fatal("clients retained after shutdown")
	}
	if _, ok := <-client.send; ok {
		t.Fatal("client channel retained after shutdown")
	}
}
