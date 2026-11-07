package radar

import (
	"testing"
	"ttnmapper-services/pkg/database"
)

func TestAddBeamsToGatewayMulti(t *testing.T) {
	beams := map[int]map[uint]database.RadarBeam{}
	AddBeamsToGatewayMulti(beams, []database.RadarBeam{
		{Level: -100, Bearing: 90, DistanceMax: 10, Distance2nd: 5},
		{Level: -100, Bearing: 90, DistanceMax: 20, Distance2nd: 15},
		{Level: -100, Bearing: 180, DistanceMax: 7, Distance2nd: 3},
		{Level: -110, Bearing: 90, DistanceMax: 30, Distance2nd: 25},
	})
	got := beams[-100][90]
	if got.DistanceMax != 20 || got.Distance2nd != 15 {
		t.Fatalf("merged=%+v", got)
	}
	if beams[-100][180].DistanceMax != 7 || beams[-110][90].DistanceMax != 30 {
		t.Fatal("bearings or levels were combined")
	}
}
