package radar_beam

import (
	"fmt"
	"log"
	"sync"
	"time"
	utils2 "ttnmapper-services/pkg/aggregations/utils"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
	"ttnmapper-services/pkg/utils"
)

const ()

var (
	Levels           = []int{-100, -105, -110, -115, -120, -200}
	radarBeamDbCache sync.Map
)

func AggregateNewData(message types.TtnMapperUplinkMessage) {
	if message.Experiment != "" {
		return
	}
	if message.Latitude == 0 && message.Longitude == 0 {
		return
	}

	// Iterate gateways. We store it flat in the database
	for _, gateway := range message.Gateways {
		// If the point is too far from the gateway, ignore it
		if !utils2.CheckDistanceFromGateway(gateway, message) {
			continue
		}

		antennaID := database.FindAntenna(gateway.NetworkId, gateway.GatewayId, gateway.AntennaIndex).ID
		if antennaID == 0 {
			log.Println("Can't find antenna in database")
			continue
		}
		log.Print("AntennaID ", antennaID)

		seconds := message.Time / 1000000000
		nanos := message.Time % 1000000000
		entryTime := time.Unix(seconds, nanos)

		level := getLevel(gateway)
		bearing := utils2.GetBearingLive(gateway, message)
		distance := utils2.GetDistanceLive(gateway, message) * 1000.0 // km to metres

		radarBeam, err := getRadarBeam(antennaID, level, bearing)
		if err != nil {
			continue
		}
		incrementRadarBeam(&radarBeam, entryTime, distance)
		storeRadarBeamInCache(radarBeam)
		database.SaveRadarBeam(radarBeam)
	}
}

func AggregateMovedGateway(movedGateway types.TtnMapperGatewayMoved) {
	movedTime := database.GetGatewayLastMovedTime(movedGateway.NetworkId, movedGateway.GatewayId)
	log.Print("Gateway ", movedGateway.GatewayId, "moved at ", movedTime)

	// Find the antenna IDs for the moved gateway
	antennas := database.GetAntennaForGateway(movedGateway.NetworkId, movedGateway.GatewayId)

	for _, antenna := range antennas {
		ReprocessAntenna(antenna, movedTime)
	}
}

func ReprocessAntenna(antenna database.Antenna, installedAtLocation time.Time) {

	// Get the gateway this antenna belongs to
	gatewayIndexer := database.GatewayIndexer{
		NetworkId: antenna.NetworkId,
		GatewayId: antenna.GatewayId,
	}
	gateway, err := database.GetGateway(gatewayIndexer)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// Get a list of beams to delete
	radarBeams := database.GetRadarBeamsForAntenna(antenna)

	// Remove from local cache
	for _, radarBeam := range radarBeams {
		radarBeamIndexer := database.RadarBeamIndexer{AntennaID: radarBeam.AntennaID, Level: radarBeam.Level, Bearing: radarBeam.Bearing}
		radarBeamDbCache.Delete(radarBeamIndexer)
	}

	gatewayRadarBeams := map[database.RadarBeamIndexer]database.RadarBeam{}

	// Get all existing packets since gateway last moved
	rows, err := database.GetPacketsForAntennaAfter(antenna, installedAtLocation)
	if err != nil {
		log.Println(err.Error())
		return
	}

	i := 0
	for rows.Next() {
		i++
		fmt.Printf("\rPacket %d   ", i)

		var packet database.Packet
		err := database.ScanRows(rows, &packet)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		// If the point is too far from the gateway, ignore it
		if !utils2.CheckDistanceFromAntenna(antenna, packet) {
			log.Println("too far away", packet.ID)
			continue
		}

		level := getLevelFromPacket(packet)
		bearing := utils2.GetBearingDatabase(gateway, packet)
		distance := utils2.GetDistanceDatabase(gateway, packet) * 1000.0 // km to metres

		radarBeam, err := getRadarBeamNotDb(antenna.ID, level, bearing)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		incrementRadarBeam(&radarBeam, packet.Time, distance)
		storeRadarBeamInCache(radarBeam) // so that we don't read it again from the database

		// Also store in a map to write to the database later
		radarBeamIndexer := database.RadarBeamIndexer{AntennaID: radarBeam.AntennaID, Level: radarBeam.Level, Bearing: radarBeam.Bearing}
		gatewayRadarBeams[radarBeamIndexer] = radarBeam
	}
	err = rows.Close()
	if err != nil {
		log.Println(err.Error())
	}

	// Delete old cells from database
	database.DeleteRadarBeamsForAntenna(antenna)

	if len(gatewayRadarBeams) == 0 {
		log.Println("No packets")
		return
	}
	fmt.Println()

	// Then add new ones
	log.Printf("Result is %d radar beams", len(gatewayRadarBeams))
	err = storeRadarBeamsInDb(gatewayRadarBeams)
	if err != nil {
		log.Fatalf(err.Error())
	}

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
			utils.FailOnError(err, "Failed to find db entry for radar beam")
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

func storeRadarBeamsInDb(radarBeams map[database.RadarBeamIndexer]database.RadarBeam) error {
	if len(radarBeams) == 0 {
		log.Println("No radar beams to insert")
		return nil
	}

	radarBeamSlice := make([]database.RadarBeam, 0)
	for _, val := range radarBeams {
		radarBeamSlice = append(radarBeamSlice, val)
	}

	return database.CreateRadarBeams(radarBeamSlice)
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
