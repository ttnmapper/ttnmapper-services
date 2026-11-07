package main

import (
	"database/sql"
	"testing"
	"time"
	"ttnmapper-services/pkg/oldstack"
)

func TestMysqlPacketToUplink(t *testing.T) {
	for _, tc := range []struct {
		name, provider, user, wantUser, wantSource string
		valid                                      bool
	}{
		{"legacy accuracy", "gps", "", "", "gps", true},
		{"legacy user", "mapper", "", "mapper", "", true},
		{"separate fields", "fused", "mapper", "mapper", "fused", true},
		{"no provider", "", "mapper", "mapper", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet := oldstack.Packet{Time: time.Unix(1600000000, 123), NodeAddr: "device", AppId: sql.NullString{String: "app", Valid: true}, GatewayAddr: "001122334455aabb", Frequency: 868.1, DataRate: sql.NullString{String: "SF9BW125", Valid: true}, Provider: sql.NullString{String: tc.provider, Valid: tc.valid}, UserId: tc.user, Latitude: -33.9, Longitude: 18.8, Rssi: -90, Snr: 5}
			got := MysqlPacketToUplinkMessage(packet)
			if got.UserId != tc.wantUser || got.AccuracySource != tc.wantSource {
				t.Errorf("user=%q source=%q", got.UserId, got.AccuracySource)
			}
			if got.Time != 1600000000000000123 || got.AppID != "app" || got.DevID != "device" || got.Frequency != 868100000 || got.Modulation != "LORA" || got.SpreadingFactor != 9 || got.Bandwidth != 125000 || got.Latitude != -33.9 || got.Longitude != 18.8 {
				t.Errorf("uplink=%+v", got)
			}
			if len(got.Gateways) != 1 {
				t.Fatalf("gateways=%+v", got.Gateways)
			}
			gw := got.Gateways[0]
			if gw.GatewayId != "eui-001122334455aabb" || gw.GatewayEui != "001122334455AABB" || gw.Rssi != -90 || gw.Snr != 5 {
				t.Errorf("gateway=%+v", gw)
			}
		})
	}
}
