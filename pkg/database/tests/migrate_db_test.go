package tests

import (
	"encoding/json"
	"github.com/kelseyhightower/envconfig"
	"log"
	"testing"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/utils"
)

type Configuration struct {
	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresPort     string `envconfig:"POSTGRES_PORT"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDatabase string `envconfig:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `envconfig:"POSTGRES_DEBUG_LOG"`
}

var myConfiguration = Configuration{
	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: true,
}

func initDb() {
	err := envconfig.Process("", &myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Init database")
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

	initDb()

	log.Println("Performing auto migrate")
	if err := database.Db.AutoMigrate(
		//&database.Device{},
		//&database.Frequency{},
		//&database.DataRate{},
		//&database.CodingRate{},
		//&database.AccuracySource{},
		//&database.Experiment{},
		//&database.User{},
		//&database.UserAgent{},
		//&database.Antenna{},
		//&database.FineTimestampKeyID{},
		//&database.Packet{},
		//&database.Gateway{},
		//&database.TestTable{},
		//&database.PacketBrokerRoutingPolicy{},
		&database.GatewayBoundingBox{},
	); err != nil {
		log.Println("Unable autoMigrateDB - ", err.Error())
	}
}

func TestInsertTestTable(t *testing.T) {
	initDb()

	attributes := make(map[string]interface{}, 0)
	attributes["name"] = "test entry"

	marshalled, err := json.Marshal(attributes)
	if err != nil {
		log.Println(err.Error())
	}

	testEntry := database.TestTable{
		ID:         0,
		Attributes: marshalled,
	}

	database.Db.Create(&testEntry).Commit()
}

func TestSelectTestTable(t *testing.T) {
	initDb()

	var testEntries []database.TestTable
	database.Db.Find(&testEntries)

	for _, entry := range testEntries {
		attributes := map[string]interface{}{}
		err := json.Unmarshal(entry.Attributes, &attributes)
		if err != nil {
			log.Println(err.Error())
		}
		log.Println(attributes["name"])
	}
}

func TestGetGatewaysByNameOrId(t *testing.T) {
	initDb()

	gateways := database.GetGatewaysByNameOrId("nice-silver-shell")

	log.Println(utils.PrettyPrint(gateways))
}

func TestGetPeeredNetworks(t *testing.T) {
	initDb()

	peeredNetworks := database.GetPeeredNetworks("NS_TTS_V3://ttn@000013")
	log.Println(utils.PrettyPrint(peeredNetworks))
}
