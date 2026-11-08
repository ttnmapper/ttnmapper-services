package database

import (
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
	"log"
	"sync"
	"time"
)

var (
	// Caches for static values, do not expire
	deviceDbCache         sync.Map
	antennaDbCache        sync.Map
	dataRateDbCache       sync.Map
	codingRateDbCache     sync.Map
	frequencyDbCache      sync.Map
	accuracySourceDbCache sync.Map
	userAgentDbCache      sync.Map
	userIdDbCache         sync.Map
	experimentNameDbCache sync.Map

	// Caches for dynamic content, needs to expire
	gatewayDbCache             *cache.Cache
	networkOnlineGatewaysCache *cache.Cache

	Db *gorm.DB
)

type Context PostgresOptions

func (databaseContext *Context) Init() {
	var err error
	Db, err = OpenPostgres(PostgresOptions(*databaseContext))
	if err != nil {
		panic(err)
	}
	gatewayDbCache = cache.New(5*time.Minute, time.Minute)
	networkOnlineGatewaysCache = cache.New(5*time.Minute, time.Minute)
}

func AutoMigrate(models ...interface{}) {
	// Create tables if they do not exist
	log.Println("Performing auto migrate")
	if err := Db.AutoMigrate(
		//	&Device{},
		//	&Frequency{},
		//	&DataRate{},
		//	&CodingRate{},
		//	&AccuracySource{},
		//	&Experiment{},
		//	&User{},
		//	&UserAgent{},
		//	&Antenna{},
		//	&FineTimestampKeyID{},
		//	&Packet{},
		models...,
	); err != nil {
		log.Println("Unable autoMigrateDB - " + err.Error())
	}
}

func GetAllTtsNetworksToFetch() []TtsV3FetchStatus {
	var networks []TtsV3FetchStatus
	Db.Find(&networks)
	return networks
}
