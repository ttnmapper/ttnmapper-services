package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"slices"

	"github.com/artyom/csvstruct"
	"github.com/kelseyhightower/envconfig"

	"ttnmapper-services/pkg/database"
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

type DataBbox struct {
	North float64 `csv:"north"`
	East  float64 `csv:"east"`
	South float64 `csv:"south"`
	West  float64 `csv:"west"`
}

func main() {

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

	bboxes := getBboxes("cmd/export-packets/Apeldoorn.csv")

	var gateways []database.Gateway
	for _, bbox := range bboxes {
		gateway := database.GetGwsInBb(bbox.North, bbox.East, bbox.South, bbox.West)
		gateways = append(gateways, gateway...)
	}

	slices.CompactFunc(gateways, func(i, j database.Gateway) bool {
		return i.GatewayId == j.GatewayId
	})

	log.Println("Found ", len(gateways), " gateways")

	var packets []database.Packet
	for _, gateway := range gateways {
		log.Println("Getting packets for ", gateway.NetworkId, " - ", gateway.GatewayId, " ...")

		antennas := database.GetAntennaForGateway(gateway.NetworkId, gateway.GatewayId)
		movedTime := database.GetGatewayLastMovedTime(gateway.NetworkId, gateway.GatewayId)
		log.Println("Last move", movedTime)

		for _, antenna := range antennas {
			rows, err := database.GetPacketsForAntennaAfter(antenna, movedTime)
			if err != nil {
				log.Println(err.Error())
				return
			}

			// Drop any rows outside our list of bounding boxes
			gatewayPackets := rowsToPacketsInRange(rows, bboxes)
			packets = append(packets, gatewayPackets...)
			rows.Close()
		}
	}

	file, err := os.Create("gateway-ids.txt")
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer file.Close()
	for _, gateway := range gateways {
		_, _ = file.WriteString(fmt.Sprintf("%d,", gateway.ID))
	}

	filePackets, err := os.Create("packet-ids.txt")
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer filePackets.Close()

	queryStart := `SELECT * FROM packets
         JOIN public.frequencies frequency on packets.frequency_id = frequency.id
         JOIN public.data_rates datarate on packets.data_rate_id = datarate.id
         JOIN public.coding_rates codingrate on packets.coding_rate_id = codingrate.id
         JOIN public.antennas antenna on packets.antenna_id = antenna.id
         JOIN public.accuracy_sources accuracy_source on packets.accuracy_source_id = accuracy_source.id`

	filePackets.WriteString(queryStart + " WHERE packets.id IN (")

	for _, packet := range packets {
		filePackets.WriteString(fmt.Sprintf("%d,", packet.ID))
	}

	filePackets.WriteString(")")
}

func getBboxes(filename string) []DataBbox {
	var bboxes []DataBbox

	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		log.Fatal(err)
	}
	scan, err := csvstruct.NewScanner(header, &DataBbox{})
	if err != nil {
		log.Fatal(err)
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		var bbox DataBbox
		if err := scan(record, &bbox); err != nil {
			log.Fatal(err)
		}
		bboxes = append(bboxes, bbox)
	}
	return bboxes
}

func rowsToPacketsInRange(rows *sql.Rows, bboxes []DataBbox) []database.Packet {
	var packets []database.Packet
	i := 0
	for rows.Next() {
		i++
		fmt.Printf("\rPacket %d   ", i)

		var packet database.Packet
		err := database.ScanRows(rows, &packet)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		inRange := false
		for _, bbox := range bboxes {
			if isInBbox(packet, bbox) {
				inRange = true
				break
			}
		}

		if inRange {
			packets = append(packets, packet)
		}

	}
	return packets
}

func isInBbox(packet database.Packet, bbox DataBbox) bool {
	if packet.Latitude < bbox.South || packet.Latitude > bbox.North {
		return false
	}
	if packet.Longitude < bbox.West || packet.Longitude > bbox.East {
		return false
	}
	return true
}
