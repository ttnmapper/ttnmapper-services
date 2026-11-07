package grid_cell

import (
	"errors"
	"sync"
	"testing"
	"time"

	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func resetLiveCache(t *testing.T) {
	gridCellDbCache = sync.Map{}
	t.Cleanup(func() { gridCellDbCache = sync.Map{} })
}
func liveMessage() types.TtnMapperUplinkMessage {
	return types.TtnMapperUplinkMessage{Time: time.Second.Nanoseconds(), Latitude: 2, Longitude: 1, Gateways: []types.TtnMapperGateway{{NetworkId: "network", GatewayId: "gateway", Rssi: -99, Snr: -2}}}
}
func TestLiveAggregationRepeatedPackets(t *testing.T) {
	resetLiveCache(t)
	var saved []database.GridCell
	store := liveStore{
		gateway: func(index database.GatewayIndexer) (database.Gateway, error) {
			if index.NetworkId != "network" || index.GatewayId != "gateway" {
				t.Errorf("gateway index=%+v", index)
			}
			return database.Gateway{Latitude: 1, Longitude: 1}, nil
		},
		antenna: func(network, gateway string, index uint8) database.Antenna {
			if network != "network" || gateway != "gateway" || index != 0 {
				t.Error("wrong antenna identity")
			}
			return database.Antenna{ID: 7}
		},
		get:  func(id uint, lat, lon float64) (database.GridCell, error) { return getGridCellNotDb(id, lat, lon) },
		save: func(entry database.GridCell) error { saved = append(saved, entry); return nil },
	}
	message := liveMessage()
	aggregateNewData(message, store)
	message.Time = 0
	aggregateNewData(message, store)
	if len(saved) != 2 {
		t.Fatalf("saves=%d", len(saved))
	}
	got := saved[1]
	if got.AntennaID != 7 || !got.LastUpdated.Equal(time.Unix(0, time.Second.Nanoseconds())) {
		t.Fatalf("metadata=%+v", got)
	}
	if got.Bucket105 != 2 || got.BucketHigh != 0 {
		t.Fatalf("signal buckets=%+v", got)
	}
}
func TestLiveAggregationSkipsInvalidInputsAndFailures(t *testing.T) {
	for _, stage := range []string{"experiment", "null island", "unknown gateway", "gateway lookup", "too far", "same location", "missing antenna", "entry lookup", "save"} {
		t.Run(stage, func(t *testing.T) {
			resetLiveCache(t)
			message := liveMessage()
			failure := errors.New("storage failure")
			gets, saves := 0, 0
			if stage == "experiment" {
				message.Experiment = "test"
			}
			if stage == "null island" {
				message.Latitude = 0
				message.Longitude = 0
			}
			store := liveStore{
				gateway: func(database.GatewayIndexer) (database.Gateway, error) {
					if stage == "experiment" || stage == "null island" {
						t.Fatal("invalid message performed a lookup")
					}
					if stage == "gateway lookup" {
						return database.Gateway{}, failure
					}
					if stage == "unknown gateway" {
						return database.Gateway{}, nil
					}
					lat := 1.0
					if stage == "too far" {
						lat = 10
					}
					if stage == "same location" {
						lat = 2
					}
					return database.Gateway{Latitude: lat, Longitude: 1}, nil
				},
				antenna: func(string, string, uint8) database.Antenna {
					if stage == "missing antenna" {
						return database.Antenna{}
					}
					return database.Antenna{ID: 7}
				},
				get: func(id uint, lat, lon float64) (database.GridCell, error) {
					gets++
					if stage == "entry lookup" {
						return database.GridCell{}, failure
					}
					return getGridCellNotDb(id, lat, lon)
				},
				save: func(database.GridCell) error {
					saves++
					if stage == "save" {
						return failure
					}
					return nil
				},
			}
			aggregateNewData(message, store)
			if stage == "save" {
				if saves != 1 {
					t.Fatalf("saves=%d", saves)
				}
			} else if saves != 0 {
				t.Fatalf("unexpected saves=%d gets=%d", saves, gets)
			}
			items := 0
			gridCellDbCache.Range(func(_, _ interface{}) bool { items++; return true })
			if items != 0 {
				t.Fatal("failed update entered cache")
			}
		})
	}
}
func TestFailedSavePreservesCachedAggregate(t *testing.T) {
	resetLiveCache(t)
	fail := false
	var got database.GridCell
	store := liveStore{
		gateway: func(database.GatewayIndexer) (database.Gateway, error) {
			return database.Gateway{Latitude: 1, Longitude: 1}, nil
		},
		antenna: func(string, string, uint8) database.Antenna { return database.Antenna{ID: 7} },
		get:     func(id uint, lat, lon float64) (database.GridCell, error) { return getGridCellNotDb(id, lat, lon) },
		save: func(entry database.GridCell) error {
			if fail {
				return errors.New("write failed")
			}
			got = entry
			return nil
		},
	}
	message := liveMessage()
	aggregateNewData(message, store)
	fail = true
	aggregateNewData(message, store)
	fail = false
	aggregateNewData(message, store)
	if got.Bucket105 != 2 {
		t.Fatalf("failed save changed cached count: %+v", got)
	}
}
