package radar_beam

import (
	"testing"
	"time"
	"ttnmapper-services/pkg/database"
)

func TestIncrementRadarBeam(t *testing.T) {
	stamp := time.Unix(1600000000, 0)
	var beam database.RadarBeam
	for _, tc := range []struct{ distance, max, second float64 }{{10, 10, 0}, {5, 10, 5}, {20, 20, 10}, {1, 20, 10}, {20, 20, 20}} {
		incrementRadarBeam(&beam, stamp, tc.distance)
		if beam.DistanceMax != tc.max || beam.Distance2nd != tc.second {
			t.Fatalf("beam=%+v after distance %f", beam, tc.distance)
		}
	}
	incrementRadarBeam(&beam, stamp.Add(-time.Hour), 0)
	if beam.Samples != 6 || !beam.LastUpdated.Equal(stamp) {
		t.Fatalf("beam=%+v", beam)
	}
}
