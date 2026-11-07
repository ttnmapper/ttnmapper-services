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

func TestSignalLevelBoundaries(t *testing.T) {
	for _, tc := range []struct {
		rssi, snr float32
		want      int
	}{
		{-99, 0, -100}, {-100, 0, -105}, {-104, 0, -105}, {-105, 0, -110},
		{-110, 0, -115}, {-115, 0, -120}, {-120, 0, -200}, {-200, 0, -200},
		{-99, -2, -105}, {-99, 10, -100},
	} {
		got := getLevelFromPacket(database.Packet{Rssi: tc.rssi, Snr: tc.snr})
		if got != tc.want {
			t.Errorf("rssi=%v snr=%v level=%d want=%d", tc.rssi, tc.snr, got, tc.want)
		}
	}
}
