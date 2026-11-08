package websiteapi

import (
	"log"
	"net/http"
	"time"
	"ttnmapper-services/pkg/database"

	"github.com/kelseyhightower/envconfig"
)

type Configuration struct {
	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresPort     string `envconfig:"POSTGRES_PORT"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDatabase string `envconfig:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `envconfig:"POSTGRES_DEBUG_LOG"`

	HttpListenAddress string `envconfig:"HTTP_LISTEN_ADDRESS"`
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

	// Register routes
	router := Routes()

	// Start the http endpoint
	log.Println("Starting server on", myConfiguration.HttpListenAddress)
	routerWithTimeout := http.TimeoutHandler(router, time.Minute*1, "Handler Timeout!")
	log.Fatal(http.ListenAndServe(myConfiguration.HttpListenAddress, routerWithTimeout))
}
