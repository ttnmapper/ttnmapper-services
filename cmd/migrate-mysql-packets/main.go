package main

import (
	"log"
	"time"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/oldstack"
	"ttnmapper-services/pkg/types"
	"ttnmapper-services/pkg/utils"

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

	// Stop datetime
	stopDateTime := time.Date(2020, 6, 27, 19, 46, 5, 713330000, time.UTC)

	// Get all gateway moves from mysql
	var mysqlPackets []oldstack.Packet
	//var offset uint = 175972727 //
	// 2021/09/18 05:06:21 Inserted 175971727
	// 2021/09/18 05:06:21 175971803 already in postgres
	// 2021/09/18 05:06:21 175972727 already in postgres
	// 2021/09/18 05:06:21 ERROR: syntax error at end of input (SQLSTATE 42601)
	// 20000858 // 2021-09-17 16:00 SAST interrupt for hdd resize  0 //1409906

	// Experiments
	var offset uint = 0

	for {
		//mysqlPackets = oldstack.GetPackets(500, offset)
		mysqlPackets = oldstack.GetExperimentPackets(500, offset)
		if len(mysqlPackets) == 0 {
			break
		}

		uplinkMessages := make([]types.TtnMapperUplinkMessage, 0)
		for _, mPacket := range mysqlPackets {
			offset = mPacket.ID

			if mPacket.Time.After(stopDateTime) {
				//log.Println(offset, "already in postgres")
				continue
			}

			message := MysqlPacketToUplinkMessage(mPacket)
			//log.Println(offset)
			//log.Println(utils.PrettyPrint(mPacket))
			//log.Println(utils.PrettyPrint(message))
			//log.Println(offset, message.UserId)
			//fmt.Printf("%d: %s, ", offset, message.UserId)
			uplinkMessages = append(uplinkMessages, message)
		}

		// Add this batch to postgres

		var entriesToInsert []database.Packet
		for _, message := range uplinkMessages {
			for _, gateway := range message.Gateways {
				// Copy required fields in correct format into a database row struct
				entry, err := database.UplinkMessageToPacket(message, gateway)
				utils.FailOnError(err, "")

				entriesToInsert = append(entriesToInsert, entry)
			}
		}

		err := database.InsertPacketsBatch(entriesToInsert)
		if err != nil {
			log.Println(err.Error())
		}
		log.Println("Inserted", offset)
	}

}

func MysqlPacketToUplinkMessage(mPacket oldstack.Packet) types.TtnMapperUplinkMessage {
	var message types.TtnMapperUplinkMessage

	message.Time = mPacket.Time.UnixNano()

	message.NetworkId = "thethingsnetwork.org"
	if mPacket.AppId.Valid {
		message.AppID = mPacket.AppId.String
	}
	message.DevID = mPacket.NodeAddr

	// Modulation
	if mPacket.Modulation.Valid {
		message.Modulation = mPacket.Modulation.String
	} else {
		message.Modulation = "LORA"
	}

	// DataRate
	if message.Modulation == "LORA" {
		if mPacket.DataRate.Valid {
			sf, bw := oldstack.DatarateToSfBw(mPacket.DataRate.String)
			message.SpreadingFactor = sf
			message.Bandwidth = bw
		}
	} else if message.Modulation == "FSK" {
		// mysql did not store a datarate string for fsk
		// Examples: 99937947
		//99940389
		//99941580
		//99944214
		//99946794
		//99950030
		//99952599
		message.Bitrate = 50000 // Regional parameters v1.0.1 always uses 50kbps for fsk
	}

	message.CodingRate = "4/5" // assume as this is standard

	// Freq
	message.Frequency = utils.SanitizeFrequency(mPacket.Frequency)

	// Fcnt
	if mPacket.FrameCount.Valid {
		message.FCnt = int64(mPacket.FrameCount.Int32)
	} else {
		message.FCnt = 0
	}

	// Lat
	message.Latitude = mPacket.Latitude
	// Lon
	message.Longitude = mPacket.Longitude
	// Alt
	if mPacket.Altitude.Valid {
		message.Altitude = mPacket.Altitude.Float64
	} else {
		message.Altitude = 0
	}

	// Accuracy
	if mPacket.Accuracy.Valid {
		message.AccuracyMeters = mPacket.Accuracy.Float64
	}
	// Hdop
	if mPacket.Hdop.Valid {
		message.Hdop = mPacket.Hdop.Float64
	}
	// Sats
	if mPacket.Satellites.Valid {
		message.Satellites = mPacket.Satellites.Int32
	}

	// Provider
	/*
		gps
		NULL
		network
		http://pade.nl/lora/coverage.json
		ursm-zurich
		hans-enschede
		Trooster - Hengelo
		urs-zurich
		decentlab-zurich
		Martijn Griekspoor - Amsterdam
		iOS
		iPhone
		JPM - Enschede
		lukas@theiler.io
		lora-mapper.org
		@kgbvax - MÃ¼nster
		LocateGlobal.eu
		jpmeijers
		PaulB
		vannut
		Tord Andersson
		Corbo - Enschede
		Lukas Haas
		Tjardick
		android-log-file
		ttnmapper@atilas.nl
		Sille
		tjeerdytsma
		Kersing
		nicoschottelius
		DigitalGlarus
		Marco Hebing
		Sodaq balloon test
		WirelessThings
		Yannick Lanz
		Dennisg
		Rene Apeldoorn
		fake
		Christos Tranoris
		Joris Tip
		Dave Olsthoorn
		Luc - Oostende
		Johnny Willemsen
		Pete Hoffswell
		Dennis Ruigrok
		Andri Yadi
		Peter Affolter
		Erik Verberne
		Marcel van Bakel
		Martijn Heus
		Pietje Woerden
		moritz weibel Zürich
		fused
		Michael Schmutz
		Gonçalo Rocha
		FabLab Lannion
		joris_binary
		wireless_things
		sodaq_tracker_2
		ursm
		jpm_ascii
		mbox_gps
		preparsed_json
		lex_ph2lb
		adeunis_demo
		sodaq_universal_tracker_v2
		loramote_2_gps
		iopush-gps-001


		HDOP
		Cayenne LPP
		sats
		loraone_v3


		payload_fields
		accuracy
		gps_hdop

		satellites
		numsat
		titi
		custom
		custom payload / titi
		registry
	*/
	if mPacket.Provider.Valid && mPacket.UserId == "" && isAccuracyProvider(mPacket.Provider.String) {
		// For a very short period we stored the accuracy source in this field
		message.AccuracySource = mPacket.Provider.String
		message.UserId = mPacket.UserId
	} else if mPacket.Provider.Valid && mPacket.UserId == "" {
		// Then we started storing the user in this field
		message.UserId = mPacket.Provider.String
	} else if mPacket.Provider.Valid {
		// Then we added the userid field and used provider for the accuracy source
		message.AccuracySource = mPacket.Provider.String
		message.UserId = mPacket.UserId
	} else {
		// and if the provider field is not set, just use the userid
		message.UserId = mPacket.UserId
	}

	// MqttTopic
	// not used

	// UserAgent
	if mPacket.UserAgent.Valid {
		message.UserAgent = mPacket.UserAgent.String
	}
	// Experiment
	message.Experiment = mPacket.ExperimentName

	// Gateway
	// SNR
	// RSSI
	networkId, gatewayId, gatewayEui := oldstack.GwaddrToNetIdEui(mPacket.GatewayAddr)
	gateway := types.TtnMapperGateway{
		NetworkId:    networkId,
		GatewayId:    gatewayId,
		GatewayEui:   gatewayEui,
		AntennaIndex: 0,
		Time:         0,
		ChannelIndex: 0,
		Rssi:         float32(mPacket.Rssi),
		SignalRssi:   0,
		Snr:          float32(mPacket.Snr),
	}
	message.Gateways = append(message.Gateways, gateway)

	return message
}

func isAccuracyProvider(lookup string) bool {
	switch lookup {
	case
		"gps",
		"NULL",
		"network",
		"iOS",
		"ios",
		"iPhone",
		"android-log-file",
		"fake",
		"fused",
		"preparsed_json",
		"HDOP",
		"Cayenne LPP",
		"sats",
		"loraone_v3",
		"payload_fields",
		"accuracy",
		"gps_hdop",
		"satellites",
		"numsat",
		"titi",
		"custom",
		"custom payload / titi",
		"registry":
		return true
	}
	return false
}
