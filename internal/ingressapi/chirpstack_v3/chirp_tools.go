package chirpstack_v3

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"ttnmapper-services/internal/ingressapi/types"
	"ttnmapper-services/internal/ingressapi/utils"
)

func ParseChirpV3Payload(packetIn UplinkEvent, packetOut *types.TtnMapperUplinkMessage) error {
	var payloadFieldsIn map[string]interface{}
	if err := json.Unmarshal([]byte(packetIn.ObjectJson), &payloadFieldsIn); err != nil {
		return err
	}

	if err := utils.ParsePayloadFields(int64(packetOut.FPort), payloadFieldsIn, packetOut); err != nil {
		return err
	}

	return nil
}

func CopyChirpV3Fields(packetIn UplinkEvent, packetOut *types.TtnMapperUplinkMessage) {
	packetOut.Time = time.Now().UnixNano()

	packetOut.AppID = packetIn.ApplicationName
	packetOut.DevID = packetIn.DeviceName
	packetOut.DevEui = strings.ToUpper(hex.EncodeToString(packetIn.DevEui))

	packetOut.FPort = uint8(packetIn.FPort)
	packetOut.FCnt = int64(packetIn.FCnt)

	packetOut.Frequency = utils.SanitizeFrequency(float64(packetIn.TxInfo.Frequency))
	packetOut.Modulation = packetIn.TxInfo.Modulation.String()

	if packetOut.Modulation == "LORA" {
		modInfo := packetIn.TxInfo.LoraModulationInfo
		packetOut.SpreadingFactor = uint8(modInfo.SpreadingFactor)
		packetOut.Bandwidth = uint64(modInfo.Bandwidth * 1000) // kHz to Hz
		packetOut.CodingRate = modInfo.CodeRate
	} else { // FSK
		modInfo := packetIn.TxInfo.FskModulationInfo
		packetOut.Bitrate = uint64(modInfo.Datarate)
	}

	for _, gatewayIn := range packetIn.RxInfo {
		gatewayOut := types.TtnMapperGateway{}
		gatewayOut.Attributes = make(map[string]interface{}, 0)

		gatewayOut.NetworkId = packetOut.NetworkId
		gatewayEui := hex.EncodeToString(gatewayIn.GatewayId)
		gatewayOut.GatewayId = "eui-" + strings.ToLower(gatewayEui)
		gatewayOut.GatewayEui = strings.ToUpper(gatewayEui)
		// gatewayOut.Description = ... // TODO: Get Gateway Name

		// If the gateway id is packetbroker, ignore
		if gatewayOut.GatewayId == "packetbroker" {
			continue
		}

		// gateway Time is the wall clock time
		if gatewayIn.Time != nil {
			gatewayOut.Time = int64(gatewayIn.Time.Nanosecond())
		}

		// gateway Timestamp is the internal clock counter of the concentrator
		// not provided by ChirpStack

		// Fine timestamp - not encrypted
		if gatewayIn.FineTimestampType == fineTimestampPlain {
			gatewayOut.FineTimestamp = uint64(gatewayIn.PlainFineTimestamp.Time.Nanosecond())
		}
		// Fine timestamp - encrypted
		if gatewayIn.FineTimestampType == fineTimestampEncrypted {
			gatewayOut.FineTimestampEncrypted = gatewayIn.EncryptedFineTimestamp.EncryptedNs
			gatewayOut.FineTimestampEncryptedKeyId = strconv.Itoa(int(gatewayIn.EncryptedFineTimestamp.AesKeyIndex))
		}

		gatewayOut.AntennaIndex = uint8(gatewayIn.Antenna)
		gatewayOut.ChannelIndex = uint32(gatewayIn.Channel)
		gatewayOut.Rssi = float32(gatewayIn.Rssi)
		// Missing Channel/Signal RSSI
		gatewayOut.Snr = float32(gatewayIn.LoraSnr)

		if gatewayIn.Location != nil {
			gatewayOut.Latitude = float64(gatewayIn.Location.Latitude)
			gatewayOut.Longitude = float64(gatewayIn.Location.Longitude)
			gatewayOut.Altitude = int32(gatewayIn.Location.Altitude)
			gatewayOut.LocationAccuracy = int32(gatewayIn.Location.Accuracy)
			gatewayOut.LocationSource = gatewayIn.Location.Source.String()
		}

		packetOut.Gateways = append(packetOut.Gateways, gatewayOut)
	}
}
