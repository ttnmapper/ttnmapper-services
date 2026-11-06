package tms

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type demTestTransport func(*http.Request) (*http.Response, error)

func (transport demTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return transport(r)
}

func TestConcurrentDEMDownloadsShareOneRequest(t *testing.T) {
	originalClient := demHTTPClient
	t.Cleanup(func() { demHTTPClient = originalClient })
	var requests atomic.Int32
	demHTTPClient = &http.Client{Transport: demTestTransport(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		time.Sleep(20 * time.Millisecond)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("DEM data")), Header: make(http.Header)}, nil
	})}
	dir := t.TempDir()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := downloadCopernicusTile(dir, 52, 4); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 1 {
		t.Fatalf("HTTP requests = %d, want 1", requests.Load())
	}
	data, err := os.ReadFile(filepath.Join(dir, "Copernicus_DSM_COG_30_N52_00_E004_00_DEM.tif"))
	if err != nil || string(data) != "DEM data" {
		t.Fatalf("download = %q, %v", data, err)
	}
}

func TestTerrainElevationReuseAndDEMInvalidation(t *testing.T) {
	original, originalRunner := MyConfiguration, sharedSignalServerRunner
	t.Cleanup(func() {
		MyConfiguration, sharedSignalServerRunner = original, originalRunner
		signalServerRunnerOnce = sync.Once{}
		if originalRunner != nil {
			signalServerRunnerOnce.Do(func() {})
		}
	})
	dir := t.TempDir()
	bin := filepath.Join(dir, "signalserver")
	script := "#!/bin/sh\nprintf 'call\\n' >> \"$SIMCOV_TEST_CALLS\"\nwhile [ $# -gt 0 ]; do\nif [ \"$1\" = -o ]; then shift; report=\"$1.txt\"; fi\nshift\ndone\nsleep 0.02\nprintf 'Ground elevation: 25.00 meters AMSL\\n' > \"$report\"\n"
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	MyConfiguration.SignalServerBin = bin
	signalServerRunnerOnce = sync.Once{}
	logPath := filepath.Join(dir, "calls")
	t.Setenv("SIMCOV_TEST_CALLS", logPath)
	demPath := filepath.Join(dir, "Copernicus_DSM_COG_30_N52_00_E004_00_DEM.tif")
	if err := os.WriteFile(demPath, []byte("DEM"), 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if elevation, err := queryTerrainElevation(52.1, 4.1, dir); err != nil || elevation != 25 {
				t.Errorf("elevation = %v, %v", elevation, err)
			}
		}()
	}
	wg.Wait()
	if _, err := queryTerrainElevation(52.1, 4.1, dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil || strings.Count(string(data), "call\n") != 1 {
		t.Fatalf("calls = %q, %v", data, err)
	}
	changed := time.Now().Add(time.Second)
	if err := os.Chtimes(demPath, changed, changed); err != nil {
		t.Fatal(err)
	}
	if _, err := queryTerrainElevation(52.1, 4.1, dir); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(logPath)
	if err != nil || strings.Count(string(data), "call\n") != 2 {
		t.Fatalf("calls after DEM update = %q, %v", data, err)
	}
}
