//go:build integration

// Package integration exercises the production database helpers against the
// disposable PostgreSQL instance owned by check-postgres-integration.sh.
package integration

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ttnmapper-services/pkg/aggregations/grid_cell"
	"ttnmapper-services/pkg/aggregations/radar_beam"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func TestMain(m *testing.M) {
	socket := os.Getenv("TTNMAPPER_INTEGRATION_SOCKET")
	marker, err := os.ReadFile(filepath.Join(socket, "marker"))
	if !strings.HasPrefix(socket, "/tmp/ttnmapper-pg-integration.") || filepath.Base(socket) != "socket" || err != nil || string(marker) != "disposable-ttnmapper-integration\n" {
		fmt.Fprintln(os.Stderr, "Run scripts/check-postgres-integration.sh; an existing server DSN is not accepted.")
		os.Exit(2)
	}
	ctx := database.Context{Host: socket, Port: "55439", User: "ttnmapper_test", Database: "ttnmapper_integration"}
	ctx.Init()
	// Migrate real models, including their foreign keys and numeric types.
	err = database.Db.AutoMigrate(&database.Device{}, &database.Frequency{}, &database.DataRate{}, &database.CodingRate{}, &database.AccuracySource{}, &database.Experiment{}, &database.User{}, &database.UserAgent{}, &database.Antenna{}, &database.FineTimestampKeyID{}, &database.Packet{}, &database.Gateway{}, &database.GatewayLocation{}, &database.GridCell{}, &database.RadarBeam{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	result := m.Run()
	conn, err := database.Db.DB()
	if err == nil {
		_ = conn.Close()
	}
	os.Exit(result)
}

func mustExec(t *testing.T, sql string, args ...interface{}) {
	t.Helper()
	if err := database.Db.Exec(sql, args...).Error; err != nil {
		t.Fatal(err)
	}
}
func sqlState(err error) string {
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		return state.SQLState()
	}
	return ""
}
func message(network string, at time.Time) (types.TtnMapperUplinkMessage, types.TtnMapperGateway) {
	return types.TtnMapperUplinkMessage{NetworkId: network, AppID: "app", DevID: "device", DevEui: "0011223344556677", Time: at.UnixNano(), FPort: 1, FCnt: 42, Frequency: 868100000, Modulation: "LORA", Bandwidth: 125000, SpreadingFactor: 7, CodingRate: "4/5", Latitude: 2.1234567, Longitude: 1.1234567, Altitude: 123.45, AccuracyMeters: 3.456, Satellites: 8, Hdop: 1.23, AccuracySource: "gps", UserId: "mapper", UserAgent: "integration-test"}, types.TtnMapperGateway{NetworkId: network, GatewayId: "gateway", Rssi: -101, Snr: 2, Time: at.Add(-time.Millisecond).UnixNano(), Timestamp: 123, ChannelIndex: 3, SignalRssi: -102}
}
func convert(t *testing.T, msg types.TtnMapperUplinkMessage, gw types.TtnMapperGateway) database.Packet {
	t.Helper()
	entry, err := database.UplinkMessageToPacket(msg, gw)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}
func insert(t *testing.T, msg types.TtnMapperUplinkMessage, gw types.TtnMapperGateway) database.Packet {
	t.Helper()
	entry := convert(t, msg, gw)
	if err := database.InsertPacket(&entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestPacketInsertionAndLookups(t *testing.T) {
	at := time.Date(2026, 11, 8, 12, 0, 0, 123456789, time.UTC)
	msg, gw := message(t.Name(), at)
	entry := insert(t, msg, gw)
	var got database.Packet
	if err := database.Db.First(&got, entry.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.ID == 0 || got.FPort != 1 || got.FCnt != 42 || got.Rssi != -101 || got.Snr != 2 || got.ChannelIndex != 3 {
		t.Fatalf("stored packet=%+v", got)
	}
	if !got.Time.Equal(at.Truncate(time.Microsecond)) || got.GatewayTime == nil || !got.GatewayTime.Equal(at.Add(-time.Millisecond).Truncate(time.Microsecond)) {
		t.Fatalf("stored timestamps=%v %v", got.Time, got.GatewayTime)
	}
	if got.Latitude != 2.123457 || got.Longitude != 1.123457 || got.Altitude != 123.5 || got.AccuracyMeters == nil || *got.AccuracyMeters != 3.46 || got.Hdop == nil || *got.Hdop != 1.2 || got.SignalRssi == nil || *got.SignalRssi != -102 || got.ExperimentID != nil {
		t.Fatalf("stored numeric/nullable fields=%+v", got)
	}
	var device database.Device
	var frequency database.Frequency
	var rate database.DataRate
	var antenna database.Antenna
	for _, lookup := range []struct {
		record interface{}
		id     uint
	}{{&device, got.DeviceID}, {&frequency, got.FrequencyID}, {&rate, got.DataRateID}, {&antenna, got.AntennaID}} {
		if err := database.Db.First(lookup.record, lookup.id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if device.NetworkId != msg.NetworkId || device.AppId != "app" || device.DevId != "device" || frequency.Herz != 868100000 || rate.Modulation != "LORA" || rate.Bandwidth != 125000 || rate.SpreadingFactor != 7 || antenna.GatewayId != "gateway" {
		t.Fatalf("lookup values=%+v %+v %+v %+v", device, frequency, rate, antenna)
	}
	again := insert(t, msg, gw)
	if again.ID == entry.ID || again.DeviceID != entry.DeviceID || again.FrequencyID != entry.FrequencyID || again.AntennaID != entry.AntennaID {
		t.Fatal("repeated reception did not reuse dimensions and create a distinct packet")
	}
	// Verify actual foreign-key enforcement rather than merely assuming AutoMigrate added it.
	orphan := entry
	orphan.ID = 0
	orphan.DeviceID = 999999999
	if err := database.InsertPacket(&orphan); sqlState(err) != "23503" {
		t.Fatalf("orphan insertion error=%v", err)
	}
}

func TestPartialPacketWriteFailure(t *testing.T) {
	msg, gw := message(t.Name(), time.Now().UTC().Truncate(time.Microsecond))
	first := insert(t, msg, gw)
	// Inject a failure only for the second reception's channel.
	mustExec(t, `CREATE FUNCTION reject_packet_channel() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.channel_index = 99 THEN RAISE EXCEPTION 'injected packet failure'; END IF; RETURN NEW; END $$`)
	mustExec(t, `CREATE TRIGGER reject_packet_channel BEFORE INSERT ON packets FOR EACH ROW EXECUTE FUNCTION reject_packet_channel()`)
	t.Cleanup(func() {
		mustExec(t, `DROP TRIGGER reject_packet_channel ON packets`)
		mustExec(t, `DROP FUNCTION reject_packet_channel()`)
	})
	gw.ChannelIndex = 99
	second := convert(t, msg, gw)
	if err := database.InsertPacket(&second); sqlState(err) != "P0001" {
		t.Fatalf("injected failure=%v", err)
	}
	var count int64
	if err := database.Db.Model(&database.Packet{}).Where("device_id = ?", first.DeviceID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected earlier write to remain and failing row absent; count=%d", count)
	}
	t.Log("Current behavior: an earlier packet row remains after a later insert fails; no delivery-wide transaction")
}

type replayFixture struct {
	antenna database.Antenna
	other   database.Antenna
	cutoff  time.Time
}

func seedReplay(t *testing.T, network string) replayFixture {
	t.Helper()
	cutoff := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	gateway := database.Gateway{NetworkId: network, GatewayId: "gateway", Latitude: 1, Longitude: 1}
	if err := database.Db.Create(&gateway).Error; err != nil {
		t.Fatal(err)
	}
	move := database.GatewayLocation{NetworkId: network, GatewayId: "gateway", InstalledAt: cutoff, Latitude: 1, Longitude: 1}
	if err := database.Db.Create(&move).Error; err != nil {
		t.Fatal(err)
	}
	var last database.Packet
	for _, tc := range []struct {
		offset     time.Duration
		experiment bool
		latitude   float64
	}{
		{-time.Second, false, 2}, {0, false, 2}, {time.Second, false, 2}, {2 * time.Second, false, 2}, {3 * time.Second, true, 2}, {4 * time.Second, false, 20},
	} {
		msg, gw := message(network, cutoff.Add(tc.offset))
		msg.Latitude = tc.latitude
		msg.Longitude = 1
		if tc.experiment {
			msg.Experiment = "excluded"
		}
		last = insert(t, msg, gw)
	}
	var antenna database.Antenna
	if err := database.Db.First(&antenna, last.AntennaID).Error; err != nil {
		t.Fatal(err)
	}
	msg, gw := message(network, cutoff.Add(time.Second))
	gw.AntennaIndex = 1
	otherPacket := insert(t, msg, gw)
	var other database.Antenna
	if err := database.Db.First(&other, otherPacket.AntennaID).Error; err != nil {
		t.Fatal(err)
	}
	// Old aggregates must be replaced, and another antenna must stay untouched.
	for _, a := range []database.Antenna{antenna, other} {
		if err := database.Db.Create(&database.GridCell{AntennaID: a.ID, X: 1, Y: 1, BucketHigh: 99}).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Db.Create(&database.RadarBeam{AntennaID: a.ID, Level: -200, Bearing: 359, Samples: 99}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return replayFixture{antenna, other, cutoff}
}
func assertOtherAntenna(t *testing.T, f replayFixture) {
	t.Helper()
	var cells []database.GridCell
	var beams []database.RadarBeam
	if err := database.Db.Where("antenna_id = ?", f.other.ID).Find(&cells).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Db.Where("antenna_id = ?", f.other.ID).Find(&beams).Error; err != nil {
		t.Fatal(err)
	}
	if len(cells) != 1 || cells[0].BucketHigh != 99 || len(beams) != 1 || beams[0].Samples != 99 {
		t.Fatal("rebuild modified another antenna")
	}
}
func TestRebuildAfterGatewayMove(t *testing.T) {
	f := seedReplay(t, t.Name())
	// This query excludes before-cutoff, exactly-at-cutoff and experiment packets.
	rows, err := database.GetPacketsForAntennaAfter(f.antenna, f.cutoff)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var times []time.Time
	for rows.Next() {
		var packet database.Packet
		if err := database.ScanRows(rows, &packet); err != nil {
			t.Fatal(err)
		}
		if packet.ExperimentID != nil || packet.AntennaID != f.antenna.ID {
			t.Fatalf("replay returned an unrelated or experiment packet: %+v", packet)
		}
		times = append(times, packet.Time)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(times) != 3 {
		t.Fatalf("replay query returned %d packets, want three (including out-of-range)", len(times))
	}
	for _, stamp := range times {
		if !stamp.After(f.cutoff) {
			t.Fatalf("cutoff was not exclusive: %v", stamp)
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := grid_cell.ReprocessAntenna(f.antenna, f.cutoff); err != nil {
			t.Fatal(err)
		}
		if err := radar_beam.ReprocessAntenna(f.antenna, f.cutoff); err != nil {
			t.Fatal(err)
		}
		cells := database.GetGridcellsForAntenna(f.antenna)
		beams := database.GetRadarBeamsForAntenna(f.antenna)
		if len(cells) != 1 || cells[0].Bucket105 != 2 || cells[0].BucketHigh != 0 || !cells[0].LastUpdated.Equal(f.cutoff.Add(2*time.Second)) {
			t.Fatalf("rebuilt cells=%+v", cells)
		}
		if len(beams) != 1 || beams[0].Level != -105 || beams[0].Bearing != 0 || beams[0].Samples != 2 || math.Abs(beams[0].DistanceMax-111195) > 10 || beams[0].Distance2nd != beams[0].DistanceMax {
			t.Fatalf("rebuilt beams=%+v", beams)
		}
		assertOtherAntenna(t, f)
	}
}

func TestRebuildFailuresPreserveOldAggregates(t *testing.T) {
	for _, kind := range []string{"grid", "radar"} {
		t.Run(kind, func(t *testing.T) {
			for _, stage := range []string{"read", "delete", "insert", "commit"} {
				t.Run(stage, func(t *testing.T) {
					f := seedReplay(t, t.Name())
					oldCells := database.GetGridcellsForAntenna(f.antenna)
					oldBeams := database.GetRadarBeamsForAntenna(f.antenna)
					rebuild := grid_cell.ReprocessAntenna
					table := "grid_cells"
					if kind == "radar" {
						rebuild = radar_beam.ReprocessAntenna
						table = "radar_beams"
					}
					var restore func()
					if stage == "read" {
						mustExec(t, `ALTER TABLE packets RENAME TO packets_unavailable`)
						restore = func() { mustExec(t, `ALTER TABLE packets_unavailable RENAME TO packets`) }
					} else {
						mustExec(t, `CREATE FUNCTION reject_rebuild() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected rebuild failure'; END $$`)
						trigger := fmt.Sprintf(`CREATE TRIGGER reject_rebuild BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION reject_rebuild()`, table)
						if stage == "delete" {
							trigger = fmt.Sprintf(`CREATE TRIGGER reject_rebuild BEFORE DELETE ON %s FOR EACH ROW EXECUTE FUNCTION reject_rebuild()`, table)
						}
						if stage == "commit" {
							trigger = fmt.Sprintf(`CREATE CONSTRAINT TRIGGER reject_rebuild AFTER INSERT ON %s DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_rebuild()`, table)
						}
						mustExec(t, trigger)
						restore = func() {
							mustExec(t, fmt.Sprintf(`DROP TRIGGER reject_rebuild ON %s`, table))
							mustExec(t, `DROP FUNCTION reject_rebuild()`)
						}
					}
					active := true
					t.Cleanup(func() {
						if active {
							restore()
						}
					})
					err := rebuild(f.antenna, f.cutoff)
					if err == nil {
						t.Fatal("failed rebuild returned no error")
					}
					if stage != "read" && sqlState(err) != "P0001" {
						t.Fatalf("injected error=%v", err)
					}
					if !reflect.DeepEqual(oldCells, database.GetGridcellsForAntenna(f.antenna)) || !reflect.DeepEqual(oldBeams, database.GetRadarBeamsForAntenna(f.antenna)) {
						t.Fatal("failed rebuild changed old aggregates")
					}
					assertOtherAntenna(t, f)
					restore()
					active = false
					// A retry in the same process must not reuse partial replay counts/IDs.
					if err := rebuild(f.antenna, f.cutoff); err != nil {
						t.Fatal(err)
					}
					if kind == "grid" {
						cells := database.GetGridcellsForAntenna(f.antenna)
						if len(cells) != 1 || cells[0].Bucket105 != 2 || cells[0].BucketHigh != 0 {
							t.Fatalf("retry cells=%+v", cells)
						}
					} else {
						beams := database.GetRadarBeamsForAntenna(f.antenna)
						if len(beams) != 1 || beams[0].Samples != 2 || beams[0].Bearing != 0 {
							t.Fatalf("retry beams=%+v", beams)
						}
					}
				})
			}
		})
	}
}

func TestEmptyRebuildClearsOnlyTargetAntenna(t *testing.T) {
	f := seedReplay(t, t.Name())
	cutoff := f.cutoff.Add(time.Hour)
	if err := grid_cell.ReprocessAntenna(f.antenna, cutoff); err != nil {
		t.Fatal(err)
	}
	if err := radar_beam.ReprocessAntenna(f.antenna, cutoff); err != nil {
		t.Fatal(err)
	}
	if len(database.GetGridcellsForAntenna(f.antenna)) != 0 || len(database.GetRadarBeamsForAntenna(f.antenna)) != 0 {
		t.Fatal("successful empty rebuild retained old rows")
	}
	assertOtherAntenna(t, f)
}

// The production batch size is 1000. A failure in a later batch must roll back
// the delete and every earlier batch, not merely the final INSERT statement.
func TestReplacementRollsBackEarlierBatches(t *testing.T) {
	for _, kind := range []string{"grid", "radar"} {
		t.Run(kind, func(t *testing.T) {
			f := seedReplay(t, t.Name())
			table, column := "grid_cells", "x"
			if kind == "radar" {
				table, column = "radar_beams", "bearing"
			}
			oldCells := database.GetGridcellsForAntenna(f.antenna)
			oldBeams := database.GetRadarBeamsForAntenna(f.antenna)
			mustExec(t, fmt.Sprintf(`CREATE FUNCTION reject_late_batch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.%s = 1001 THEN RAISE EXCEPTION 'injected late batch failure'; END IF; RETURN NEW; END $$`, column))
			mustExec(t, fmt.Sprintf(`CREATE TRIGGER reject_late_batch BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION reject_late_batch()`, table))
			t.Cleanup(func() {
				mustExec(t, fmt.Sprintf(`DROP TRIGGER reject_late_batch ON %s`, table))
				mustExec(t, `DROP FUNCTION reject_late_batch()`)
			})
			var err error
			if kind == "grid" {
				cells := make([]database.GridCell, 1002)
				for i := range cells {
					cells[i] = database.GridCell{AntennaID: f.antenna.ID, X: i, Y: 1, Bucket105: 1}
				}
				err = database.ReplaceGridCellsForAntenna(f.antenna.ID, cells)
			} else {
				beams := make([]database.RadarBeam, 1002)
				for i := range beams {
					beams[i] = database.RadarBeam{AntennaID: f.antenna.ID, Level: -105, Bearing: uint(i), Samples: 1}
				}
				err = database.ReplaceRadarBeamsForAntenna(f.antenna.ID, beams)
			}
			if sqlState(err) != "P0001" {
				t.Fatalf("batch failure=%v", err)
			}
			if !reflect.DeepEqual(oldCells, database.GetGridcellsForAntenna(f.antenna)) || !reflect.DeepEqual(oldBeams, database.GetRadarBeamsForAntenna(f.antenna)) {
				t.Fatal("earlier batches or deletion were committed")
			}
			assertOtherAntenna(t, f)
		})
	}
}

func TestLookupFailureDoesNotCacheMissingDevice(t *testing.T) {
	msg, gw := message(t.Name(), time.Now().UTC().Truncate(time.Microsecond))
	mustExec(t, `CREATE FUNCTION reject_device() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected device failure'; END $$`)
	mustExec(t, `CREATE TRIGGER reject_device BEFORE INSERT ON devices FOR EACH ROW EXECUTE FUNCTION reject_device()`)
	active := true
	t.Cleanup(func() {
		if active {
			mustExec(t, `DROP TRIGGER reject_device ON devices`)
		}
		mustExec(t, `DROP FUNCTION reject_device()`)
	})
	if _, err := database.UplinkMessageToPacket(msg, gw); sqlState(err) != "P0001" {
		t.Fatalf("lookup failure=%v", err)
	}
	var count int64
	if err := database.Db.Model(&database.Device{}).Where("network_id = ?", msg.NetworkId).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed lookup created %d devices", count)
	}
	mustExec(t, `DROP TRIGGER reject_device ON devices`)
	active = false
	got := insert(t, msg, gw)
	if got.DeviceID == 0 || got.ID == 0 {
		t.Fatal("retry used a missing cached device")
	}
}

func TestNullableAndExperimentFields(t *testing.T) {
	msg, gw := message(t.Name(), time.Now().UTC().Truncate(time.Microsecond))
	msg.Experiment = "integration-experiment"
	msg.AccuracyMeters = 0
	msg.Hdop = 0
	msg.Satellites = 0
	gw.Time = 0
	gw.Timestamp = 0
	gw.SignalRssi = 0
	entry := insert(t, msg, gw)
	var got database.Packet
	if err := database.Db.First(&got, entry.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.ExperimentID == nil {
		t.Fatal("experiment stored as NULL")
	}
	var experiment database.Experiment
	if err := database.Db.First(&experiment, *got.ExperimentID).Error; err != nil {
		t.Fatal(err)
	}
	if experiment.Name != msg.Experiment || got.GatewayTime != nil || got.Timestamp != nil || got.SignalRssi != nil || got.AccuracyMeters != nil || got.Hdop != nil || got.Satellites != nil || got.FineTimestamp != nil || got.FineTimestampEncrypted != nil || got.FineTimestampKeyID != nil {
		t.Fatalf("nullable/experiment fields=%+v", got)
	}
}

func TestLiveUpdateUsesCommittedRebuildIDs(t *testing.T) {
	f := seedReplay(t, t.Name())
	if err := grid_cell.ReprocessAntenna(f.antenna, f.cutoff); err != nil {
		t.Fatal(err)
	}
	if err := radar_beam.ReprocessAntenna(f.antenna, f.cutoff); err != nil {
		t.Fatal(err)
	}
	cells := database.GetGridcellsForAntenna(f.antenna)
	beams := database.GetRadarBeamsForAntenna(f.antenna)
	if len(cells) != 1 || len(beams) != 1 {
		t.Fatal("missing rebuilt aggregates")
	}
	cellID, beamID := cells[0].ID, beams[0].ID
	msg, gw := message(f.antenna.NetworkId, f.cutoff.Add(5*time.Second))
	msg.Latitude = 2
	msg.Longitude = 1
	msg.Gateways = []types.TtnMapperGateway{gw}
	grid_cell.AggregateNewData(msg)
	radar_beam.AggregateNewData(msg)
	cells = database.GetGridcellsForAntenna(f.antenna)
	beams = database.GetRadarBeamsForAntenna(f.antenna)
	if len(cells) != 1 || cells[0].ID != cellID || cells[0].Bucket105 != 3 {
		t.Fatalf("post-rebuild live cells=%+v", cells)
	}
	if len(beams) != 1 || beams[0].ID != beamID || beams[0].Samples != 3 {
		t.Fatalf("post-rebuild live beams=%+v", beams)
	}
	assertOtherAntenna(t, f)
}
