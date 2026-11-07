package database

import (
	"database/sql"
	"time"
	"ttnmapper-services/pkg/types"
	"ttnmapper-services/pkg/utils"
)

/*
Run started 2021-09-17 11:45 UTC
*/

func UplinkMessageToPacket(message types.TtnMapperUplinkMessage, gateway types.TtnMapperGateway) (Packet, error) {
	return uplinkMessageToPacket(message, gateway, func(record interface{}) error { return Db.FirstOrCreate(record, record).Error })
}

func uplinkMessageToPacket(message types.TtnMapperUplinkMessage, gateway types.TtnMapperGateway, resolve func(interface{}) error) (Packet, error) {
	var entry = Packet{}

	// Time
	seconds := message.Time / 1000000000
	nanos := message.Time % 1000000000
	entry.Time = time.Unix(seconds, nanos)

	// DeviceID
	deviceIndexer := DeviceIndexer{NetworkId: message.NetworkId, AppId: message.AppID, DevId: message.DevID, DevEui: message.DevEui}
	i, ok := deviceDbCache.Load(deviceIndexer)
	if ok {
		entry.DeviceID = i.(uint)
	} else {
		//log.Println("Get or create device from/in DB:", deviceIndexer)
		deviceDb := Device{NetworkId: message.NetworkId, AppId: message.AppID, DevId: message.DevID, DevEui: message.DevEui}
		err := resolve(&deviceDb)
		if err != nil {
			return entry, err
		}
		entry.DeviceID = deviceDb.ID
		deviceDbCache.Store(deviceIndexer, deviceDb.ID)
	}

	// FPort, FCnt
	entry.FPort = message.FPort
	entry.FCnt = uint32(message.FCnt)

	// FrequencyID
	i, ok = frequencyDbCache.Load(message.Frequency)
	if ok {
		entry.FrequencyID = i.(uint)
	} else {
		frequencyDb := Frequency{Herz: message.Frequency}
		err := resolve(&frequencyDb)
		if err != nil {
			return entry, err
		}
		entry.FrequencyID = frequencyDb.ID
		frequencyDbCache.Store(message.Frequency, frequencyDb.ID)
	}

	// DataRateID
	dataRateIndexer := DataRateIndexer{
		Modulation:      message.Modulation,
		Bandwidth:       message.Bandwidth,
		SpreadingFactor: message.SpreadingFactor,
		Bitrate:         message.Bitrate}
	i, ok = dataRateDbCache.Load(dataRateIndexer)
	if ok {
		entry.DataRateID = i.(uint)
	} else {
		dataRateDb := DataRate{
			Modulation:      message.Modulation,
			Bandwidth:       message.Bandwidth,
			SpreadingFactor: message.SpreadingFactor,
			Bitrate:         message.Bitrate}
		err := resolve(&dataRateDb)
		if err != nil {
			return entry, err
		}
		entry.DataRateID = dataRateDb.ID
		dataRateDbCache.Store(dataRateIndexer, dataRateDb.ID)
	}

	// CodingRateID
	i, ok = codingRateDbCache.Load(message.CodingRate)
	if ok {
		entry.CodingRateID = i.(uint)
	} else {
		codingRateDb := CodingRate{Name: message.CodingRate}
		err := resolve(&codingRateDb)
		if err != nil {
			return entry, err
		}
		entry.CodingRateID = codingRateDb.ID
		codingRateDbCache.Store(message.CodingRate, codingRateDb.ID)
	}

	// AntennaID - packets are stored with a pointer to the antenna that received it. A network has multiple gateways, a gateway has multiple antennas.
	// We therefore store coverage data per antenna, assuming antenna index 0 when we don't know the antenna index.
	antennaIndexer := AntennaIndexer{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId, AntennaIndex: gateway.AntennaIndex}
	i, ok = antennaDbCache.Load(antennaIndexer)
	if ok {
		entry.AntennaID = i.(uint)
	} else {
		antennaDb := Antenna{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId, AntennaIndex: gateway.AntennaIndex}
		err := resolve(&antennaDb)
		if err != nil {
			return entry, err
		}
		entry.AntennaID = antennaDb.ID
		antennaDbCache.Store(antennaIndexer, antennaDb.ID)
	}

	// GatewayTime
	if gateway.Time != 0 {
		seconds = gateway.Time / 1000000000
		nanos = gateway.Time % 1000000000
		gatewayTime := time.Unix(seconds, nanos)
		entry.GatewayTime = &gatewayTime
	}

	// Timestamp
	if gateway.Timestamp != 0 {
		entry.Timestamp = &gateway.Timestamp
	}

	// FineTimestamp
	if gateway.FineTimestamp != 0 {
		entry.FineTimestamp = &gateway.FineTimestamp
	}

	// FineTimestampEncrypted
	if len(gateway.FineTimestampEncrypted) > 0 {
		entry.FineTimestampEncrypted = &gateway.FineTimestampEncrypted
	}

	// FineTimestampKeyID
	if gateway.FineTimestampEncryptedKeyId != "" {
		// TODO: cache if this is done often
		fineTimestampKeyId := FineTimestampKeyID{FineTimestampEncryptedKeyId: gateway.FineTimestampEncryptedKeyId}
		err := resolve(&fineTimestampKeyId)
		if err != nil {
			return entry, err
		}
		entry.FineTimestampKeyID = &fineTimestampKeyId.ID
	}

	// ChannelIndex
	entry.ChannelIndex = gateway.ChannelIndex

	// Rssi, SignalRssi, Snr
	entry.Rssi = gateway.Rssi
	if gateway.SignalRssi != 0 {
		entry.SignalRssi = &gateway.SignalRssi
	}
	entry.Snr = gateway.Snr

	// Latitude, Longitude, Altitude, AccuracyMeters, Satellites, Hdop
	entry.Latitude = utils.CapFloatTo(message.Latitude, 10, 6)
	entry.Longitude = utils.CapFloatTo(message.Longitude, 10, 6)
	entry.Altitude = utils.CapFloatTo(message.Altitude, 6, 1)

	if message.AccuracyMeters != 0 {
		accuracy := utils.CapFloatTo(message.AccuracyMeters, 6, 2)
		entry.AccuracyMeters = &accuracy
	}
	if message.Satellites != 0 {
		entry.Satellites = &message.Satellites
	}
	if message.Hdop != 0 {
		hdop := utils.CapFloatTo(message.Hdop, 3, 1)
		entry.Hdop = &hdop
	}

	// AccuracySourceID
	i, ok = accuracySourceDbCache.Load(message.AccuracySource)
	if ok {
		entry.AccuracySourceID = i.(uint)
	} else {
		accuracySourceDb := AccuracySource{Name: message.AccuracySource}
		err := resolve(&accuracySourceDb)
		if err != nil {
			return entry, err
		}
		entry.AccuracySourceID = accuracySourceDb.ID
		accuracySourceDbCache.Store(message.AccuracySource, accuracySourceDb.ID)
	}

	// ExperimentID
	if message.Experiment != "" {
		i, ok = experimentNameDbCache.Load(message.Experiment)
		if ok {
			experimentId := i.(uint)
			entry.ExperimentID = &experimentId
		} else {
			experimentNameDb := Experiment{Name: message.Experiment}
			err := resolve(&experimentNameDb)
			if err != nil {
				return entry, err
			}
			entry.ExperimentID = &experimentNameDb.ID
			experimentNameDbCache.Store(message.Experiment, experimentNameDb.ID)
		}
	}

	// UserID
	i, ok = userIdDbCache.Load(message.UserId)
	if ok {
		entry.UserID = i.(uint)
	} else {
		userIdDb := User{Identifier: message.UserId}
		err := resolve(&userIdDb)
		if err != nil {
			return entry, err
		}
		entry.UserID = userIdDb.ID
		userIdDbCache.Store(message.UserId, userIdDb.ID)
	}

	// UserAgentID
	i, ok = userAgentDbCache.Load(message.UserAgent)
	if ok {
		entry.UserAgentID = i.(uint)
	} else {
		userAgentDb := UserAgent{Name: message.UserAgent}
		err := resolve(&userAgentDb)
		if err != nil {
			return entry, err
		}
		entry.UserAgentID = userAgentDb.ID
		userAgentDbCache.Store(message.UserAgent, userAgentDb.ID)
	}

	return entry, nil
}

func ScanRows(row *sql.Rows, result interface{}) error {
	return Db.ScanRows(row, result)
}
