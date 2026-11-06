package noc

import (
	"log"
	"testing"
	"ttnmapper-services/pkg/utils"
)

func TestFetchNocStatuses(t *testing.T) {
	gateways, err := FetchNocStatuses()
	if err != nil {
		t.Fatal(err)
	}
	log.Println(utils.PrettyPrint(gateways))
}
