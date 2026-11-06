package main

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/database"

	"github.com/kelseyhightower/envconfig"
)

func Init() {
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
}

/*
2021/09/19 12:25:44 701 NS_TTS_V3://ttn@000013 313330371c006d00 - conflict between gps and static locations >100m apart
2021/09/19 12:25:44 Locations 322
2021/09/19 12:25:59 788 NS_TTS_V3://ttn@000013 343632383a004100
2021/09/19 12:25:59 Locations 496

network_id='NS_TTS_V3://ttn@000013' and gateway_id='atomagateway1' - 7158
*/

func TestProcessGateway(t *testing.T) {
	Init()
	ProcessGateway("NS_TTS_V3://ttn@000013", "atomagateway1")
}
