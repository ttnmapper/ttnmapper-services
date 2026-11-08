package responses

import "time"

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

//type NetworkGateways struct {
//	_ []Gateway
//}

type Gateway struct {
	DatabaseId uint   `json:"database_id"`
	NetworkId  string `json:"network_id"`
	GatewayId  string `json:"gateway_id"`
	GatewayEUI string `json:"gateway_eui"`
	Name       string `json:"name"`

	LastHeard time.Time `json:"last_heard"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  int32     `json:"altitude"`

	North float64 `json:"north"`
	South float64 `json:"south"`
	West  float64 `json:"west"`
	East  float64 `json:"east"`

	Attributes map[string]interface{} `json:"attributes"`
}

type DeviceMeasurement struct {
	Id   uint      `json:"database_id"`
	Time time.Time `json:"time"`

	AppId           string `json:"app_id"`
	DevId           string `json:"dev_id"`
	DevEui          string `json:"dev_eui"`
	DeviceNetworkId string `json:"device_network_id"`

	FPort uint `json:"f_port"`
	FCnt  uint `json:"f_cnt"`

	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Altitude       float64 `json:"altitude"`
	AccuracyMeters float64 `json:"accuracy_meters"`
	Satellites     uint    `json:"satellites"`
	Hdop           float64 `json:"hdop"`
	AccuracySource string  `json:"location_source"`

	ChannelIndex uint     `json:"channel_index"`
	Rssi         float64  `json:"rssi"`
	SignalRssi   *float64 `json:"signal_rssi"`
	Snr          float64  `json:"snr"`

	Frequency       uint   `json:"frequency"`
	Modulation      string `json:"modulation"`
	Bandwidth       uint   `json:"bandwidth"`
	SpreadingFactor uint   `json:"spreading_factor"`
	Bitrate         uint   `json:"bitrate"`
	CodingRate      string `json:"coding_rate"`

	GatewayNetworkId string    `json:"gateway_network_id"`
	GatewayId        string    `json:"gateway_id"`
	AntennaIndex     uint      `json:"antenna_index"`
	GatewayTime      time.Time `json:"gateway_time"`
	FineTimestamp    uint64    `json:"fine_timestamp"`

	UserAgent string `json:"user_agent"`

	Experiment string `json:"experiment"`
}

type ExperimentResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func TruncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
