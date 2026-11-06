package main

/*
This migration ran completely on 2021-09-04

ssh -L 3306:127.0.0.1:3306 root@ttnmapper.org
*/

import (
	"log"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/oldstack"

	"github.com/kelseyhightower/envconfig"
)

type Configuration struct {
	MysqlHost     string `envconfig:"MYSQL_HOST"`
	MysqlPort     string `envconfig:"MYSQL_PORT"`
	MysqlUser     string `envconfig:"MYSQL_USER"`
	MysqlPassword string `envconfig:"MYSQL_PASSWORD"`
	MysqlDatabase string `envconfig:"MYSQL_DATABASE"`
	MysqlDebugLog bool   `envconfig:"MYSQL_DEBUG_LOG"`

	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresPort     string `envconfig:"POSTGRES_PORT"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDatabase string `envconfig:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `envconfig:"POSTGRES_DEBUG_LOG"`

	PrometheusPort string `envconfig:"PROMETHEUS_PORT"`
}

var myConfiguration = Configuration{
	MysqlHost:     "localhost",
	MysqlPort:     "3306",
	MysqlUser:     "username",
	MysqlPassword: "password",
	MysqlDatabase: "database",
	MysqlDebugLog: false,

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	PrometheusPort: "9100",
}

func main() {
	return // prevent accidental rerun
	err := envconfig.Process("", &myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Init Mysql database")
	mysqlContext := oldstack.DatabaseContext{
		Host:     myConfiguration.MysqlHost,
		Port:     myConfiguration.MysqlPort,
		User:     myConfiguration.MysqlUser,
		Database: myConfiguration.MysqlDatabase,
		Password: myConfiguration.MysqlPassword,
		DebugLog: myConfiguration.MysqlDebugLog,
	}
	mysqlContext.Init()

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

	// Get all gateway moves from mysql
	var gatewayMoves []oldstack.GatewayUpdate
	offset := 0
	for {
		gatewayMoves = oldstack.GetGatewayUpdates(500, offset)
		if len(gatewayMoves) == 0 {
			break
		}

		gatewayLocations := make([]database.GatewayLocation, 0)
		for _, move := range gatewayMoves {
			offset++
			gatewayLocation := MysqlGatewayUpdateToPostgresGatewayLocation(move)
			//log.Println(offset, gatewayLocation.NetworkId, gatewayLocation.GatewayId, gatewayLocation.InstalledAt)
			gatewayLocations = append(gatewayLocations, gatewayLocation)
		}

		err := database.InsertGatewayLocationsBatch(gatewayLocations)
		if err != nil {
			log.Fatalf(err.Error())
		}
		log.Println("Inserted", offset)
	}

	// Convert to postgres gateway locations

}

func MysqlGatewayToPostgresGateway() {

}

func MysqlGatewayUpdateToPostgresGatewayLocation(mysqlGwUpdate oldstack.GatewayUpdate) database.GatewayLocation {
	var gatewayLocation database.GatewayLocation

	networkId, gatewayId, _ := oldstack.GwaddrToNetIdEui(mysqlGwUpdate.GatewayAddress)
	gatewayLocation.NetworkId = networkId
	gatewayLocation.GatewayId = gatewayId
	gatewayLocation.InstalledAt = mysqlGwUpdate.InstalledAt
	gatewayLocation.Latitude = mysqlGwUpdate.Latitude
	gatewayLocation.Longitude = mysqlGwUpdate.Longitude
	gatewayLocation.Altitude = int32(mysqlGwUpdate.Altitude)

	return gatewayLocation
}
