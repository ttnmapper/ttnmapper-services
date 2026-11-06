package helium

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"testing"
	"ttnmapper-services/pkg/utils"
)

func TestFetchStatuses(t *testing.T) {
	cursor := ""
	for {
		response, err := FetchStatuses(cursor)
		if err != nil {
			t.Fatalf(err.Error())
		}

		//log.Println(utils.PrettyPrint(response))
		log.Printf("Found %d hotspots", len(response.Data))

		for _, hotspot := range response.Data {
			if hotspot.Status.Height != nil {
				log.Println(utils.PrettyPrint(hotspot))
			}
			//	//log.Println(utils.PrettyPrint(hotspot))
			//	ttnMapperGw, err := HeliumHotspotToTtnMapperGateway(hotspot)
			//	if err != nil {
			//		t.Fatalf(err.Error())
			//	}
			//	log.Println(utils.PrettyPrint(ttnMapperGw))
		}

		cursor = response.Cursor
		if cursor == "" {
			break
		}
	}
}

func TestParseHotspotJson(t *testing.T) {
	buf, err := ioutil.ReadFile("response.txt")
	if err != nil {
		t.Fatalf(err.Error())
	}

	var apiResponse HotspotApiResponse
	err = json.Unmarshal(buf, &apiResponse)
	if err != nil {
		t.Fatalf(err.Error())
	}

	for i, hotspot := range apiResponse.Data {
		log.Println(i)
		log.Println(utils.PrettyPrint(hotspot))
		gateway, _ := HeliumHotspotToTtnMapperGateway(hotspot)
		log.Println(utils.PrettyPrint(gateway))
	}
}
