package tms

import (
	"github.com/patrickmn/go-cache"
	"strconv"
	"testing"
	"time"
)

func TestGetAntennaOnlineFromCache(t *testing.T) {
	oldCache, oldDB := AntennaLastHeardCache, DB
	AntennaLastHeardCache = cache.New(cache.NoExpiration, 0)
	DB = nil
	t.Cleanup(func() { AntennaLastHeardCache = oldCache; DB = oldDB })
	now := time.Now()
	for i, tc := range []struct {
		name   string
		heard  time.Time
		online bool
	}{
		{"recent", now.Add(-time.Hour), true}, {"old", now.Add(-6 * 24 * time.Hour), false}, {"never", time.Time{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uint(i + 1)
			AntennaLastHeardCache.Set(strconv.Itoa(int(id)), tc.heard, cache.NoExpiration)
			if got := GetAntennaOnline(id); got != tc.online {
				t.Fatalf("online=%v, want %v", got, tc.online)
			}
		})
	}
}
