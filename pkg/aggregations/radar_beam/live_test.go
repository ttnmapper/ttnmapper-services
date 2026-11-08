package radar_beam

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func resetLiveCache(t *testing.T) {
	radarBeamDbCache = sync.Map{}
	t.Cleanup(func() { radarBeamDbCache = sync.Map{} })
}
func liveMessage() types.TtnMapperUplinkMessage {
	return types.TtnMapperUplinkMessage{Time: time.Second.Nanoseconds(), Latitude: 2, Longitude: 1, Gateways: []types.TtnMapperGateway{{NetworkId: "network", GatewayId: "gateway", Rssi: -99, Snr: -2}}}
}
func TestLiveAggregationRepeatedPackets(t *testing.T) {
	resetLiveCache(t)
	var saved []database.RadarBeam
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
		get: func(id uint, level int, bearing uint) (database.RadarBeam, error) {
			return getRadarBeamNotDb(id, level, bearing)
		},
		save: func(entry database.RadarBeam) error { saved = append(saved, entry); return nil },
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
	if got.Level != -105 || got.Bearing != 0 || got.Samples != 2 || math.Abs(got.DistanceMax-111195) > 10 || got.DistanceMax != got.Distance2nd {
		t.Fatalf("radar=%+v", got)
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
				get: func(id uint, level int, bearing uint) (database.RadarBeam, error) {
					gets++
					if stage == "entry lookup" {
						return database.RadarBeam{}, failure
					}
					return getRadarBeamNotDb(id, level, bearing)
				},
				save: func(database.RadarBeam) error {
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
			radarBeamDbCache.Range(func(_, _ interface{}) bool { items++; return true })
			if items != 0 {
				t.Fatal("failed update entered cache")
			}
		})
	}
}
func TestFailedSavePreservesCachedAggregate(t *testing.T) {
	resetLiveCache(t)
	fail := false
	var got database.RadarBeam
	store := liveStore{
		gateway: func(database.GatewayIndexer) (database.Gateway, error) {
			return database.Gateway{Latitude: 1, Longitude: 1}, nil
		},
		antenna: func(string, string, uint8) database.Antenna { return database.Antenna{ID: 7} },
		get: func(id uint, level int, bearing uint) (database.RadarBeam, error) {
			return getRadarBeamNotDb(id, level, bearing)
		},
		save: func(entry database.RadarBeam) error {
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
	if got.Samples != 2 {
		t.Fatalf("failed save changed cached count: %+v", got)
	}
}

// In-memory store used only by the isolated live-aggregation tests.
func getRadarBeamNotDb(antennaId uint, level int, bearing uint) (database.RadarBeam, error) {
	radarBeam := database.RadarBeam{}

	// Try and find in cache
	radarBeamIndexer := database.RadarBeamIndexer{
		AntennaID: antennaId,
		Level:     level,
		Bearing:   bearing,
	}
	i, ok := radarBeamDbCache.Load(radarBeamIndexer)
	if ok {
		radarBeam = i.(database.RadarBeam)
		//log.Print("Found grid cell in cache")
	} else {
		radarBeam.AntennaID = antennaId
		radarBeam.Level = level
		radarBeam.Bearing = bearing
	}
	return radarBeam, nil
}
