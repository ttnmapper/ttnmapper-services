package main

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/utils"

	"github.com/kelseyhightower/envconfig"
)

func initTests() {
	err := envconfig.Process("", &myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("[Configuration]\n%s\n", utils.PrettyPrint(myConfiguration)) // output: [UserA, UserB]

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

//func TestFectNocStatuses(t *testing.T) {
//	initTests()
//	fetchNocStatuses()
//}

//func TestFetchWebStatuses(t *testing.T) {
//	initTests()
//	fetchWebStatuses()
//}

func TestFetchPacketBrokerStatuses(t *testing.T) {
	initTests()
	fetchPacketBrokerStatuses()
}

func TestFetchHeliumStatuses(t *testing.T) {
	initTests()
	fetchHeliumStatuses()
}

func TestFetchHeliumSnapshot(t *testing.T) {
	initTests()
	fetchHeliumSnapshot()
}

func TestFetchAllTtsStatuses(t *testing.T) {
	initTests()
	fetchAllTtsStatuses()
}

func TestFetchTtsStatuses(t *testing.T) {
	initTests()
	var network = database.TtsV3FetchStatus{
		ID:       0,
		TenantId: "handson-systems",
		ApiKey:   "",
	}
	fetchTtsStatuses(network)
}

func TestFetchPbRoutingPolicies(t *testing.T) {
	initTests()
	FetchPbRoutingPolicies()
}
