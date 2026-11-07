package database

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
	"ttnmapper-services/pkg/types"
)

func resetPacketCaches(t *testing.T) {
	caches := []*sync.Map{&deviceDbCache, &antennaDbCache, &dataRateDbCache, &codingRateDbCache, &frequencyDbCache, &accuracySourceDbCache, &userAgentDbCache, &userIdDbCache, &experimentNameDbCache}
	for _, c := range caches {
		*c = sync.Map{}
	}
	t.Cleanup(func() {
		for _, c := range caches {
			*c = sync.Map{}
		}
	})
}
func packetFixture() (types.TtnMapperUplinkMessage, types.TtnMapperGateway) {
	return types.TtnMapperUplinkMessage{NetworkId: "network", AppID: "app", DevID: "device", DevEui: "0011223344556677", Time: 1600000000123456789, FPort: 2, FCnt: 42, Frequency: 868100000, Modulation: "LORA", Bandwidth: 125000, SpreadingFactor: 7, CodingRate: "4/5", Latitude: -33.9123456, Longitude: 18.8123456, Altitude: 123.45, AccuracyMeters: 3.456, Satellites: 8, Hdop: 1.23, AccuracySource: "gps", Experiment: "test", UserId: "mapper", UserAgent: "unit-test"}, types.TtnMapperGateway{NetworkId: "network", GatewayId: "gateway", AntennaIndex: 1, Time: 1600000000987654321, Timestamp: 123, FineTimestamp: 456, FineTimestampEncrypted: []byte{1, 2}, FineTimestampEncryptedKeyId: "key", ChannelIndex: 3, Rssi: -100, SignalRssi: -101, Snr: -5}
}
func TestUplinkMessageToPacket(t *testing.T) {
	resetPacketCaches(t)
	msg, gw := packetFixture()
	ids := map[string]uint{}
	records := map[string]interface{}{}
	resolve := func(record interface{}) error {
		v := reflect.ValueOf(record).Elem()
		name := v.Type().Name()
		records[name] = v.Interface()
		id := uint(len(ids) + 1)
		ids[name] = id
		v.FieldByName("ID").SetUint(uint64(id))
		return nil
	}
	got, err := uplinkMessageToPacket(msg, gw, resolve)
	if err != nil {
		t.Fatal(err)
	}
	device := records["Device"].(Device)
	if device.NetworkId != msg.NetworkId || device.AppId != msg.AppID || device.DevId != msg.DevID || device.DevEui != msg.DevEui {
		t.Fatalf("resolved device=%+v", device)
	}
	rate := records["DataRate"].(DataRate)
	if rate.Modulation != "LORA" || rate.Bandwidth != 125000 || rate.SpreadingFactor != 7 || rate.Bitrate != 0 || records["Frequency"].(Frequency).Herz != 868100000 || records["CodingRate"].(CodingRate).Name != "4/5" {
		t.Fatalf("resolved radio=%+v", rate)
	}
	antenna := records["Antenna"].(Antenna)
	if antenna.NetworkId != gw.NetworkId || antenna.GatewayId != gw.GatewayId || antenna.AntennaIndex != 1 {
		t.Fatalf("resolved antenna=%+v", antenna)
	}
	if got.Time.UnixNano() != msg.Time || got.GatewayTime == nil || got.GatewayTime.UnixNano() != gw.Time {
		t.Fatalf("timestamps=%+v", got)
	}
	if got.FPort != 2 || got.FCnt != 42 || got.DeviceID != ids["Device"] || got.FrequencyID != ids["Frequency"] || got.DataRateID != ids["DataRate"] || got.CodingRateID != ids["CodingRate"] || got.AntennaID != ids["Antenna"] {
		t.Fatalf("identity/radio=%+v", got)
	}
	if got.Latitude != msg.Latitude || got.Longitude != msg.Longitude || got.Altitude != msg.Altitude || got.AccuracyMeters == nil || *got.AccuracyMeters != msg.AccuracyMeters || got.Hdop == nil || *got.Hdop != msg.Hdop || got.Satellites == nil || *got.Satellites != 8 {
		t.Fatalf("coordinates=%+v", got)
	}
	if got.Timestamp == nil || *got.Timestamp != 123 || got.FineTimestamp == nil || *got.FineTimestamp != 456 || got.FineTimestampEncrypted == nil || !reflect.DeepEqual(*got.FineTimestampEncrypted, []byte{1, 2}) || got.FineTimestampKeyID == nil || *got.FineTimestampKeyID != ids["FineTimestampKeyID"] {
		t.Fatalf("gateway fields=%+v", got)
	}
	if got.ChannelIndex != 3 || got.Rssi != -100 || got.SignalRssi == nil || *got.SignalRssi != -101 || got.Snr != -5 || got.ExperimentID == nil || *got.ExperimentID != ids["Experiment"] || got.UserID != ids["User"] || got.UserAgentID != ids["UserAgent"] || got.AccuracySourceID != ids["AccuracySource"] {
		t.Fatalf("optional fields=%+v", got)
	}
}
func TestPacketAbsentOptionalFields(t *testing.T) {
	resetPacketCaches(t)
	got, err := uplinkMessageToPacket(types.TtnMapperUplinkMessage{}, types.TtnMapperGateway{}, func(record interface{}) error {
		reflect.ValueOf(record).Elem().FieldByName("ID").SetUint(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Time.Equal(time.Unix(0, 0)) || got.GatewayTime != nil || got.Timestamp != nil || got.FineTimestamp != nil || got.FineTimestampEncrypted != nil || got.FineTimestampKeyID != nil || got.SignalRssi != nil || got.AccuracyMeters != nil || got.Satellites != nil || got.Hdop != nil || got.ExperimentID != nil {
		t.Fatalf("unexpected optional fields=%+v", got)
	}
}
func TestPacketLookupFailuresAreRetried(t *testing.T) {
	for _, kind := range []string{"Device", "Frequency", "DataRate", "CodingRate", "Antenna", "FineTimestampKeyID", "AccuracySource", "Experiment", "User", "UserAgent"} {
		t.Run(kind, func(t *testing.T) {
			resetPacketCaches(t)
			msg, gw := packetFixture()
			failure := errors.New("lookup failed")
			failed := false
			resolve := func(record interface{}) error {
				v := reflect.ValueOf(record).Elem()
				if v.Type().Name() == kind && !failed {
					failed = true
					return failure
				}
				v.FieldByName("ID").SetUint(7)
				return nil
			}
			if _, err := uplinkMessageToPacket(msg, gw, resolve); !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			got, err := uplinkMessageToPacket(msg, gw, resolve)
			if err != nil || got.UserAgentID != 7 {
				t.Fatalf("retry=%+v error=%v", got, err)
			}
		})
	}
}
func TestPacketCachesSeparateIdentities(t *testing.T) {
	resetPacketCaches(t)
	msg, gw := packetFixture()
	gw.FineTimestampEncryptedKeyId = ""
	calls := map[string]int{}
	resolve := func(record interface{}) error {
		v := reflect.ValueOf(record).Elem()
		kind := v.Type().Name()
		calls[kind]++
		v.FieldByName("ID").SetUint(uint64(calls[kind]))
		return nil
	}
	first, err := uplinkMessageToPacket(msg, gw, resolve)
	if err != nil {
		t.Fatal(err)
	}
	again, err := uplinkMessageToPacket(msg, gw, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, again) {
		t.Fatal("cached conversion differs")
	}
	for kind, count := range calls {
		if count != 1 {
			t.Errorf("%s lookups=%d", kind, count)
		}
	}
	msg.NetworkId = "other"
	gw.NetworkId = "other"
	gw.AntennaIndex = 2
	msg.Frequency = 868300000
	other, err := uplinkMessageToPacket(msg, gw, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if other.DeviceID == first.DeviceID || other.AntennaID == first.AntennaID || other.FrequencyID == first.FrequencyID || other.UserID != first.UserID {
		t.Fatalf("cache identity mismatch: first=%+v other=%+v", first, other)
	}
}

func TestPacketNumericLimits(t *testing.T) {
	resetPacketCaches(t)
	message := types.TtnMapperUplinkMessage{Latitude: 20000, Longitude: -20000, Altitude: 200000, AccuracyMeters: 20000, Hdop: 200}
	got, err := uplinkMessageToPacket(message, types.TtnMapperGateway{}, func(record interface{}) error {
		reflect.ValueOf(record).Elem().FieldByName("ID").SetUint(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Latitude != 9999.999999 || got.Longitude != -9999.999999 || got.Altitude != 99999.9 || got.AccuracyMeters == nil || *got.AccuracyMeters != 9999.99 || got.Hdop == nil || *got.Hdop != 99.9 {
		t.Fatalf("numeric caps=%+v", got)
	}
}
