package helium

import (
	"bufio"
	"compress/bzip2"
	"github.com/bserdar/jsonstream"
	"log"
	"net/http"
	"ttnmapper-services/pkg/types"
)

func FetchDisk91Snapshot() ([]Disk91Snapshot, error) {
	url := "http://etl-api.disk91.com/share/coveragemap.json.bz2"
	log.Println("Fetching Disk91 Snapshot", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	// create a reader
	br := bufio.NewReader(resp.Body)
	// create a bzip2.reader, using the reader we just created
	cr := bzip2.NewReader(br)

	ndLinesReader := jsonstream.NewLineReader(cr)

	var statuses []Disk91Snapshot
	for {
		var data Disk91Snapshot
		err := ndLinesReader.Unmarshal(&data)
		if err != nil {
			log.Println(err.Error())
			break
		}
		statuses = append(statuses, data)
	}
	log.Println("parsed done")

	_ = resp.Body.Close()

	return statuses, nil
}

func Disk91SnapshotToTtnMapperGateway(hotspot Disk91Snapshot) (types.TtnMapperGateway, error) {

	gateway := types.TtnMapperGateway{
		NetworkId:    "NS_HELIUM://000024",
		GatewayId:    hotspot.HotspotId,
		GatewayEui:   "",
		Name:         hotspot.AnimalName,
		AntennaIndex: 0,
		Time:         hotspot.LastSeen * 1000000, // json has millis unix timestamp, make in nanos
		//Timestamp:                   0,
		//FineTimestamp:               0,
		//FineTimestampEncrypted:      nil,
		//FineTimestampEncryptedKeyId: "",
		//ChannelIndex:                hotspot.Channel,
		//Rssi:                        hotspot.Rssi,
		//SignalRssi:                  0,
		//Snr:                         hotspot.Snr,
		Latitude:         hotspot.Position.Lat,
		Longitude:        hotspot.Position.Lng,
		Altitude:         int32(hotspot.Position.Alt),
		LocationAccuracy: 0,
		LocationSource:   "",
	}

	gateway.Attributes = make(map[string]interface{}, 0)
	gateway.Attributes["gain"] = hotspot.Position.Gain
	// Add other fields as required

	return gateway, nil
}
