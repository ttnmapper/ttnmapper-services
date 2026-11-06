package grid_cell

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
		NetworkId: "thethingsnetwork.org",
		GatewayId: "eui-58a0cbfffe8023e7",
	}
	AggregateMovedGateway(movedGateway)
	//gateway := types.Gateway{NetworkId: "thethingsnetwork.org", GatewayId: "eui-58a0cbfffe8023e7"}
	//ReprocessSingleGateway(gateway)
}

func TestReprocessSpiess(t *testing.T) {
	IniDb()

	// Get all gateways heard by device ID
	//	query := `
	//SELECT DISTINCT(antenna_id) FROM packets p
	//JOIN devices d on d.id = p.device_id
	//WHERE d.dev_id = 't-beam-tracker'
	//AND d.app_id = 'ttn-tracker-sensorsiot'`

	//	rows, _ := main.db.Raw(query).Rows()
	//	for rows.Next() {
	//		var antennaId uint
	//		rows.Scan(&antennaId)
	//
	//		// Find the antenna IDs for the moved gateway
	//		var antenna database.Antenna
	//		main.db.First(&antenna, antennaId)
	//
	//		var movedTime time.Time
	//		lastMovedQuery := `
	//SELECT max(installed_at) FROM gateway_locations
	//WHERE network_id = ?
	//AND gateway_id = ?`
	//		timeRow := main.db.Raw(lastMovedQuery, antenna.NetworkId, antenna.GatewayId).Row()
	//		timeRow.Scan(&movedTime)
	//
	//		log.Println(antenna.GatewayId, movedTime)
	//		ReprocessAntenna(antenna, movedTime)
	//		break
	//	}
	//	rows.Close()
	//	main.db.Close()
}

func TestReprocessHelium(t *testing.T) {
	IniDb()

	antennas := database.GetAntennasForNetwork("NS_HELIUM://000024")

	for _, antenna := range antennas {
		movedTime := database.GetGatewayLastMovedTime(antenna.NetworkId, antenna.GatewayId)
		log.Println(antenna.GatewayId, movedTime)
		ReprocessAntenna(antenna, movedTime)
	}
}
