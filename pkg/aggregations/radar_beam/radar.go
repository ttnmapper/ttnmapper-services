package radar_beam

import (
	"fmt"
	"log"
	"sync"
	"time"
	utils2 "ttnmapper-services/pkg/aggregations/utils"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

const ()

var (
	Levels           = []int{-100, -105, -110, -115, -120, -200}
	radarBeamDbCache sync.Map
)

type liveStore struct {
	gateway func(database.GatewayIndexer) (database.Gateway, error)
	antenna func(string, string, uint8) database.Antenna
	get     func(uint, int, uint) (database.RadarBeam, error)
	save    func(database.RadarBeam) error
}

func AggregateNewData(message types.TtnMapperUplinkMessage) {
	aggregateNewData(message, liveStore{database.GetGateway, database.FindAntenna, getRadarBeam, database.SaveRadarBeam})
}
func aggregateNewData(message types.TtnMapperUplinkMessage, store liveStore) {
	if message.Experiment != "" || message.Latitude == 0 && message.Longitude == 0 {
		return
	}
	for _, gateway := range message.Gateways {
		location, err := store.gateway(database.GatewayIndexer{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId})
		if err != nil || location.Latitude == 0 && location.Longitude == 0 {
			continue
		}
		packet := database.Packet{Latitude: message.Latitude, Longitude: message.Longitude}
		distance := utils2.GetDistanceDatabase(location, packet)
		if !utils2.CheckDistance(distance) {
			continue
		}
		antenna := store.antenna(gateway.NetworkId, gateway.GatewayId, gateway.AntennaIndex)
		if antenna.ID == 0 {
			continue
		}
		entryTime := time.Unix(0, message.Time)
		level := getLevel(gateway)
		bearing := utils2.GetBearingDatabase(location, packet)
		entry, err := store.get(antenna.ID, level, bearing)
		if err != nil {
			continue
		}
		incrementRadarBeam(&entry, entryTime, distance*1000)
		if err := store.save(entry); err != nil {
			log.Print(err)
			continue
		}
		storeRadarBeamInCache(entry)
	}
}

func AggregateMovedGateway(movedGateway types.TtnMapperGatewayMoved) {
	movedTime := database.GetGatewayLastMovedTime(movedGateway.NetworkId, movedGateway.GatewayId)
	log.Print("Gateway ", movedGateway.GatewayId, "moved at ", movedTime)

	// Find the antenna IDs for the moved gateway
	antennas := database.GetAntennaForGateway(movedGateway.NetworkId, movedGateway.GatewayId)

	for _, antenna := range antennas {
		if err := ReprocessAntenna(antenna, movedTime); err != nil {
			log.Print(err)
			return
		}
	}
}

// ReprocessAntenna builds in local memory and replaces rows transactionally.
// Neither replay errors nor failed commits change the live cache.
func ReprocessAntenna(antenna database.Antenna, installedAtLocation time.Time) error {
	gateway, err := database.GetGateway(database.GatewayIndexer{NetworkId: antenna.NetworkId, GatewayId: antenna.GatewayId})
	if err != nil {
		return fmt.Errorf("load gateway for replay: %w", err)
	}
	if gateway.Latitude == 0 && gateway.Longitude == 0 {
		return fmt.Errorf("gateway location unknown")
	}
	rows, err := database.GetPacketsForAntennaAfter(antenna, installedAtLocation)
	if err != nil {
		return fmt.Errorf("read replay packets: %w", err)
	}
	defer rows.Close()
	rebuilt := map[database.RadarBeamIndexer]database.RadarBeam{}
	for rows.Next() {
		var packet database.Packet
		if err := database.ScanRows(rows, &packet); err != nil {
			return fmt.Errorf("scan replay packet: %w", err)
		}
		distance := utils2.GetDistanceDatabase(gateway, packet)
		if !utils2.CheckDistance(distance) {
			continue
		}
		level := getLevelFromPacket(packet)
		bearing := utils2.GetBearingDatabase(gateway, packet)
		key := database.RadarBeamIndexer{AntennaID: antenna.ID, Level: level, Bearing: bearing}
		beam, ok := rebuilt[key]
		if !ok {
			beam = database.RadarBeam{AntennaID: antenna.ID, Level: level, Bearing: bearing}
		}
		incrementRadarBeam(&beam, packet.Time, distance*1000)
		rebuilt[key] = beam
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate replay packets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close replay packets: %w", err)
	}
	entries := make([]database.RadarBeam, 0, len(rebuilt))
	for _, entry := range rebuilt {
		entries = append(entries, entry)
	}
	return replaceRebuilt(antenna.ID, entries, database.ReplaceRadarBeamsForAntenna)
}

// Publish the replacement cache only after the database commit succeeds.
func replaceRebuilt(antennaID uint, entries []database.RadarBeam, replace func(uint, []database.RadarBeam) error) error {
	if err := replace(antennaID, entries); err != nil {
		return fmt.Errorf("replace rebuilt aggregates: %w", err)
	}
	radarBeamDbCache.Range(func(key, value interface{}) bool {
		if key.(database.RadarBeamIndexer).AntennaID == antennaID {
			radarBeamDbCache.Delete(key)
		}
		return true
	})
	for _, entry := range entries {
		storeRadarBeamInCache(entry)
	}
	return nil
}

func getLevelFromPacket(packet database.Packet) int {
	gateway := types.TtnMapperGateway{Rssi: packet.Rssi, Snr: packet.Snr}
	return getLevel(gateway)
}

func getLevel(gateway types.TtnMapperGateway) int {
	signal := gateway.Rssi
	if gateway.Snr < 0 {
		signal = gateway.Rssi + gateway.Snr
	}

	for _, level := range Levels {
		// -50 > -100
		if signal > float32(level) {
			return level
		}
	}

	// If not found, return last level
	return Levels[len(Levels)-1]
}

func getRadarBeam(antennaId uint, level int, bearing uint) (database.RadarBeam, error) {
	radarBeamDb := database.RadarBeam{}
	var err error

	// Try and find in cache first
	radarBeamIndexer := database.RadarBeamIndexer{
		AntennaID: antennaId,
		Level:     level,
		Bearing:   bearing,
	}
	i, ok := radarBeamDbCache.Load(radarBeamIndexer)
	if ok {
		radarBeamDb = i.(database.RadarBeam)
	} else {
		radarBeamDb, err = database.GetRadarBeam(radarBeamIndexer)
		if err != nil {
			log.Print(antennaId, level, bearing)
			return database.RadarBeam{}, err
		}
	}
	return radarBeamDb, nil
}

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

func incrementRadarBeam(radarBeam *database.RadarBeam, entryTime time.Time, distance float64) {
	radarBeam.Samples++

	if entryTime.After(radarBeam.LastUpdated) {
		radarBeam.LastUpdated = entryTime
	}

	previousMax := radarBeam.DistanceMax
	previous2nd := radarBeam.Distance2nd

	if distance > previousMax {
		radarBeam.DistanceMax = distance
		radarBeam.Distance2nd = previousMax
	} else if distance > previous2nd {
		radarBeam.Distance2nd = distance
	}
}

func storeRadarBeamInCache(radarBeam database.RadarBeam) {
	radarBeamIndexer := database.RadarBeamIndexer{AntennaID: radarBeam.AntennaID, Level: radarBeam.Level, Bearing: radarBeam.Bearing}
	radarBeamDbCache.Store(radarBeamIndexer, radarBeam)
}
