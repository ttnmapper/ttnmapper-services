package main

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/oldstack"
	"ttnmapper-services/pkg/utils"

	"github.com/kelseyhightower/envconfig"
)

func Init() {
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
}

func TestMysqlPacketToUplink(t *testing.T) {
	Init()
	packet := oldstack.GetPacket(455916)
	uplinkMessage := MysqlPacketToUplinkMessage(packet)
	//log.Println(utils.PrettyPrint(uplinkMessage))
	if uplinkMessage.UserId != "jpmeijers" {
		t.Fatalf("userid wrong")
	}

	packet = oldstack.GetPacket(218190)
	uplinkMessage = MysqlPacketToUplinkMessage(packet)
	log.Println(utils.PrettyPrint(uplinkMessage))
	if uplinkMessage.UserId != "http://pade.nl/lora/coverage.json" {
		t.Fatalf("userid wrong")
	}

	packet = oldstack.GetPacket(340010)
	uplinkMessage = MysqlPacketToUplinkMessage(packet)
	log.Println(utils.PrettyPrint(uplinkMessage))
	if uplinkMessage.UserId != "JPM - Enschede" {
		t.Fatalf("userid wrong")
	}

	packet = oldstack.GetPacket(3400100)
	uplinkMessage = MysqlPacketToUplinkMessage(packet)
	log.Println(utils.PrettyPrint(uplinkMessage))
	if uplinkMessage.UserId != "f5hugGDXunU" {
		t.Fatalf("userid wrong")
	}
	if uplinkMessage.AccuracySource != "fused" {
		t.Fatalf("accuracy source wrong")
	}
}

func TestGetPackets(t *testing.T) {
	Init()
	mysqlPackets := oldstack.GetPackets(1, 32402)

	for _, mPacket := range mysqlPackets {
		log.Println(utils.PrettyPrint(mPacket))
		uplinkMessage := MysqlPacketToUplinkMessage(mPacket)
		log.Println(utils.PrettyPrint(uplinkMessage))
		if uplinkMessage.AccuracySource != "gps" {
			t.Fatalf("accuracy must be from gps")
		}
	}

	mysqlPackets = oldstack.GetPackets(1, 227388)

	for _, mPacket := range mysqlPackets {
		log.Println(utils.PrettyPrint(mPacket))
		uplinkMessage := MysqlPacketToUplinkMessage(mPacket)
		log.Println(utils.PrettyPrint(uplinkMessage))
		if uplinkMessage.AccuracySource != "ios" {
			t.Fatalf("accuracy must be from ios")
		}
	}
}
