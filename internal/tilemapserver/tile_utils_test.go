package tms

import (
	"reflect"
	"testing"
)

func TestGetZ19TileRangeBuffer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		x, y, z int
		buffer  float64
		want    [4]int
	}{
		{"same zoom", 100, 200, 19, 0, [4]int{100, 200, 101, 201}},
		{"two zooms", 100, 100, 17, 0, [4]int{400, 400, 404, 404}},
		{"half tile buffer", 100, 100, 17, 0.5, [4]int{398, 398, 406, 406}},
		{"round outwards", 100, 100, 19, 0.1, [4]int{99, 99, 102, 102}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b, c, d := GetZ19TileRangeBuffer(tc.x, tc.y, tc.z, tc.buffer)
			got := [4]int{a, b, c, d}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("bounds=%v, want %v", got, tc.want)
			}
		})
	}
}
