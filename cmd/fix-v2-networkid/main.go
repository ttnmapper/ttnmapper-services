package main

import (
	"log"
	"ttnmapper-services/pkg/database"

	"github.com/kelseyhightower/envconfig"
)

/*
Migration completed 2021-09-17

DELETE FROM antennas WHERE network_id LIKE 'NS_TTN_V2://%'

DELETE FROM gateways WHERE network_id LIKE 'NS_TTN_V2://%'

DELETE FROM gateway_locations WHERE network_id LIKE 'NS_TTN_V2://%'

DELETE FROM gateway_bounding_boxes WHERE network_id LIKE 'NS_TTN_V2://%'

DELETE FROM gateway_location_forces WHERE network_id LIKE 'NS_TTN_V2://%'

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

	antennas := database.GetAllOldNamingTtnV2Antennas()

	for i, oldAntenna := range antennas {
		newAntenna := database.FindAntenna("thethingsnetwork.org", oldAntenna.GatewayId, oldAntenna.AntennaIndex)
		log.Printf("%d/%d %s:\t%d ==> %d", i, len(antennas), oldAntenna.GatewayId, oldAntenna.ID, newAntenna.ID)

		database.UpdatePacketsAntennaId(oldAntenna.ID, newAntenna.ID)
	}
}
