package radar_beam

import (
	"github.com/kelseyhightower/envconfig"
	"log"
	"testing"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

type Configuration struct {
	PostgresHost          string `envconfig:"POSTGRES_HOST"`
	PostgresPort          string `envconfig:"POSTGRES_PORT"`
	PostgresUser          string `envconfig:"POSTGRES_USER"`
	PostgresPassword      string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDatabase      string `envconfig:"POSTGRES_DATABASE"`
	PostgresDebugLog      bool   `envconfig:"POSTGRES_DEBUG_LOG"`
	PostgresInsertThreads int    `envconfig:"POSTGRES_INSERT_THREADS"`
}

var myConfiguration = Configuration{
	PostgresHost:          "localhost",
	PostgresPort:          "5432",
	PostgresUser:          "username",
	PostgresPassword:      "password",
	PostgresDatabase:      "database",
	PostgresDebugLog:      false,
	PostgresInsertThreads: 1,
}

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

func TestAggregateMovedGateway(t *testing.T) {
	IniDb()

	//movedGateway := types.TtnMapperGatewayMoved{
	//	NetworkId:    "NS_TTS_V3://ttn@000013",
	//	GatewayId:    "eui-000080029c09dd87",
	//}
	movedGateway := types.TtnMapperGatewayMoved{
		NetworkId: "NS_TTS_V3://ttn@000013",
		GatewayId: "eui-60c5a8fffe71a964",
	}
	AggregateMovedGateway(movedGateway)
	//gateway := types.Gateway{NetworkId: "thethingsnetwork.org", GatewayId: "eui-58a0cbfffe8023e7"}
	//ReprocessSingleGateway(gateway)
}
