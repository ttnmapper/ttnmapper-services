package helium

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
	"ttnmapper-services/pkg/types"
)

func FetchStatuses(cursor string) (HotspotApiResponse, error) {
	var apiResponse HotspotApiResponse

	httpClient := http.Client{
		Timeout: time.Second * 60, // Maximum of 1 minute
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.helium.io/v1/hotspots", nil)
	if err != nil {
		return apiResponse, err
	}
	req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")

	if cursor != "" {
		q := url.Values{}
		q.Add("cursor", cursor)
		req.URL.RawQuery = q.Encode()
	}

	log.Println("Fetching", req.URL.String())
	res, err := httpClient.Do(req)
	if err != nil {
		return apiResponse, err
	}

	// debug print body
	//log.Println(utils.PrettyPrint(res.Header))
	//
	buf, bodyErr := ioutil.ReadAll(res.Body)
	if bodyErr != nil {
		log.Print("bodyErr ", bodyErr.Error())
	}
	//log.Printf("%s", buf)
	//return hotspots, nil
	// end debug print body

	//err = json.NewDecoder(res.Body).Decode(&apiResponse)
	err = json.Unmarshal(buf, &apiResponse)
	if err != nil {
		log.Printf("%s", buf)
		return apiResponse, err
	}

	err = res.Body.Close()
	if err != nil {
		return apiResponse, err
	}

	return apiResponse, nil
}

func HeliumHotspotToTtnMapperGateway(hotspot Hotspot) (types.TtnMapperGateway, error) {

	gateway := types.TtnMapperGateway{
		NetworkId:    "NS_HELIUM://000024",
		GatewayId:    hotspot.Address,
		GatewayEui:   "",
		Name:         hotspot.Name,
		AntennaIndex: 0,
		Time:         hotspot.Status.Timestamp.UnixNano(),
		//Timestamp:                   0,
		//FineTimestamp:               0,
		//FineTimestampEncrypted:      nil,
		//FineTimestampEncryptedKeyId: "",
		//ChannelIndex:                hotspot.Channel,
		//Rssi:                        hotspot.Rssi,
		//SignalRssi:                  0,
		//Snr:                         hotspot.Snr,
		Latitude:         hotspot.Latitude,
		Longitude:        hotspot.Longitude,
		Altitude:         hotspot.Elevation,
		LocationAccuracy: 0,
		LocationSource:   "",
	}

	// Last heard times can not be in the future. In that case use now.
	if hotspot.Status.Timestamp.After(time.Now()) {
		gateway.Time = time.Now().UnixNano()
	}

	// Only accept last heard times for online gateways
	if hotspot.Status.Online != "online" {
		gateway.Time = 0
	}

	gateway.Attributes = make(map[string]interface{}, 0)
	gateway.Attributes["mode"] = hotspot.Mode
	gateway.Attributes["timestamp_added"] = hotspot.TimestampAdded.UnixNano()
	gateway.Attributes["gain"] = hotspot.Gain
	// Add other fields as required

	return gateway, nil
}
