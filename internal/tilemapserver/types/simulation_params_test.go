package types

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGatewayParamsCacheReloadsReplacedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "params.json")
	if err := os.WriteFile(path, []byte(`{"net":{"gw":{"height_agl":5,"lat":52,"lon":4}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			params := GetGatewayParams(path, "net", "gw", 1, 2, 3)
			if params.HeightAgl != 5 || params.Lat != 52 || params.Lon != 4 {
				t.Errorf("params = %+v", params)
			}
		}()
	}
	wg.Wait()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".new"
	if err := os.WriteFile(replacement, []byte(`{"net":{"gw":{"height_agl":7,"lat":52,"lon":4}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if params := GetGatewayParams(path, "net", "gw", 1, 2, 3); params.HeightAgl != 7 {
		t.Fatalf("stale params = %+v", params)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if params := GetGatewayParams(path, "net", "gw", 1, 2, 3); params.Lat != 1 || params.Lon != 2 {
		t.Fatalf("deleted params file still used: %+v", params)
	}
}
