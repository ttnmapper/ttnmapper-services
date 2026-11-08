package gatewaychannels

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"time"
	"ttnmapper-services/internal/gatewaychannels/types"
)

func processRawPackets() {
	processPackets(rawPacketsChannel, updateGateway)
}
func processPackets(packets <-chan amqp.Delivery, update func(types.TtnMapperUplinkMessage, types.TtnMapperGateway)) {
	for d := range packets {

		// The message form amqp is a json string. Unmarshal to ttnmapper uplink struct
		var message types.TtnMapperUplinkMessage
		if err := json.Unmarshal(d.Body, &message); err != nil {
			log.Print("AMQP " + err.Error())
			continue
		}

		// Iterate gateways. We store it flat in the database
		for _, gateway := range message.Gateways {
			updateTime := time.Unix(0, message.Time)
			log.Print("AMQP ", "", "\t", gateway.GatewayId+"\t", updateTime)
			gateway.Time = message.Time

			// Ignore locations obtained from live data
			gateway.Latitude = 0
			gateway.Longitude = 0
			gateway.Altitude = 0

			update(message, gateway)
		}
	}
}

func updateGateway(message types.TtnMapperUplinkMessage, gateway types.TtnMapperGateway) {
	gatewayStart := time.Now()

	// Count number of gateways we processed
	processedGateways.Inc()

	if err := associateGateway(message, gateway, gatewayStore{getAntennaDb, getFrequencyDb, setAntennaFrequencyDb}); err != nil {
		failOnError(err, "Can't update antenna frequency in DB")
	}

	// Prometheus stats
	gatewayElapsed := time.Since(gatewayStart)
	insertDuration.Observe(float64(gatewayElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
}

type gatewayStore struct {
	antenna   func(types.TtnMapperGateway) (types.Gateway, error)
	frequency func(uint64) (types.Frequency, error)
	associate func(uint, uint) error
}

func associateGateway(message types.TtnMapperUplinkMessage, gateway types.TtnMapperGateway, store gatewayStore) error {
	antenna, err := store.antenna(gateway)
	if err != nil {
		return fmt.Errorf("find antenna: %w", err)
	}
	frequency, err := store.frequency(message.Frequency)
	if err != nil {
		return fmt.Errorf("find frequency: %w", err)
	}
	if err := store.associate(antenna.ID, frequency.ID); err != nil {
		return fmt.Errorf("associate antenna frequency: %w", err)
	}
	return nil
}

/*
Takes a TTN Mapper Gateway and search for it in the database and return the database entry id
*/
func getAntennaDb(gateway types.TtnMapperGateway) (types.Gateway, error) {
	return cachedAntenna(gateway, func(gw types.TtnMapperGateway) (types.Gateway, error) {
		entry := types.Gateway{NetworkId: gw.NetworkId, GatewayId: gw.GatewayId}
		err := db.FirstOrCreate(&entry, &entry).Error
		return entry, err
	})
}
func cachedAntenna(gateway types.TtnMapperGateway, lookup func(types.TtnMapperGateway) (types.Gateway, error)) (types.Gateway, error) {

	antennaIndexer := types.AntennaIndexer{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId, AntennaIndex: gateway.AntennaIndex}
	i, ok := antennaCache.Load(antennaIndexer)
	if ok {
		gatewayDb := i.(types.Gateway)
		return gatewayDb, nil

	} else {
		antennaDb, err := lookup(gateway)
		if err != nil {
			return antennaDb, err
		}

		antennaCache.Store(antennaIndexer, antennaDb)
		return antennaDb, nil
	}
}

func getFrequencyDb(frequency uint64) (types.Frequency, error) {
	return cachedFrequency(frequency, func(hz uint64) (types.Frequency, error) {
		entry := types.Frequency{Herz: hz}
		err := db.FirstOrCreate(&entry, &entry).Error
		return entry, err
	})
}
func cachedFrequency(frequency uint64, lookup func(uint64) (types.Frequency, error)) (types.Frequency, error) {
	i, ok := frequencyCache.Load(frequency)
	if ok {
		frequencyDb := i.(types.Frequency)
		return frequencyDb, nil

	} else {
		frequencyDb, err := lookup(frequency)
		if err != nil {
			return frequencyDb, err
		}
		frequencyCache.Store(frequency, frequencyDb)
		return frequencyDb, nil
	}
}

func setAntennaFrequencyDb(antennaID uint, frequencyID uint) error {

	//antennaFrequencyDb := types.AntennaFrequency{
	//	AntennaID:   antennaID,
	//	FrequencyID: frequencyID,
	//}

	/*
		UPDATE totals
		   SET count = count + 1
		WHERE name = 'bill';

		INSERT INTO customers (name, email)
		VALUES
			(
				'Microsoft',
				'hotline@microsoft.com'
			)
		ON CONFLICT (name)
		DO
				UPDATE
			  SET email = EXCLUDED.email || ';' || customers.email;

		db.Exec("UPDATE orders SET shipped_at=? WHERE id IN (?)", time.Now(), []int64{11,22,33})
	*/
	// Need to use a raw query because... ???
	query := "INSERT INTO antenna_frequencies (antenna_id, frequency_id, count, last_heard) " +
		"VALUES (?, ?, 1, NOW()) " +
		"ON CONFLICT (antenna_id, frequency_id) DO " +
		"UPDATE SET count = antenna_frequencies.count + 1, last_heard = NOW() " +
		"WHERE antenna_frequencies.antenna_id = ? AND antenna_frequencies.frequency_id = ? "

	return db.Exec(query, antennaID, frequencyID, antennaID, frequencyID).Error
}
