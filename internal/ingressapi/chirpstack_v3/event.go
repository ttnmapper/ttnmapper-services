package chirpstack_v3

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// UplinkEvent contains the ChirpStack v3 JSON webhook fields used by the mapper.
// v3 accepts JSON only. Importing its generated protobuf types alongside v4
// registers incompatible descriptors with identical global names.
type UplinkEvent struct {
	ApplicationName string      `json:"applicationName"`
	DeviceName      string      `json:"deviceName"`
	DevEui          base64Bytes `json:"devEUI"`
	FCnt            uint32Value `json:"fCnt"`
	FPort           uint32Value `json:"fPort"`
	ObjectJson      string      `json:"objectJSON"`
	TxInfo          *txInfo     `json:"txInfo"`
	RxInfo          []*rxInfo   `json:"rxInfo"`
}

type txInfo struct {
	Frequency          uint32Value `json:"frequency"`
	Modulation         modulation  `json:"modulation"`
	LoraModulationInfo *struct {
		Bandwidth       uint32Value `json:"bandwidth"`
		SpreadingFactor uint32Value `json:"spreadingFactor"`
		CodeRate        string      `json:"codeRate"`
	} `json:"loRaModulationInfo"`
	FskModulationInfo *struct {
		Datarate uint32Value `json:"datarate"`
	} `json:"fskModulationInfo"`
}

type rxInfo struct {
	GatewayId          base64Bytes       `json:"gatewayID"`
	Time               *time.Time        `json:"time"`
	Rssi               int32Value        `json:"rssi"`
	LoraSnr            floatValue        `json:"loRaSNR"`
	Channel            uint32Value       `json:"channel"`
	Antenna            uint32Value       `json:"antenna"`
	FineTimestampType  fineTimestampType `json:"fineTimestampType"`
	PlainFineTimestamp *struct {
		Time *time.Time `json:"time"`
	} `json:"plainFineTimestamp"`
	EncryptedFineTimestamp *struct {
		AesKeyIndex uint32Value `json:"aesKeyIndex"`
		EncryptedNs base64Bytes `json:"encryptedNS"`
	} `json:"encryptedFineTimestamp"`
	Location *struct {
		Latitude  floatValue     `json:"latitude"`
		Longitude floatValue     `json:"longitude"`
		Altitude  floatValue     `json:"altitude"`
		Accuracy  uint32Value    `json:"accuracy"`
		Source    locationSource `json:"source"`
	} `json:"location"`
}

func decodeUplinkEvent(body []byte) (UplinkEvent, error) {
	var event UplinkEvent
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return event, err
	}
	if _, ok := value.(map[string]any); !ok {
		return event, fmt.Errorf("uplink must be a JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return event, fmt.Errorf("unexpected data after uplink")
	}
	// ProtoJSON accepts both the original snake_case and JSON camelCase names.
	// encoding/json matches the resulting keys to struct tags without case.
	data, err := json.Marshal(normalizeFieldNames(value))
	if err != nil {
		return event, err
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return event, err
	}
	if event.TxInfo == nil {
		return event, fmt.Errorf("txInfo is required")
	}
	switch event.TxInfo.Modulation {
	case modulationLoRa:
		if event.TxInfo.LoraModulationInfo == nil {
			return event, fmt.Errorf("loRaModulationInfo is required for LORA")
		}
	case modulationFSK:
		if event.TxInfo.FskModulationInfo == nil {
			return event, fmt.Errorf("fskModulationInfo is required for FSK")
		}
	default:
		return event, fmt.Errorf("unsupported modulation")
	}
	for _, gateway := range event.RxInfo {
		if gateway == nil {
			return event, fmt.Errorf("rxInfo must not contain null entries")
		}
		if gateway.FineTimestampType == fineTimestampPlain && (gateway.PlainFineTimestamp == nil || gateway.PlainFineTimestamp.Time == nil) {
			return event, fmt.Errorf("plain fine timestamp time is required")
		}
		if gateway.FineTimestampType == fineTimestampEncrypted && gateway.EncryptedFineTimestamp == nil {
			return event, fmt.Errorf("encrypted fine timestamp is required")
		}
	}
	return event, nil
}

func normalizeFieldNames(value any) any {
	switch value := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(value))
		for key, item := range value {
			normalized[strings.ReplaceAll(key, "_", "")] = normalizeFieldNames(item)
		}
		return normalized
	case []any:
		for i, item := range value {
			value[i] = normalizeFieldNames(item)
		}
	}
	return value
}

type uint32Value uint32
type int32Value int32
type floatValue float64

func (n *uint32Value) UnmarshalJSON(data []byte) error {
	var value json.Number
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if string(data) == "null" {
		return nil
	}
	parsed, err := strconv.ParseUint(string(value), 10, 32)
	*n = uint32Value(parsed)
	return err
}

func (n *int32Value) UnmarshalJSON(data []byte) error {
	var value json.Number
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if string(data) == "null" {
		return nil
	}
	parsed, err := strconv.ParseInt(string(value), 10, 32)
	*n = int32Value(parsed)
	return err
}

func (n *floatValue) UnmarshalJSON(data []byte) error {
	var value json.Number
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if string(data) == "null" {
		return nil
	}
	parsed, err := strconv.ParseFloat(string(value), 64)
	*n = floatValue(parsed)
	return err
}

type base64Bytes []byte

func (b *base64Bytes) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	// ProtoJSON accepts standard and URL-safe base64, with or without padding.
	value = strings.NewReplacer("-", "+", "_", "/").Replace(value)
	encoding := base64.StdEncoding
	if !strings.Contains(value, "=") {
		encoding = base64.RawStdEncoding
	}
	decoded, err := encoding.DecodeString(value)
	*b = decoded
	return err
}

type modulation int32
type fineTimestampType int32
type locationSource int32

const (
	modulationLoRa         modulation        = 0
	modulationFSK          modulation        = 1
	fineTimestampEncrypted fineTimestampType = 1
	fineTimestampPlain     fineTimestampType = 2
)

var modulationNames = []string{"LORA", "FSK", "LR_FHSS"}
var fineTimestampNames = []string{"NONE", "ENCRYPTED", "PLAIN"}
var locationSourceNames = []string{"UNKNOWN", "GPS", "CONFIG", "GEO_RESOLVER_TDOA", "GEO_RESOLVER_RSSI", "GEO_RESOLVER_GNSS", "GEO_RESOLVER_WIFI"}

func (v *modulation) UnmarshalJSON(data []byte) error {
	n, err := decodeEnum(data, modulationNames)
	*v = modulation(n)
	return err
}
func (v *fineTimestampType) UnmarshalJSON(data []byte) error {
	n, err := decodeEnum(data, fineTimestampNames)
	*v = fineTimestampType(n)
	return err
}
func (v *locationSource) UnmarshalJSON(data []byte) error {
	n, err := decodeEnum(data, locationSourceNames)
	*v = locationSource(n)
	return err
}
func (v modulation) String() string     { return enumName(int32(v), modulationNames) }
func (v locationSource) String() string { return enumName(int32(v), locationSourceNames) }

func decodeEnum(data []byte, names []string) (int32, error) {
	if len(data) > 0 && data[0] == '"' {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return 0, err
		}
		for i, candidate := range names {
			if name == candidate {
				return int32(i), nil
			}
		}
		return 0, fmt.Errorf("unknown enum name %q", name)
	}
	var value int32
	err := json.Unmarshal(data, &value)
	return value, err
}

func enumName(value int32, names []string) string {
	if value >= 0 && int(value) < len(names) {
		return names[value]
	}
	return strconv.Itoa(int(value))
}
