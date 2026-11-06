package main

import (
	"log"
	"ttnmapper-services/pkg/database"

	"github.com/kelseyhightower/envconfig"
	apt "github.com/ormembaar/angry-purple-tiger"
)

/*
Migration ran 2022-01-08 18:30 SAST
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

	antennas := database.GetOldMappedHeliumAntennas()
	for i, oldAntenna := range antennas {
		log.Printf("%d/%d %s", i, len(antennas), oldAntenna.GatewayId)
		newAntenna := database.GetNewHeliumAntennaForOldAntenna(oldAntenna)
		if oldAntenna.GatewayId != apt.Sum([]byte(newAntenna.GatewayId)) {
			log.Println("Mismatch", oldAntenna.GatewayId, newAntenna.GatewayId, apt.Sum([]byte(newAntenna.GatewayId)))
			continue
		}

		if newAntenna.ID == 0 {
			log.Println("Antenna does not exist")
			continue
		}

		database.UpdatePacketsAntennaId(oldAntenna.ID, newAntenna.ID)

		// update first heard time at current location
		oldMoved := database.GetGatewayLastMove(oldAntenna.NetworkId, oldAntenna.GatewayId)
		newMoved := database.GetGatewayLastMove(newAntenna.NetworkId, newAntenna.GatewayId)

		if oldMoved.ID == 0 {
			log.Println("No old move found")
			continue
		}
		if newMoved.ID == 0 {
			log.Println("No new move found")
			continue
		}

		//log.Println(oldMoved.InstalledAt, newMoved.InstalledAt)
		newMoved.InstalledAt = oldMoved.InstalledAt
		database.Db.Save(&newMoved)
	}
}
