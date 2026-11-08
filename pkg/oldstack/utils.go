package oldstack

import (
	"log"
	"strconv"
	"strings"
)

func GwaddrToNetIdEui(gwaddr string) (networkId string, gatewayId string, gatewayEui string) {

	// Assume TTNv2
	networkId = "thethingsnetwork.org"

	possibleEui := strings.TrimPrefix(gwaddr, "eui-")
	_, err := strconv.ParseUint(possibleEui, 16, 64)
	if len(possibleEui) == 16 && err == nil {
		// Is 16 hex chars, so an eui
		gatewayEui = strings.ToUpper(possibleEui)
		gatewayId = "eui-" + strings.ToLower(possibleEui)
	} else {
		// Otherwise it's not an eui
		gatewayId = gwaddr
	}

	return
}

func DatarateToSfBw(datarate string) (spreadingFactor uint8, bandwidth uint64) {
	// If empty, assume SF7BW125
	if datarate == "" {
		return 7, 125000
	}
	drParts := strings.Split(datarate, "BW")
	bandwidthInt, err := strconv.Atoi(drParts[1])
	if err != nil {
		log.Fatal(err)
	}
	bandwidth = uint64(bandwidthInt * 1000) // kHz to Hz
	sf, err := strconv.Atoi(strings.TrimPrefix(drParts[0], "SF"))
	if err != nil {
		log.Fatal(err)
	}
	spreadingFactor = uint8(sf)

	return spreadingFactor, bandwidth
}
