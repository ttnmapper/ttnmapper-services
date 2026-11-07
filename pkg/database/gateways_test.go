package database

import (
	"github.com/patrickmn/go-cache"
	"testing"
)

func TestInsertGatewayLocationsRejectsEmptyBatch(t *testing.T) {
	old := Db
	Db = nil
	t.Cleanup(func() { Db = old })
	if err := InsertGatewayLocationsBatch(nil); err == nil || err.Error() != "nothing to insert" {
		t.Fatalf("empty batch error = %v", err)
	}
}
func TestOnlineGatewaysCacheSeparatesNetworks(t *testing.T) {
	oldDb := Db
	Db = nil
	t.Cleanup(func() { Db = oldDb })
	oldCache := networkOnlineGatewaysCache
	networkOnlineGatewaysCache = cache.New(cache.NoExpiration, 0)
	t.Cleanup(func() { networkOnlineGatewaysCache = oldCache })
	networkOnlineGatewaysCache.Set("one", []GatewayWithBoundingBox{{Gateway: Gateway{GatewayId: "first"}}}, cache.NoExpiration)
	networkOnlineGatewaysCache.Set("two", []GatewayWithBoundingBox{{Gateway: Gateway{GatewayId: "second"}}}, cache.NoExpiration)
	for _, tc := range []struct{ network, id string }{{"one", "first"}, {"two", "second"}} {
		got := GetOnlineGatewaysForNetwork(tc.network)
		if len(got) != 1 || got[0].Gateway.GatewayId != tc.id {
			t.Fatalf("network %s = %+v", tc.network, got)
		}
	}
}
