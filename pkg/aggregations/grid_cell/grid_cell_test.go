package grid_cell

import (
	"reflect"
	"testing"
	"time"
	"ttnmapper-services/pkg/database"
)

func TestIncrementBucket(t *testing.T) {
	stamp := time.Unix(1600000000, 0)
	for _, tc := range []struct {
		name      string
		rssi, snr float32
		want      database.GridCell
	}{
		{"strong positive SNR", -94, 10, database.GridCell{BucketHigh: 1}},
		{"boundary", -95, 0, database.GridCell{Bucket100: 1}},
		{"negative SNR", -94, -7, database.GridCell{Bucket105: 1}},
		{"weak", -145, 0, database.GridCell{BucketLow: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got database.GridCell
			incrementBucket(&got, stamp, tc.rssi, tc.snr)
			tc.want.LastUpdated = stamp
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("cell=%+v, want %+v", got, tc.want)
			}
			incrementBucket(&got, stamp.Add(-time.Hour), tc.rssi, tc.snr)
			if !got.LastUpdated.Equal(stamp) {
				t.Fatal("older packet moved last update backwards")
			}
		})
	}
}
