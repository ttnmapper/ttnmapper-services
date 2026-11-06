package main

import (
	"encoding/json"
	"log"
	"math"
	"time"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
	"ttnmapper-services/pkg/utils"

	"github.com/streadway/amqp"
	"github.com/umahmood/haversine"
)

func processRawPackets() {
	// Wait for a message and insert it into Postgres
	for d := range rawPacketsChannel {

		// The message from amqp is a json string. Unmarshal to ttnmapper uplink struct
		var message types.TtnMapperUplinkMessage
		if err := json.Unmarshal(d.Body, &message); err != nil {
			log.Print("AMQP " + err.Error())
			continue
		}

		// Iterate gateways in packet
		for _, gateway := range message.Gateways {
			updateTime := time.Unix(0, message.Time)
			log.Print("AMQP ", "", "\t", gateway.GatewayId+"\t", updateTime)

			// We use the "last heard" on the network
			gateway.Time = message.Time

			// Ignore packet broker
			if gateway.GatewayId == "packetbroker" {
				continue
			}

			UpdateGateway(gateway)
		}
	}
}

func UpdateGateway(gateway types.TtnMapperGateway) {
	gatewayStart := time.Now()

	// Count number of gateways we processed
	processedGateways.Inc()

	// Last heard time
	seconds := gateway.Time / 1000000000
	nanos := gateway.Time % 1000000000
	lastHeard := time.Unix(seconds, nanos)

	// A gateway's last heard can not be in the future
	if lastHeard.After(time.Now()) {
		lastHeard = time.Now()
	}

	// Find the database IDs for this gateway and it's antennas
	gatewayDb, err := database.GetOrCreateGateway(database.GatewayIndexer{
		NetworkId: gateway.NetworkId,
		GatewayId: gateway.GatewayId,
	})
	if err != nil {
		utils.FailOnError(err, "Can't find gateway in DB")
	}

	// Check if our lastHeard time is newer that the lastHeard in the database
	// If it's not we are using old cached data which should be ignored
	if lastHeard.Before(gatewayDb.LastHeard) {
		log.Println("\tStatus record stale")
		return
	}

	// Check if the coordinates should be forced to a specific location
	gatewayLocationForced := false
	if isForced, forcedCoordinates := database.GatewayCoordinatesForced(gateway); isForced == true {
		log.Println("\tGateway coordinates forced")
		gatewayLocationForced = true
		gateway.Latitude = forcedCoordinates.Latitude
		gateway.Longitude = forcedCoordinates.Longitude
		gateway.Altitude = forcedCoordinates.Altitude
	}

	// Check if the provided coordinates are valid
	if valid, reason := CoordinatesValid(gateway); !valid {
		log.Println("\tGateway coordinates invalid. " + reason)
		log.Println("\tForcing to 0,0.")
		gateway.Latitude = 0.0
		gateway.Longitude = 0.0
		gateway.Altitude = 0.0
	}

	// Check if gateway moved. If the location is not provided, do not move, unless it's forced to 0,0
	if gatewayLocationForced || (gateway.Latitude != 0.0 && gateway.Longitude != 0.0) {
		oldLocation := haversine.Coord{Lat: gatewayDb.Latitude, Lon: gatewayDb.Longitude}
		newLocation := haversine.Coord{Lat: gateway.Latitude, Lon: gateway.Longitude}
		_, km := haversine.Distance(oldLocation, newLocation)

		// Did it move more than 100m
		if km > 0.1 {
			movedGateways.Inc()
			log.Println("\tGATEWAY MOVED")
			log.Println("\t", gatewayDb.Latitude, gatewayDb.Longitude)
			log.Println("\t", gateway.Latitude, gateway.Longitude)
			log.Println("\t", km, "km")

			movedGateway := types.TtnMapperGatewayMoved{}
			movedGateway.NetworkId = gateway.NetworkId
			movedGateway.GatewayId = gateway.GatewayId

			movedGateway.Time = lastHeard.UnixNano() // the time the move was detected, but should not be used

			movedGateway.LatitudeOld = gatewayDb.Latitude
			movedGateway.LongitudeOld = gatewayDb.Longitude
			movedGateway.AltitudeOld = gatewayDb.Altitude

			movedGateway.LatitudeNew = gateway.Latitude
			movedGateway.LongitudeNew = gateway.Longitude
			movedGateway.AltitudeNew = gateway.Altitude

			database.GatewayInsertNewLocation(gateway, lastHeard)
			publishMovedGateway(movedGateway)
		}
	}

	// Cache and gateway table should reflect last location
	if !gatewayLocationForced {
		if gateway.Latitude == 0 && gateway.Longitude == 0 {
			// Previous location was valid, do not update to invalid
		} else {
			// Update to latest unless latest is invalid
			gatewayDb.Latitude = gateway.Latitude
			gatewayDb.Longitude = gateway.Longitude
		}
		if gateway.Altitude == 0 {
			// Use previous altitude
		} else {
			gatewayDb.Altitude = gateway.Altitude
		}
	} else {
		// Forced, so always update
		gatewayDb.Latitude = gateway.Latitude
		gatewayDb.Longitude = gateway.Longitude
		gatewayDb.Altitude = gateway.Altitude
	}

	// Update gateway in db with fields that are set
	gatewayDb.LastHeard = lastHeard
	if gateway.GatewayEui != "" {
		eui := gateway.GatewayEui
		gatewayDb.GatewayEui = &eui
	}
	if gateway.Name != "" {
		name := gateway.Name
		gatewayDb.Name = &name
	}

	if gateway.LocationAccuracy != 0 {
		accuracy := gateway.LocationAccuracy
		gatewayDb.LocationAccuracy = &accuracy
	}
	if gateway.LocationSource != "" {
		source := gateway.LocationSource
		gatewayDb.LocationSource = &source
	}

	// Old attributes unmarshal
	attributes := make(map[string]interface{}, 0)
	err = json.Unmarshal(gatewayDb.Attributes, &attributes)
	if err != nil {
		//log.Println(err.Error())
		// This means the database does not contain any data
	}
	// Add new attributes one by one
	for k, v := range gateway.Attributes {
		attributes[k] = v
	}
	// Marshal new attributes
	newAttributes, err := json.Marshal(attributes)
	if err != nil {
		log.Println(err.Error())
	}
	gatewayDb.Attributes = newAttributes

	// Update in database
	database.SaveGateway(&gatewayDb)

	log.Println("\tUpdated")
	updatedGateways.Inc()

	// Prometheus stats
	gatewayElapsed := time.Since(gatewayStart)
	insertDuration.Observe(float64(gatewayElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
}

func CoordinatesValid(gateway types.TtnMapperGateway) (valid bool, reason string) {

	if math.Abs(gateway.Latitude) < 1 && math.Abs(gateway.Longitude) < 1 {
		return false, "Null island"
	}
	if math.Abs(gateway.Latitude) > 90 {
		return false, "Latitude out of bounds"
	}
	if math.Abs(gateway.Longitude) > 180 {
		return false, "Longitude out of bounds"
	}

	// Default SCG location
	if gateway.Latitude == 52.0 && gateway.Longitude == 6.0 {
		return false, "Single channel gateway default coordinates"
	}

	// Default Lorier LR2 location
	if gateway.Latitude == 10.0 && gateway.Longitude == 20.0 {
		return false, "Lorier LR2 default coordinates"
	}

	// Ukrainian hack
	if gateway.Latitude == 50.008724 && gateway.Longitude == 36.215805 {
		return false, "Ukrainian hack coordinates"
	}

	// Shenzhen factory, reusing EUIs and moving valid gateways
	if gateway.Latitude > 22.69 && gateway.Latitude < 22.71 && gateway.Longitude > 114.2300000 && gateway.Longitude < 114.25 {
		return false, "Shenzhen factory coordinates"
	}

	return true, ""
}

func publishMovedGateway(gateway types.TtnMapperGatewayMoved) {

	gatewayMovedAmqpConn, err := amqp.Dial("amqp://" + myConfiguration.AmqpUser + ":" + myConfiguration.AmqpPassword + "@" + myConfiguration.AmqpHost + ":" + myConfiguration.AmqpPort + "/")
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer gatewayMovedAmqpConn.Close()

	gatewayMovedAmqpChannel, err := gatewayMovedAmqpConn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer gatewayMovedAmqpChannel.Close()

	err = gatewayMovedAmqpChannel.ExchangeDeclare(
		myConfiguration.AmqpExchangeGatewayMoved, // name
		"fanout",                                 // type
		true,                                     // durable
		false,                                    // auto-deleted
		false,                                    // internal
		false,                                    // no-wait
		nil,                                      // arguments
	)
	utils.FailOnError(err, "Failed to declare an exchange")

	gatewayJsonData, err := json.Marshal(gateway)
	if err != nil {
		log.Println("\t\tCan't marshal gateway to json")
		return
	}

	err = gatewayMovedAmqpChannel.Publish(
		myConfiguration.AmqpExchangeGatewayMoved, // exchange
		"",                                       // routing key
		false,                                    // mandatory
		false,                                    // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        gatewayJsonData,
		})
	utils.FailOnError(err, "Failed to publish a message")

	log.Printf("\tPublished to AMQP exchange")
	//log.Printf("\t%s", gatewayJsonData)
}
