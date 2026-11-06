package tms

import (
	"fmt"
	"github.com/patrickmn/go-cache"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func initDb() {
	err := LoadConfiguration("conf.json")
	if err != nil {
		log.Println(err)
	}

	log.Printf("[Configuration]\n%s\n", PrettyPrint(MyConfiguration)) // output: [UserA, UserB]

	var gormLogLevel = logger.Silent
	if MyConfiguration.PostgresDebugLog {
		log.Println("Database debug logging enabled")
		gormLogLevel = logger.Info
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=disable application_name=%s",
		MyConfiguration.PostgresHost,
		MyConfiguration.PostgresPort,
		MyConfiguration.PostgresUser,
		MyConfiguration.PostgresDatabase,
		MyConfiguration.PostgresPassword,
		filepath.Base(os.Args[0]),
	)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		panic(err.Error())
	}
}

func TestGetNetworkSamplesInRange(t *testing.T) {
	initDb()

	// https://tile.openstreetmap.org/14/9050/9835.png - Stellebosch Central
	// https://stamen-tiles-d.a.ssl.fastly.net/toner-lite/12/2170/1345.png - wolfsburg central
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(2170, 1345, 12, 0)
	//networkId := "thethingsnetwork.org"
	networkId := "NS_CHIRP://wolfsburg.digital"
	//networkId := "NS_TTS_V3://ttn@000013"
	data, _ := GetNetworkSamplesInRange(networkId, xMin, yMin, xMax, yMax)
	log.Println(data)
}

func TestGetGatewaySamplesInRange(t *testing.T) {
	initDb()

	// https://tile.openstreetmap.org/14/9050/9835.png - Stellebosch Central
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(9050, 9835, 14, 0)
	gatewayId := "eui-60c5a8fffe761551"
	networkId := "thethingsnetwork.org"
	//networkId := "NS_TTS_V3://ttn@000013"
	data, _ := GetGatewaySamplesInRange(networkId, gatewayId, xMin, yMin, xMax, yMax)
	log.Println(data)
}

func TestGetAntennaOnline(t *testing.T) {
	initDb()
	AntennaLastHeardCache = cache.New(1*time.Hour, 2*time.Hour)

	log.Println(GetAntennaOnline(1200))
	log.Println(GetAntennaOnline(1200))
}
