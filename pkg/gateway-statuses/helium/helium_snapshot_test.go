package helium

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/utils"
)

func TestFetchSnapshot(t *testing.T) {
	//online := 0
	//offline := 0
	//other := 0

	snapshotTime, hotspots, err := FetchSnapshot()
	if err != nil {
		t.Fatalf(err.Error())
	}

	log.Printf("Found %d hotspots in snapshot", len(hotspots))
	for _, hotspot := range hotspots {
		if hotspot.Name == "cool-cerulean-poodle" {
			log.Println(utils.PrettyPrint(hotspot))
			gateway, err := HotspotSnapshotToTtnMapperGateway(snapshotTime, hotspot)
			if err != nil {
				t.Fatalf(err.Error())
			}
			log.Println(utils.PrettyPrint(gateway))
			//if hotspot.Online != "online" {
			//	log.Println(gateway.Time)
			//}
			//if hotspot.Online == "online" {
			//	online++
			//} else if hotspot.Online == "offline" {
			//	offline++
			//} else {
			//	other++
			//}
		}
	}
	//log.Println("Online", online)
	//log.Println("Offline", offline)
	//log.Println("Other", other)
}
