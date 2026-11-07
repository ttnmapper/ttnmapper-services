package utils

import (
	"math"
	"testing"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func TestBearing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lon float64
		want     uint
	}{
		{"north", 1, 0, 0}, {"east", 0, 1, 90}, {"south", -1, 0, 180}, {"west", 0, -1, 270},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetBearingLive(types.TtnMapperGateway{}, types.TtnMapperUplinkMessage{Latitude: tc.lat, Longitude: tc.lon}); got != tc.want {
				t.Errorf("live=%d, want %d", got, tc.want)
			}
			if got := GetBearingDatabase(database.Gateway{}, database.Packet{Latitude: tc.lat, Longitude: tc.lon}); got != tc.want {
				t.Errorf("database=%d, want %d", got, tc.want)
			}
		})
	}
}
func TestDistance(t *testing.T) {
	if got := GetDistanceDatabase(database.Gateway{}, database.Packet{Longitude: 1}); math.Abs(got-111.195) > 0.01 {
		t.Errorf("one equatorial degree = %f km", got)
	}
	for _, tc := range []struct {
		km   float64
		want bool
	}{{0, false}, {0.1, true}, {200, true}, {200.1, false}} {
		if got := CheckDistance(tc.km); got != tc.want {
			t.Errorf("CheckDistance(%f)=%v", tc.km, got)
		}
	}
}
