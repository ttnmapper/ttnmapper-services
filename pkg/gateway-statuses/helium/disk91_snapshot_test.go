package helium

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/utils"
)

func TestFetchDisk91Snapshot(t *testing.T) {
	hotspots, err := FetchDisk91Snapshot()
	if err != nil {
		t.Fatalf(err.Error())
	}
	if len(hotspots) == 0 {
		log.Println("no data")
	}
	log.Printf("%d hotspots in snapshot", len(hotspots))

	for _, hotspot := range hotspots {
		gateway, err := Disk91SnapshotToTtnMapperGateway(hotspot)
		if err != nil {
			t.Fatalf(err.Error())
		}
		log.Println(utils.PrettyPrint(gateway))
	}
}
