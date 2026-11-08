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
	rebuilt := map[database.GridCellIndexer]database.GridCell{}
	for rows.Next() {
		var packet database.Packet
		if err := database.ScanRows(rows, &packet); err != nil {
			return fmt.Errorf("scan replay packet: %w", err)
		}
		distance := utils2.GetDistanceDatabase(gateway, packet)
		if !utils2.CheckDistance(distance) {
			continue
		}
		cell, err := newGridCell(antenna.ID, packet.Latitude, packet.Longitude)
		if err != nil {
			continue
		}
		key := database.GridCellIndexer{AntennaID: antenna.ID, X: cell.X, Y: cell.Y}
		if previous, ok := rebuilt[key]; ok {
			cell = previous
		}
		incrementBucket(&cell, packet.Time, packet.Rssi, packet.Snr)
		rebuilt[key] = cell
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate replay packets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close replay packets: %w", err)
	}
	entries := make([]database.GridCell, 0, len(rebuilt))
	for _, entry := range rebuilt {
		entries = append(entries, entry)
	}
	return replaceRebuilt(antenna.ID, entries, database.ReplaceGridCellsForAntenna)
}

// Publish the replacement cache only after the database commit succeeds.
func replaceRebuilt(antennaID uint, entries []database.GridCell, replace func(uint, []database.GridCell) error) error {
	if err := replace(antennaID, entries); err != nil {
		return fmt.Errorf("replace rebuilt aggregates: %w", err)
	}
	gridCellDbCache.Range(func(key, value interface{}) bool {
		if key.(database.GridCellIndexer).AntennaID == antennaID {
			gridCellDbCache.Delete(key)
		}
		return true
	})
	for _, entry := range entries {
		storeGridCellInCache(entry)
	}
	return nil
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

func newGridCell(antennaID uint, latitude, longitude float64) (database.GridCell, error) {
	if latitude < -85 || latitude > 85 {
		return database.GridCell{}, errors.New("coordinates out of range")
	}
	if latitude == 0 && longitude == 0 {
		return database.GridCell{}, errors.New("null island")
	}
	tile := gosm.NewTileWithLatLong(latitude, longitude, 19)
	return database.GridCell{AntennaID: antennaID, X: tile.X, Y: tile.Y}, nil
}

func getGridCellNotDb(antennaID uint, latitude, longitude float64) (database.GridCell, error) {
	cell, err := newGridCell(antennaID, latitude, longitude)
	if err != nil {
		return cell, err
	}
	if cached, ok := gridCellDbCache.Load(database.GridCellIndexer{AntennaID: antennaID, X: cell.X, Y: cell.Y}); ok {
		return cached.(database.GridCell), nil
	}
	return cell, nil
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
