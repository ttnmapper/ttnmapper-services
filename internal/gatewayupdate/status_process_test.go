package gatewayupdate

import (
	"testing"
	"ttnmapper-services/pkg/types"
)

func TestCoordinatesValid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lon float64
		want     bool
	}{
		{"valid", -33.9, 18.8, true}, {"null island", 0, 0, false}, {"latitude", 91, 20, false}, {"longitude", 20, -181, false},
		{"SCG default", 52, 6, false}, {"Lorier default", 10, 20, false}, {"hack", 50.008724, 36.215805, false}, {"factory", 22.7, 114.24, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			valid, reason := CoordinatesValid(types.TtnMapperGateway{Latitude: tc.lat, Longitude: tc.lon})
			if valid != tc.want || (!valid && reason == "") {
				t.Fatalf("valid=%v reason=%q", valid, reason)
			}
		})
	}
}
