package database

import "time"

// AntennaFrequency is the existing gateway-channel association table.
type AntennaFrequency struct {
	ID          uint
	AntennaID   uint `gorm:"uniqueIndex:idx_antenna_frequency"`
	FrequencyID uint `gorm:"uniqueIndex:idx_antenna_frequency"`
	Count       uint64
	LastHeard   time.Time
}

type AprsDevice struct {
	ID          uint
	NetworkId   string `gorm:"index:aprs_network_app_dev,unique"`
	AppId       string `gorm:"index:aprs_network_app_dev,unique"`
	DevId       string `gorm:"index:aprs_network_app_dev,unique"`
	Callsign    string
	Ssid        int
	SymbolTable string
	Symbol      string
}
