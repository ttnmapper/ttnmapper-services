package main

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/database"

	"github.com/kelseyhightower/envconfig"
)

func IniDb() {
	err := envconfig.Process("", &myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	databaseContext := database.Context{
		Host:     myConfiguration.PostgresHost,
		Port:     myConfiguration.PostgresPort,
		User:     myConfiguration.PostgresUser,
		Database: myConfiguration.PostgresDatabase,
		Password: myConfiguration.PostgresPassword,
		DebugLog: myConfiguration.PostgresDebugLog,
	}
	databaseContext.Init()
}

func TestMigrateDb(t *testing.T) {
	IniDb()

	//database.AutoMigrate(&database.GridCell{}, &database.RadarBeam{})
	database.AutoMigrate(&database.GatewayBoundingBox{})
}

func TestReprocessSingleGateway(t *testing.T) {
	IniDb()

	gateway := database.Gateway{
		NetworkId: "NS_TTS_V3://ttn@000013",
		GatewayId: "ttn-rhein-sieg-01",
	}
	ReprocessSingleGateway(gateway)
}
