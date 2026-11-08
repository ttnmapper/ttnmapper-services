package database

import (
	"log"
	"ttnmapper-services/pkg/config"
)

type Configuration struct {
	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     string `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`
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
	err := config.Environment(&myConfiguration)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Init database")
	databaseContext := Context{
		Host:     myConfiguration.PostgresHost,
		Port:     myConfiguration.PostgresPort,
		User:     myConfiguration.PostgresUser,
		Database: myConfiguration.PostgresDatabase,
		Password: myConfiguration.PostgresPassword,
		DebugLog: myConfiguration.PostgresDebugLog,
	}
	databaseContext.Init()
}
