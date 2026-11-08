package websiteapi

import (
	"log"
	"net/http"
	"time"
	"ttnmapper-services/pkg/database"

	"ttnmapper-services/pkg/config"
)

type Configuration struct {
	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     string `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`

	HttpListenAddress string `env:"HTTP_LISTEN_ADDRESS"`
}

var myConfiguration = Configuration{
	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	HttpListenAddress: ":8080",
}

func Run() {

	err := config.Environment(&myConfiguration)
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

	// Register routes
	router := Routes()

	// Start the http endpoint
	log.Println("Starting server on", myConfiguration.HttpListenAddress)
	routerWithTimeout := http.TimeoutHandler(router, time.Minute*1, "Handler Timeout!")
	log.Fatal(http.ListenAndServe(myConfiguration.HttpListenAddress, routerWithTimeout))
}
