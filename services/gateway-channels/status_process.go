package main

import (
	"encoding/json"
	"log"
	"time"
	"ttnmapper-gateway-channels/types"
)

func processRawPackets() {
	// Wait for a message and insert it into Postgres
	for d := range rawPacketsChannel {

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

			updateGateway(message, gateway)
		}
	}
}

func updateGateway(message types.TtnMapperUplinkMessage, gateway types.TtnMapperGateway) {
	gatewayStart := time.Now()

	// Count number of gateways we processed
	processedGateways.Inc()

	// Find the database IDs for this gateway and it's antennas
	antennaDb, err := getAntennaDb(gateway)
	if err != nil {
		failOnError(err, "Can't find antenna in DB")
	}

	frequencyDb, err := getFrequencyDb(message.Frequency)
	if err != nil {
		failOnError(err, "Can't find antenna in DB")
	}

	setAntennaFrequencyDb(antennaDb.ID, frequencyDb.ID)

	// Prometheus stats
	gatewayElapsed := time.Since(gatewayStart)
	insertDuration.Observe(float64(gatewayElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
}

/*
Takes a TTN Mapper Gateway and search for it in the database and return the database entry id
*/
func getAntennaDb(gateway types.TtnMapperGateway) (types.Gateway, error) {

	antennaIndexer := types.AntennaIndexer{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId, AntennaIndex: gateway.AntennaIndex}
	i, ok := antennaCache.Load(antennaIndexer)
	if ok {
		gatewayDb := i.(types.Gateway)
		return gatewayDb, nil

	} else {
		antennaDb := types.Gateway{NetworkId: gateway.NetworkId, GatewayId: gateway.GatewayId}
		db.Where(&antennaDb).First(&antennaDb)
		if antennaDb.ID == 0 {
			err := db.FirstOrCreate(&antennaDb, &antennaDb).Error
			if err != nil {
				return antennaDb, err
			}
		}

		antennaCache.Store(antennaIndexer, antennaDb)
		return antennaDb, nil
	}
}

func getFrequencyDb(frequency uint64) (types.Frequency, error) {
	i, ok := frequencyCache.Load(frequency)
	if ok {
		frequencyDb := i.(types.Frequency)
		return frequencyDb, nil

	} else {
		frequencyDb := types.Frequency{
			Herz: frequency,
		}
		db.Where(&frequencyDb).First(&frequencyDb)
		if frequencyDb.ID == 0 {
			err := db.FirstOrCreate(&frequencyDb, &frequencyDb).Error
			if err != nil {
				return frequencyDb, err
			}
		}

		antennaCache.Store(frequency, frequencyDb)
		return frequencyDb, nil
	}
}

func setAntennaFrequencyDb(antennaID uint, frequencyID uint) {

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

	db.Exec(query, antennaID, frequencyID, antennaID, frequencyID)
}
