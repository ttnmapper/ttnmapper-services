package database

import (
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"log"
	"strings"
	"time"
	"ttnmapper-services/pkg/types"
)

func InsertGatewayLocationsBatch(gatewayLocations []GatewayLocation) error {
	if len(gatewayLocations) == 0 {
		return errors.New("nothing to insert")
	}

	tx := Db.Begin()
	valueStrings := []string{}
	valueArgs := []interface{}{}
	for _, location := range gatewayLocations {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?)")
		valueArgs = append(valueArgs, location.NetworkId)
		valueArgs = append(valueArgs, location.GatewayId)
		valueArgs = append(valueArgs, location.InstalledAt)
		valueArgs = append(valueArgs, location.Latitude)
		valueArgs = append(valueArgs, location.Longitude)
		valueArgs = append(valueArgs, location.Altitude)
	}

	stmt := fmt.Sprintf("INSERT INTO gateway_locations (network_id, gateway_id, installed_at, latitude, longitude, altitude) VALUES %s", strings.Join(valueStrings, ","))
	err := tx.Exec(stmt, valueArgs...).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit().Error
	//tx.Rollback()
	return err
}

func GetAllGateways() []Gateway {
	var gateways []Gateway
	Db.Order("id asc").Find(&gateways)
	return gateways
}

func GetAllGatewaysForNetwork(networkId string) []Gateway {
	var gateways []Gateway
	Db.Where("network_id = ?", networkId).Order("id asc").Find(&gateways)
	return gateways
}

func GetGatewaysWithId(gatewayId string) []Gateway {
	var gateways []Gateway
	Db.Where("gateway_id = ?", gatewayId).Find(&gateways)
	return gateways
}

func GetGatewaysByNameOrId(gatewaySearch string) []Gateway {
	query := `
	SELECT * FROM gateways
	WHERE gateway_id = ?
	OR name = ?`

	rows, err := Db.Raw(query, gatewaySearch, gatewaySearch).Rows()
	result := make([]Gateway, 0)
	for rows.Next() {
		measurement := Gateway{}
		err = Db.ScanRows(rows, &measurement)
		if err != nil {
			log.Println(err.Error())
		}
		result = append(result, measurement)
	}
	return result
}

func GetOnlineGatewaysForNetwork(networkId string) []GatewayWithBoundingBox {

	if cacheGateways, ok := networkOnlineGatewaysCache.Get(networkId); ok {
		return cacheGateways.([]GatewayWithBoundingBox)
	}

	var gateways []GatewayWithBoundingBox
	Db.Table("gateways").
		Select("gateways.id, gateways.network_id, gateways.gateway_id, gateways.gateway_eui, gateways.name, "+
			"gateways.last_heard, gateways.latitude, gateways.longitude, gateways.altitude, gateways.attributes, "+
			"gateway_bounding_boxes.north, gateway_bounding_boxes.south, gateway_bounding_boxes.west, gateway_bounding_boxes.east").
		Joins("LEFT JOIN gateway_bounding_boxes on gateways.gateway_id = gateway_bounding_boxes.gateway_id "+
			"and gateways.network_id = gateway_bounding_boxes.network_id").
		Where("gateways.network_id = ? AND gateways.last_heard > NOW() - INTERVAL '5 DAY'", networkId).
		Find(&gateways)

	// Store in cache
	networkOnlineGatewaysCache.Set(networkId, gateways, cache.DefaultExpiration)

	return gateways
}

func GetOnlineGatewaysForNetworkInBbox(networkId string, west float64, east float64, north float64, south float64) []Gateway {

	var gateways []Gateway
	Db.Where("network_id = ? AND last_heard > NOW() - INTERVAL '5 DAY'", networkId).
		Where("latitude >= ? AND latitude <= ? AND longitude >= ? AND longitude <= ?", south, north, west, east).
		Where("NOT (latitude = 0 AND longitude = 0)").
		Find(&gateways)

	return gateways
}

func GetGateway(indexer GatewayIndexer) (Gateway, error) {
	var gatewayDb Gateway

	var gatewayIndex = indexer.NetworkId + "/" + indexer.GatewayId
	if cacheGateway, ok := gatewayDbCache.Get(gatewayIndex); ok {
		return cacheGateway.(Gateway), nil
	} else {
		gatewayDb = Gateway{NetworkId: indexer.NetworkId, GatewayId: indexer.GatewayId}
		//log.Println("Gateway from DB")
		err := Db.First(&gatewayDb, &gatewayDb).Error
		if err != nil {
			return gatewayDb, err
		}
		if gatewayDb.ID != 0 {
			gatewayDbCache.Set(gatewayIndex, gatewayDb, cache.DefaultExpiration)
		}
	}
	return gatewayDb, nil
}

func GetOrCreateGateway(indexer GatewayIndexer) (Gateway, error) {
	var gatewayDb Gateway

	var gatewayIndex = indexer.NetworkId + "/" + indexer.GatewayId
	if cacheGateway, ok := gatewayDbCache.Get(gatewayIndex); ok {
		return cacheGateway.(Gateway), nil
	} else {
		gatewayDb = Gateway{NetworkId: indexer.NetworkId, GatewayId: indexer.GatewayId}
		//log.Println("Gateway from DB")
		err := Db.FirstOrCreate(&gatewayDb, &gatewayDb).Error
		if err != nil {
			return gatewayDb, err
		}
		if gatewayDb.ID != 0 {
			gatewayDbCache.Set(gatewayIndex, gatewayDb, cache.DefaultExpiration)
		}
	}
	return gatewayDb, nil
}

func SaveGateway(gateway *Gateway) {
	Db.Save(gateway)
}

func GetGatewayLastMovedTime(networkId string, gatewayId string) time.Time {
	var movedTime time.Time
	lastMovedQuery := `
SELECT max(installed_at) FROM gateway_locations
WHERE network_id = ?
AND gateway_id = ?`
	timeRow := Db.Raw(lastMovedQuery, networkId, gatewayId).Row()
	timeRow.Scan(&movedTime)
	return movedTime
}

func GetGatewayLastMove(networkId string, gatewayId string) GatewayLocation {
	var gatewayMove GatewayLocation
	gatewayMove.NetworkId = networkId
	gatewayMove.GatewayId = gatewayId
	Db.Order("installed_at desc").First(&gatewayMove, &gatewayMove)
	return gatewayMove
}

func GetAllOldNamingTtnV2Antennas() []Antenna {
	var antennas []Antenna
	Db.Where("network_id LIKE 'NS_TTN_V2://%' OR network_id LIKE 'NS_TTS_V3://ttnv2@000013'").Find(&antennas)
	return antennas
}

func FindAntenna(networkId string, gatewayId string, antennaIndex uint8) Antenna {
	var antenna Antenna
	antennaIndexer := AntennaIndexer{NetworkId: networkId, GatewayId: gatewayId, AntennaIndex: antennaIndex}

	i, ok := antennaDbCache.Load(antennaIndexer)
	if ok {
		antenna = i.(Antenna)
	} else {
		antenna = Antenna{NetworkId: networkId, GatewayId: gatewayId, AntennaIndex: antennaIndex}
		Db.FirstOrCreate(&antenna, &antenna)
	}
	return antenna
}

func GetAntennaForGateway(networkId string, gatewayId string) []Antenna {
	var antennas []Antenna
	Db.Where("network_id = ? and gateway_id = ?", networkId, gatewayId).Find(&antennas)
	return antennas
}

func GetAntennasForNetwork(networkId string) []Antenna {
	var antennas []Antenna
	Db.Where("network_id = ?", networkId).Find(&antennas)
	return antennas
}

func UpdatePacketsAntennaId(oldAntennaId uint, newAntennaId uint) {
	Db.Model(&Packet{}).Where("antenna_id = ?", oldAntennaId).Update("antenna_id", newAntennaId)
}

func GetDistinctGatewaysInLocations() []GatewayIndexer {
	var gateways []GatewayIndexer
	Db.Model(&GatewayLocation{}).Distinct("network_id", "gateway_id").Find(&gateways)
	return gateways
}

func GetGatewayLocations(networkId string, gatewayId string) []GatewayLocation {
	var locations []GatewayLocation
	Db.Where("network_id = ? and gateway_id = ?", networkId, gatewayId).Order("installed_at asc").Find(&locations)
	return locations
}

func DeleteGatewayLocations(networkId string, gatewayId string) {
	Db.Where("network_id = ? and gateway_id = ?", networkId, gatewayId).Delete(GatewayLocation{})
}

func InsertGatewayLocations(locations []GatewayLocation) {
	Db.Create(&locations)
}

func GatewayInsertNewLocation(gateway types.TtnMapperGateway, installedAt time.Time) {
	newLocation := GatewayLocation{
		NetworkId:   gateway.NetworkId,
		GatewayId:   gateway.GatewayId,
		InstalledAt: installedAt,
		Latitude:    gateway.Latitude,
		Longitude:   gateway.Longitude,
		Altitude:    gateway.Altitude,
	}
	Db.Create(&newLocation)
}

// TODO cache this in memory for a certain period of time
func GatewayCoordinatesForced(gateway types.TtnMapperGateway) (bool, GatewayLocationForce) {
	forcedCoords := GatewayLocationForce{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId}
	Db.First(&forcedCoords, &forcedCoords)
	if forcedCoords.ID != 0 {
		return true, forcedCoords
	} else {
		return false, forcedCoords
	}
}

func GetOldMappedHeliumAntennas() []Antenna {
	var antennas []Antenna // must have an antenna to have been mapped
	Db.Where("network_id = 'NS_HELIUM://000024' AND gateway_id ~ '^[a-z]*-[a-z]*-[a-z]*$'").Find(&antennas)
	return antennas
}

func GetNewHeliumAntennaForOldAntenna(oldAntenna Antenna) Antenna {
	var gateway Gateway
	gateway.NetworkId = "NS_HELIUM://000024"
	gateway.Name = &oldAntenna.GatewayId // three word name moved from id field to name field
	Db.Find(&gateway, &gateway)

	var antenna Antenna
	antenna.NetworkId = oldAntenna.NetworkId
	antenna.GatewayId = gateway.GatewayId
	antenna.AntennaIndex = oldAntenna.AntennaIndex
	Db.FirstOrCreate(&antenna, &antenna)
	return antenna
}

func GetGwsForNetworkInBb(networkId string, north float64, east float64, south float64, west float64) []Gateway {
	var gateways []Gateway
	Db.Where("network_id = ? AND latitude >= ? AND latitude <= ? AND longitude >= ? AND longitude <= ?", networkId, south, north, west, east).Find(&gateways)
	return gateways
}

func GetGwsInBb(north float64, east float64, south float64, west float64) []Gateway {
	var gateways []Gateway
	Db.Where("latitude >= ? AND latitude <= ? AND longitude >= ? AND longitude <= ?", south, north, west, east).Find(&gateways)
	return gateways
}
