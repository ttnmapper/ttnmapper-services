package main

import (
	"log"
	"ttnmapper-services/pkg/database"

	"github.com/kelseyhightower/envconfig"
	"github.com/umahmood/haversine"
)

/*
Ran 2021-09-19 12:30 UTC
*/

type Configuration struct {
	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresPort     string `envconfig:"POSTGRES_PORT"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDatabase string `envconfig:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `envconfig:"POSTGRES_DEBUG_LOG"`

	PrometheusPort string `envconfig:"PROMETHEUS_PORT"`
}

var myConfiguration = Configuration{

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	PrometheusPort: "9100",
}

func main() {
	err := envconfig.Process("", &myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Init Postgres database")
	databaseContext := database.Context{
		Host:     myConfiguration.PostgresHost,
		Port:     myConfiguration.PostgresPort,
		User:     myConfiguration.PostgresUser,
		Database: myConfiguration.PostgresDatabase,
		Password: myConfiguration.PostgresPassword,
		DebugLog: myConfiguration.PostgresDebugLog,
	}
	databaseContext.Init()

	gateways := database.GetDistinctGatewaysInLocations()
	for i, gateway := range gateways {
		log.Printf("%d/%d\t%s - %s", i, len(gateways), gateway.NetworkId, gateway.GatewayId)
		ProcessGateway(gateway.NetworkId, gateway.GatewayId)
	}
}

func ProcessGateway(networkId string, gatewayId string) {
	locations := database.GetGatewayLocations(networkId, gatewayId)
	if len(locations) == 1 {
		log.Println("only one location")
		return
	}
	newLocations := FilterLocations(locations)
	if len(locations) != len(newLocations) {
		//log.Println(locations)
		//log.Println(newLocations)
		log.Printf("old = %d\tnew = %d", len(locations), len(newLocations))
		database.DeleteGatewayLocations(networkId, gatewayId)
		database.InsertGatewayLocations(newLocations)
	} else {
		log.Println("not changed")
	}
}

func FilterLocations(input []database.GatewayLocation) (output []database.GatewayLocation) {
	output = append(output, input[0])

	for _, location := range input {
		moved := false
		// Check if we move more than 100m since the previous location
		lastLocation := output[len(output)-1]
		oldLocation := haversine.Coord{Lat: lastLocation.Latitude, Lon: lastLocation.Longitude}
		newLocation := haversine.Coord{Lat: location.Latitude, Lon: location.Longitude}
		_, km := haversine.Distance(oldLocation, newLocation)

		// Did it move more than 100m
		if km > 0.1 {
			moved = true
		}

		// Some gateways oscillate between two location. Therefore check one more back
		if len(output)-2 >= 0 { // only go back two places if there is more than two elements in array
			lastLocation := output[len(output)-2]
			oldLocation := haversine.Coord{Lat: lastLocation.Latitude, Lon: lastLocation.Longitude}
			newLocation := haversine.Coord{Lat: location.Latitude, Lon: location.Longitude}
			_, km := haversine.Distance(oldLocation, newLocation)

			// We did not move since the 2nd last location
			if km == 0 {
				moved = false
			}
		}

		if moved {
			output = append(output, location)
		}
	}

	return output
}
