package chirpstack_v3

import (
	"log"
	"os"
	"testing"
	"ttnmapper-services/internal/ingressapi/types"
	sharedutil "ttnmapper-services/pkg/utils"
)

func TestChirpV3(t *testing.T) {
	body, err := os.ReadFile("chirp_v3_output.json")
	if err != nil {
		t.Fatal(err)
	}
	packetOut := types.TtnMapperUplinkMessage{NetworkId: types.NS_CHIRP + "://test.example"}
	packetIn, err := decodeUplinkEvent(body)
	if err != nil {
		t.Fatal(err)
	}
	// Gateway
	// "location": {
	//     "latitude": 52.445313166666665,
	//     "longitude": 10.8140175,
	//     "altitude": 100,
	//     "source": "UNKNOWN",
	//     "accuracy": 0
	//   }
	CopyChirpV3Fields(packetIn, &packetOut)
	gw := packetOut.Gateways[0]

	log.Println(sharedutil.PrettyPrint(packetIn))
	log.Println(sharedutil.PrettyPrint(packetOut))

	gatewayCheck := gw.Latitude == 52.445313166666665 && gw.Longitude == 10.8140175 &&
		gw.Altitude == 100 && gw.LocationSource == "UNKNOWN" && gw.LocationAccuracy == 0
	if !gatewayCheck {
		t.Error("Gateway was not parsed correctly")
		return
	}

	// {\"altitude\":0,\"battery\":3977,\"downlink\":3,\"latitude\":52.42758333333333,\"longitude\":10.7915,\"rssi\":-54,\"sats\":5,
	if err := ParseChirpV3Payload(packetIn, &packetOut); err != nil {
		t.Error(err)
		return
	}

	payloadCheck := packetOut.Altitude == 0 && packetOut.Latitude == 52.42758333333333 &&
		packetOut.Longitude == 10.7915 && packetOut.Satellites == 5
	if !payloadCheck {
		t.Error("Payload was not parsed correctly")
		return
	}

}
