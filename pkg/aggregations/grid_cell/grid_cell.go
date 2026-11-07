package grid_cell

import (
	"errors"
	"fmt"
	"github.com/j4/gosm"
	"log"
	"sync"
	"time"
	utils2 "ttnmapper-services/pkg/aggregations/utils"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

var (
	gridCellDbCache sync.Map
)

type liveStore struct {
	gateway func(database.GatewayIndexer) (database.Gateway, error)
	antenna func(string, string, uint8) database.Antenna
	get     func(uint, float64, float64) (database.GridCell, error)
	save    func(database.GridCell) error
}

func AggregateNewData(message types.TtnMapperUplinkMessage) {
	aggregateNewData(message, liveStore{database.GetGateway, database.FindAntenna, getGridCell, database.SaveGridCell})
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
		entry, err := store.get(antenna.ID, message.Latitude, message.Longitude)
		if err != nil {
			continue
		}
		incrementBucket(&entry, entryTime, gateway.Rssi, gateway.Snr)
		if err := store.save(entry); err != nil {
			log.Print(err)
			continue
		}
		storeGridCellInCache(entry)
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
	// Get a list of grid cells to delete
	gridCells := database.GetGridcellsForAntenna(antenna)

	// Remove from local cache
	for _, gridCell := range gridCells {
		gridCellIndexer := database.GridCellIndexer{AntennaID: gridCell.AntennaID, X: gridCell.X, Y: gridCell.Y}
		gridCellDbCache.Delete(gridCellIndexer)
	}

	gatewayGridCells := map[database.GridCellIndexer]database.GridCell{}

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
			continue
		}

		gridCell, err := getGridCellNotDb(antenna.ID, packet.Latitude, packet.Longitude) // Do not create now as we will do a batch insert later
		if err != nil {
			continue
		}
		incrementBucket(&gridCell, packet.Time, packet.Rssi, packet.Snr)
		storeGridCellInCache(gridCell) // so that we don't read it again from the database

		// Also store in a map of gridcells we will write to the database later
		gridCellIndexer := database.GridCellIndexer{AntennaID: gridCell.AntennaID, X: gridCell.X, Y: gridCell.Y}
		gatewayGridCells[gridCellIndexer] = gridCell
	}
	err = rows.Close()
	if err != nil {
		log.Println(err.Error())
	}

	// Delete old cells from database
	database.DeleteGridCellsForAntenna(antenna)

	if len(gatewayGridCells) == 0 {
		log.Println("No packets")
		return
	}
	fmt.Println()

	// Then add new ones
	log.Printf("Result is %d grid cells", len(gatewayGridCells))
	// todo upsert as the other process could have added cells in the meantime
	err = storeGridCellsInDb(gatewayGridCells)
	if err != nil {
		log.Fatalf(err.Error())
	}
}

func getGridCell(antennaId uint, latitude float64, longitude float64) (database.GridCell, error) {
	// https://blog.jochentopf.com/2013-02-04-antarctica-in-openstreetmap.html
	// The Mercator projection generally used in online maps only covers the area between about 85.0511 degrees South and 85.0511 degrees North.
	if latitude < -85 || latitude > 85 {
		// We get a tile index that is invalid if we try handling -90,-180
		return database.GridCell{}, errors.New("coordinates out of range")
	}
	if latitude == 0 && longitude == 0 {
		// We get a tile index that is invalid if we try handling -90,-180
		return database.GridCell{}, errors.New("null island")
	}

	tile := gosm.NewTileWithLatLong(latitude, longitude, 19)

	gridCellDb := database.GridCell{}
	var err error

	// Try and find in cache first
	gridCellIndexer := database.GridCellIndexer{AntennaID: antennaId, X: tile.X, Y: tile.Y}
	i, ok := gridCellDbCache.Load(gridCellIndexer)
	if ok {
		gridCellDb = i.(database.GridCell)
	} else {
		gridCellDb, err = database.GetGridCell(gridCellIndexer)
		if err != nil {
			log.Print(antennaId, latitude, longitude, tile.X, tile.Y)
			return database.GridCell{}, err
		}
	}
	return gridCellDb, nil
}

func getGridCellNotDb(antennaId uint, latitude float64, longitude float64) (database.GridCell, error) {
	// https://blog.jochentopf.com/2013-02-04-antarctica-in-openstreetmap.html
	// The Mercator projection generally used in online maps only covers the area between about 85.0511 degrees South and 85.0511 degrees North.
	if latitude < -85 || latitude > 85 {
		// We get a tile index that is invalid if we try handling -90,-180
		return database.GridCell{}, errors.New("coordinates out of range")
	}
	if latitude == 0 && longitude == 0 {
		// We get a tile index that is invalid if we try handling -90,-180
		return database.GridCell{}, errors.New("null island")
	}

	tile := gosm.NewTileWithLatLong(latitude, longitude, 19)

	gridCell := database.GridCell{}

	// Try and find in cache
	gridCellIndexer := database.GridCellIndexer{AntennaID: antennaId, X: tile.X, Y: tile.Y}
	i, ok := gridCellDbCache.Load(gridCellIndexer)
	if ok {
		gridCell = i.(database.GridCell)
		//log.Print("Found grid cell in cache")
	} else {
		gridCell.AntennaID = antennaId
		gridCell.X = tile.X
		gridCell.Y = tile.Y
	}
	return gridCell, nil
}

func storeGridCellInCache(gridCell database.GridCell) {
	// Save to cache
	gridCellIndexer := database.GridCellIndexer{AntennaID: gridCell.AntennaID, X: gridCell.X, Y: gridCell.Y}
	gridCellDbCache.Store(gridCellIndexer, gridCell)
}

//func storeGridCellInTempCache(tempCache *sync.Map, gridCell types.GridCell) {
//	// Save to cache
//	gridCellIndexer := types.GridCellIndexer{AntennaID: gridCell.AntennaID, X: gridCell.X, Y: gridCell.Y}
//	tempCache.Store(gridCellIndexer, gridCell)
//}

//func storeGridCellInDb(gridCell database.GridCell) {
//	database.SaveGridCell(&gridCell)
//}

func storeGridCellsInDb(gridCells map[database.GridCellIndexer]database.GridCell) error {
	if len(gridCells) == 0 {
		log.Println("No grid cells to insert")
		return nil
	}

	gridCellsSlice := make([]database.GridCell, 0)
	for _, val := range gridCells {
		gridCellsSlice = append(gridCellsSlice, val)
	}

	return database.CreateGridCells(gridCellsSlice)
}

func incrementBucket(gridCell *database.GridCell, time time.Time, rssi float32, snr float32) {
	signal := rssi
	if snr < 0 {
		signal += snr
	}

	if signal > -95 {
		gridCell.BucketHigh++
	} else if signal > -100 {
		gridCell.Bucket100++
	} else if signal > -105 {
		gridCell.Bucket105++
	} else if signal > -110 {
		gridCell.Bucket110++
	} else if signal > -115 {
		gridCell.Bucket115++
	} else if signal > -120 {
		gridCell.Bucket120++
	} else if signal > -125 {
		gridCell.Bucket125++
	} else if signal > -130 {
		gridCell.Bucket130++
	} else if signal > -135 {
		gridCell.Bucket135++
	} else if signal > -140 {
		gridCell.Bucket140++
	} else if signal > -145 {
		gridCell.Bucket145++
	} else {
		gridCell.BucketLow++
	}

	if time.After(gridCell.LastUpdated) {
		gridCell.LastUpdated = time
	}
}
