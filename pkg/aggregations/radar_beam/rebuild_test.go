package radar_beam

import (
	"errors"
	"reflect"
	"testing"
	"ttnmapper-services/pkg/database"
)

func snapshotCache() map[database.RadarBeamIndexer]database.RadarBeam {
	result := map[database.RadarBeamIndexer]database.RadarBeam{}
	radarBeamDbCache.Range(func(key, value interface{}) bool {
		result[key.(database.RadarBeamIndexer)] = value.(database.RadarBeam)
		return true
	})
	return result
}
func TestRebuildPublishesCacheOnlyAfterCommit(t *testing.T) {
	resetLiveCache(t)
	old := database.RadarBeam{AntennaID: 7, ID: 10, Level: -200, Bearing: 359, Samples: 99}
	other := old
	other.AntennaID = 8
	storeRadarBeamInCache(old)
	storeRadarBeamInCache(other)
	before := snapshotCache()
	entries := []database.RadarBeam{{AntennaID: 7, Level: -105, Bearing: 0, Samples: 2}}
	failure := errors.New("commit failed")
	err := replaceRebuilt(7, entries, func(uint, []database.RadarBeam) error {
		if !reflect.DeepEqual(snapshotCache(), before) {
			t.Fatal("cache changed before commit")
		}
		return failure
	})
	if !errors.Is(err, failure) || !reflect.DeepEqual(snapshotCache(), before) {
		t.Fatal("failed commit changed cache")
	}
	err = replaceRebuilt(7, entries, func(id uint, rows []database.RadarBeam) error {
		if id != 7 || len(rows) != 1 || !reflect.DeepEqual(snapshotCache(), before) {
			t.Fatal("replacement inputs/cache changed")
		}
		rows[0].ID = 20 // The database assigns IDs before committing.
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	after := snapshotCache()
	if len(after) != 2 {
		t.Fatalf("cache=%+v", after)
	}
	target, others := 0, 0
	for _, entry := range after {
		if entry.AntennaID == 7 {
			target++
			if entry.ID != 20 {
				t.Fatal("cache retained stale row")
			}
		}
		if entry.AntennaID == 8 {
			others++
			if !reflect.DeepEqual(entry, other) {
				t.Fatal("other antenna changed")
			}
		}
	}
	if target != 1 || others != 1 {
		t.Fatalf("cache=%+v", after)
	}
	if err := replaceRebuilt(7, nil, func(uint, []database.RadarBeam) error { return nil }); err != nil {
		t.Fatal(err)
	}
	after = snapshotCache()
	if len(after) != 1 {
		t.Fatalf("empty rebuild cache=%+v", after)
	}
	for _, entry := range after {
		if entry.AntennaID != 8 {
			t.Fatal("wrong antenna retained")
		}
	}
}
