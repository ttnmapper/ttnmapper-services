package database

import (
	"gorm.io/datatypes"
	"time"
)

type Packet struct {
	ID   uint
	Time time.Time `gorm:"not null;index:idx_packets_antenna_id_time_experiment_id,priority:3;index:idx_packets_device_id_time,priority:2;index:idx_packets_experiment_id_time,where:experiment_id is not null,priority:2"` // index priority 11 is lower than default 10. Device and gateway is less unique, so will filter better first step.

	DeviceID uint `gorm:"not null;index:idx_packets_device_id_time_experiment_id,priority:1"`

	FPort uint8
	FCnt  uint32

	FrequencyID  uint `gorm:"index:idx_packets_antenna_id_frequency_id,priority:2"`
	DataRateID   uint
	CodingRateID uint

	// Gateway data
	// TODO antennaid time latitude experiment index, as we need to select max latitude since gateway moved
	AntennaID              uint `gorm:"not null;index:idx_packets_antenna_id_time,priority:1;index:idx_packets_antenna_id_latitude_experiment_id,priority:1;index:idx_packets_antenna_id_longitude_experiment_id,priority:1,index:idx_packets_antenna_id_frequency_id,priority:1"`
	GatewayTime            *time.Time
	Timestamp              *uint32
	FineTimestamp          *uint64
	FineTimestampEncrypted *[]byte
	FineTimestampKeyID     *uint
	ChannelIndex           uint32
	Rssi                   float32  `gorm:"type:numeric(6,2)"`
	SignalRssi             *float32 `gorm:"type:numeric(6,2)"`
	Snr                    float32  `gorm:"type:numeric(5,2)"`

	Latitude         float64  `gorm:"not null;type:numeric(10,6);index:idx_packets_antenna_id_latitude_experiment_id,priority:2"`
	Longitude        float64  `gorm:"not null;type:numeric(10,6);index:idx_packets_antenna_id_longitude_experiment_id,priority:2"`
	Altitude         float64  `gorm:"type:numeric(6,1)"`
	AccuracyMeters   *float64 `gorm:"type:numeric(6,2)"`
	Satellites       *int32
	Hdop             *float64 `gorm:"type:numeric(4,1)"`
	AccuracySourceID uint

	ExperimentID *uint `gorm:"index:idx_packets_device_id_time_experiment_id,priority:2;index:idx_packets_antenna_id_latitude_experiment_id,priority:3;index:idx_packets_antenna_id_longitude_experiment_id,priority:3;index:idx_packets_experiment_id_time,where:experiment_id is not null,priority:1"`

	UserID      uint
	UserAgentID uint

	DeletedAt *time.Time
}

type Device struct {
	ID        uint
	NetworkId string `gorm:"index:net_app_dev_eui,unique"`
	AppId     string `gorm:"index:net_app_dev_eui,unique"`
	DevId     string `gorm:"index:net_app_dev_eui,unique"`
	DevEui    string `gorm:"index:net_app_dev_eui,unique"`
	Packets   []Packet
}

type Frequency struct {
	ID      uint
	Herz    uint64 `gorm:"unique;not null"`
	Packets []Packet
}

type DataRate struct {
	ID              uint
	Modulation      string `gorm:"index:data_rate,unique"` // LORA or FSK or LORA-E
	Bandwidth       uint64 `gorm:"index:data_rate,unique"`
	SpreadingFactor uint8  `gorm:"index:data_rate,unique"`
	Bitrate         uint64 `gorm:"index:data_rate,unique"`
	Packets         []Packet
}

type CodingRate struct {
	ID      uint
	Name    string `gorm:"unique;not null"`
	Packets []Packet
}

type AccuracySource struct {
	ID      uint
	Name    string `gorm:"unique;not null"`
	Packets []Packet
}

type Experiment struct {
	ID      uint
	Name    string `gorm:"unique;not null"`
	Packets []Packet
}

type User struct {
	ID         uint
	Identifier string `gorm:"unique;not null"`
	Packets    []Packet
}

type UserAgent struct {
	ID      uint
	Name    string `gorm:"unique;not null"`
	Packets []Packet
}

// TODO: Currently we identify a gateway using the gateway ID provided by the network.
// But how are we going to identify them between networks, when data is sent via the packet broker?

type Antenna struct {
	ID uint

	// TTN gateway ID along with the Antenna index identifies a unique coverage area.
	NetworkId    string `gorm:"type:text;index:idx_gtw_id_antenna,unique"`
	GatewayId    string `gorm:"type:text;index:idx_gtw_id_antenna,unique"`
	AntennaIndex uint8  `gorm:"index:idx_gtw_id_antenna,unique"`

	// For now we do not set antenna locations, but add it here for future use
	//Latitude         *float64
	//Longitude        *float64
	//Altitude         *int32

	Packets []Packet
}

type Gateway struct {
	ID uint

	NetworkId  string `gorm:"type:text;UNIQUE_INDEX:idx_gtw_id;INDEX:idx_gtw_network_name,priority:1"`
	GatewayId  string `gorm:"type:text;UNIQUE_INDEX:idx_gtw_id"`
	GatewayEui *string
	Name       *string `gorm:"type:text;INDEX:idx_gtw_network_name,priority:2"`

	Latitude         float64
	Longitude        float64
	Altitude         int32
	LocationAccuracy *int32
	LocationSource   *string

	//AtLocationSince	time.Time // This value gets updated when the gateway moves
	LastHeard time.Time // This value always gets updated to reflect that the gateway is working

	//Antennas         []Antenna
	//GatewayLocations []GatewayLocation

	Attributes datatypes.JSON // general info like frequency plan, description, etc
}

type GatewayLocation struct {
	ID        uint
	NetworkId string `gorm:"type:text;INDEX:idx_gtw_id_install"`
	GatewayId string `gorm:"type:text;INDEX:idx_gtw_id_install"`

	InstalledAt time.Time `gorm:"INDEX:idx_gtw_id_install"`
	Latitude    float64
	Longitude   float64
	Altitude    int32
}

// To blacklist a gateway set its location to 0,0
type GatewayLocationForce struct {
	ID        uint
	NetworkId string `gorm:"type:text;UNIQUE_INDEX:idx_gtw_id_force"`
	GatewayId string `gorm:"type:text;UNIQUE_INDEX:idx_gtw_id_force"`

	Latitude  float64
	Longitude float64
	Altitude  int32
}

type FineTimestampKeyID struct {
	ID                          uint
	FineTimestampEncryptedKeyId string
}

type TtsV3FetchStatus struct {
	ID       uint
	TenantId string
	ApiKey   string
}

type PacketBrokerRoutingPolicy struct {
	/*
		{
			"forwarder_tenant_id": "cropwatch",
			"home_network_net_id": 19,
			"home_network_tenant_id": "ttn",
			"updated_at": {
				"seconds": 1622040895,
				"nanos": 189412000
			},
			"uplink": {
				"join_request": true,
				"mac_data": true,
				"application_data": true,
				"signal_quality": true,
				"localization": true
			},
			"downlink": {
				"join_accept": true,
				"mac_data": true,
				"application_data": true
			}
		},
	*/
	ID                      uint
	HomeNetworkId           string `gorm:"type:text;UNIQUE_INDEX:idx_pb_route"`
	ForwarderNetworkId      string `gorm:"type:text;UNIQUE_INDEX:idx_pb_route"`
	UplinkJoinRequest       bool
	UplinkMacData           bool
	UplinkApplicationData   bool
	UplinkSignalQuality     bool
	UplinkLocalization      bool
	DownlinkJoinAccept      bool
	DownlinkMacData         bool
	DownlinkApplicationData bool
}

type NetworkSubscription struct {
	ID                  uint
	NetworkId           string `gorm:"type:text;index:idx_subs_networkid,unique"`
	GatewayNames        bool
	GatewayDescriptions bool
}

type GatewayBoundingBox struct {
	ID        uint
	NetworkId string  `gorm:"type:text;index:idx_bbox_gtw_id,unique"`
	GatewayId string  `gorm:"type:text;index:idx_bbox_gtw_id,unique"`
	North     float64 `gorm:"not null;index:idx_coords_bbox"`
	South     float64 `gorm:"not null;index:idx_coords_bbox"`
	East      float64 `gorm:"not null;index:idx_coords_bbox"`
	West      float64 `gorm:"not null;index:idx_coords_bbox"`
}

type GatewayWithBoundingBox struct {
	Gateway
	GatewayBoundingBox
}

// Indexers: These structs are the same as the ones above, but used to index the cache maps
type DeviceIndexer struct {
	NetworkId string
	DevId     string
	AppId     string
	DevEui    string
}

type GatewayIndexer struct {
	NetworkId string
	GatewayId string
}

type AntennaIndexer struct {
	NetworkId    string
	GatewayId    string
	AntennaIndex uint8
}

type DataRateIndexer struct {
	Modulation      string // LORA or FSK or LORA-E
	Bandwidth       uint64
	SpreadingFactor uint8
	Bitrate         uint64
}
