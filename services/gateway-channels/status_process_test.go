package main

import (
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	"reflect"
	"sync"
	"testing"
	"ttnmapper-gateway-channels/types"
)

func TestProcessPackets(t *testing.T) {
	msg := types.TtnMapperUplinkMessage{Time: 123, Frequency: 868100000, Gateways: []types.TtnMapperGateway{{NetworkId: "network", GatewayId: "one", Time: 1, Latitude: 52, Longitude: 4, Altitude: 10}, {NetworkId: "network", GatewayId: "two"}}}
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	input := make(chan amqp.Delivery, 4)
	input <- amqp.Delivery{Body: []byte(`{`)}
	input <- amqp.Delivery{Body: []byte(`{"gateways":[]}`)}
	input <- amqp.Delivery{Body: body}
	input <- amqp.Delivery{Body: body}
	close(input)
	var got []types.TtnMapperGateway
	processPackets(input, func(message types.TtnMapperUplinkMessage, gw types.TtnMapperGateway) {
		if !reflect.DeepEqual(message, msg) {
			t.Errorf("message changed: %+v", message)
		}
		got = append(got, gw)
	})
	if len(got) != 4 {
		t.Fatalf("updates=%d", len(got))
	}
	for i, gw := range got {
		if gw.GatewayId != msg.Gateways[i%2].GatewayId || gw.Time != 123 || gw.Latitude != 0 || gw.Longitude != 0 || gw.Altitude != 0 {
			t.Errorf("gateway=%+v", gw)
		}
	}
}
func TestAssociateGateway(t *testing.T) {
	failure := errors.New("storage failed")
	for _, stage := range []string{"success", "antenna", "frequency", "associate"} {
		t.Run(stage, func(t *testing.T) {
			var calls []string
			msg := types.TtnMapperUplinkMessage{Frequency: 868100000}
			gw := types.TtnMapperGateway{GatewayId: "gw"}
			store := gatewayStore{
				antenna: func(g types.TtnMapperGateway) (types.Gateway, error) {
					calls = append(calls, "antenna")
					if !reflect.DeepEqual(g, gw) {
						t.Error("wrong gateway")
					}
					if stage == "antenna" {
						return types.Gateway{}, failure
					}
					return types.Gateway{ID: 7}, nil
				},
				frequency: func(hz uint64) (types.Frequency, error) {
					calls = append(calls, "frequency")
					if hz != msg.Frequency {
						t.Error("wrong frequency")
					}
					if stage == "frequency" {
						return types.Frequency{}, failure
					}
					return types.Frequency{ID: 9}, nil
				},
				associate: func(a, f uint) error {
					calls = append(calls, "associate")
					if a != 7 || f != 9 {
						t.Errorf("IDs=%d,%d", a, f)
					}
					if stage == "associate" {
						return failure
					}
					return nil
				},
			}
			err := associateGateway(msg, gw, store)
			if stage == "success" && err != nil {
				t.Fatal(err)
			}
			if stage != "success" && !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			want := []string{"antenna", "frequency", "associate"}
			if stage == "antenna" {
				want = want[:1]
			}
			if stage == "frequency" {
				want = want[:2]
			}
			if !reflect.DeepEqual(calls, want) {
				t.Errorf("calls=%v want=%v", calls, want)
			}
		})
	}
}
func resetCaches(t *testing.T) {
	antennaCache = sync.Map{}
	frequencyCache = sync.Map{}
	t.Cleanup(func() { antennaCache = sync.Map{}; frequencyCache = sync.Map{} })
}
func TestFrequencyCache(t *testing.T) {
	resetCaches(t)
	calls := 0
	lookup := func(hz uint64) (types.Frequency, error) {
		calls++
		return types.Frequency{ID: uint(calls), Herz: hz}, nil
	}
	a, err := cachedFrequency(868100000, lookup)
	if err != nil {
		t.Fatal(err)
	}
	b, err := cachedFrequency(868100000, lookup)
	if err != nil {
		t.Fatal(err)
	}
	c, err := cachedFrequency(868300000, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !reflect.DeepEqual(a, b) || c.Herz != 868300000 || c.ID == a.ID {
		t.Fatalf("calls=%d results=%+v %+v %+v", calls, a, b, c)
	}
	if _, ok := antennaCache.Load(uint64(868100000)); ok {
		t.Fatal("frequency stored in antenna cache")
	}
}
func TestAntennaCache(t *testing.T) {
	resetCaches(t)
	calls := 0
	lookup := func(gw types.TtnMapperGateway) (types.Gateway, error) {
		calls++
		return types.Gateway{ID: uint(calls)}, nil
	}
	for i, gw := range []types.TtnMapperGateway{{NetworkId: "one", GatewayId: "gw"}, {NetworkId: "two", GatewayId: "gw"}, {NetworkId: "one", GatewayId: "other"}, {NetworkId: "one", GatewayId: "gw", AntennaIndex: 1}} {
		a, err := cachedAntenna(gw, lookup)
		if err != nil {
			t.Fatal(err)
		}
		b, err := cachedAntenna(gw, lookup)
		if err != nil {
			t.Fatal(err)
		}
		if a.ID != uint(i+1) || !reflect.DeepEqual(a, b) {
			t.Fatalf("results=%+v %+v", a, b)
		}
	}
	if calls != 4 {
		t.Fatalf("lookups=%d", calls)
	}
}
func TestFailedLookupsAreRetried(t *testing.T) {
	resetCaches(t)
	failure := errors.New("lookup failed")
	calls := 0
	lookup := func(hz uint64) (types.Frequency, error) {
		calls++
		if calls == 1 {
			return types.Frequency{}, failure
		}
		return types.Frequency{ID: 1, Herz: hz}, nil
	}
	if _, err := cachedFrequency(868100000, lookup); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	if got, err := cachedFrequency(868100000, lookup); err != nil || got.ID != 1 || calls != 2 {
		t.Fatalf("retry=%+v error=%v calls=%d", got, err, calls)
	}
	calls = 0
	antenna := func(gw types.TtnMapperGateway) (types.Gateway, error) {
		calls++
		if calls == 1 {
			return types.Gateway{}, failure
		}
		return types.Gateway{ID: 2}, nil
	}
	gw := types.TtnMapperGateway{GatewayId: "gw"}
	if _, err := cachedAntenna(gw, antenna); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	if got, err := cachedAntenna(gw, antenna); err != nil || got.ID != 2 || calls != 2 {
		t.Fatalf("retry=%+v error=%v calls=%d", got, err, calls)
	}
}

// Duplicate deliveries represent repeated receptions: each must increment the
// association count while reusing the cached gateway and frequency IDs.
func TestRepeatedPacketsReuseIDsAndCountReceptions(t *testing.T) {
	resetCaches(t)
	gatewayLookups, frequencyLookups := 0, 0
	counts := make(map[[2]uint]int)
	store := gatewayStore{
		antenna: func(gw types.TtnMapperGateway) (types.Gateway, error) {
			return cachedAntenna(gw, func(types.TtnMapperGateway) (types.Gateway, error) {
				gatewayLookups++
				return types.Gateway{ID: uint(gatewayLookups)}, nil
			})
		},
		frequency: func(hz uint64) (types.Frequency, error) {
			return cachedFrequency(hz, func(uint64) (types.Frequency, error) {
				frequencyLookups++
				return types.Frequency{ID: uint(frequencyLookups)}, nil
			})
		},
		associate: func(a, f uint) error { counts[[2]uint{a, f}]++; return nil },
	}
	input := make(chan amqp.Delivery, 3)
	input <- amqp.Delivery{Body: []byte(`{"frequency":868100000,"gateways":[{"network_id":"network","gtw_id":"one"},{"network_id":"network","gtw_id":"two"}]}`)}
	input <- amqp.Delivery{Body: []byte(`{"frequency":868100000,"gateways":[{"network_id":"network","gtw_id":"one"}]}`)}
	input <- amqp.Delivery{Body: []byte(`{"frequency":868300000,"gateways":[{"network_id":"network","gtw_id":"one"}]}`)}
	close(input)
	processPackets(input, func(msg types.TtnMapperUplinkMessage, gw types.TtnMapperGateway) {
		if err := associateGateway(msg, gw, store); err != nil {
			t.Fatal(err)
		}
	})
	want := map[[2]uint]int{{1, 1}: 2, {2, 1}: 1, {1, 2}: 1}
	if gatewayLookups != 2 || frequencyLookups != 2 || !reflect.DeepEqual(counts, want) {
		t.Fatalf("gateway lookups=%d frequency lookups=%d counts=%v", gatewayLookups, frequencyLookups, counts)
	}
}
