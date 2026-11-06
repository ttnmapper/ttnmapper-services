package tms

import (
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ttnmapper-tms/pkg/tms/types"
)

func TestCoverageBoundsAndAntimeridian(t *testing.T) {
	for _, query := range []string{"", "west=NaN&south=0&east=1&north=1", "west=0&south=2&east=1&north=1", "west=-181&south=0&east=1&north=1"} {
		w := httptest.NewRecorder()
		GetSimCovRasterManifest(w, httptest.NewRequest("GET", "/?"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("invalid query %q: status %d", query, w.Code)
		}
	}
	bounds, err := parseCoverageBounds(httptest.NewRequest("GET", "/?west=179&south=-1&east=-179&north=1", nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		lon  float64
		want bool
	}{{179.5, true}, {-179.5, true}, {0, false}} {
		if got := gatewayIntersectsCoverageBounds(0, tt.lon, bounds, 1000); got != tt.want {
			t.Errorf("longitude %v: %v", tt.lon, got)
		}
	}
	if !gatewayIntersectsCoverageBounds(0, -179.9, coverageBounds{179, -1, 179.8, 1}, 50000) {
		t.Fatal("gateway near the opposite side of the dateline was excluded")
	}
}

func TestCoverageManifestUsesOptimizedLocationAndVersion(t *testing.T) {
	original := MyConfiguration
	t.Cleanup(func() { MyConfiguration = original })
	MyConfiguration.GatewayParamsFile = filepath.Join(t.TempDir(), "params.json")
	if err := os.WriteFile(MyConfiguration.GatewayParamsFile, []byte(`{"net":{"moved":{"lat":52,"lon":4,"height_agl":5}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	gateways := []types.Gateway{
		{GatewayId: "moved", Latitude: 1, Longitude: 1, LastHeard: time.Now()},
		{GatewayId: "offline", Latitude: 52, Longitude: 4, LastHeard: time.Now().AddDate(0, 0, -6)},
		{GatewayId: "far", Latitude: 1, Longitude: 1, LastHeard: time.Now()},
	}
	bounds := coverageBounds{3.9, 51.9, 4.1, 52.1}
	selected := selectCoverageRasters("net", gateways, bounds, "")
	if len(selected) != 1 || selected[0].GatewayID != "moved" {
		t.Fatalf("selected = %+v", selected)
	}
	MyConfiguration.SimCovRadiusKm++
	changed := selectCoverageRasters("net", gateways, bounds, "")
	if changed[0].URL == selected[0].URL {
		t.Fatal("simulation input change did not change raster URL")
	}
	if single := selectCoverageRasters("net", gateways, bounds, "offline"); len(single) != 1 {
		t.Fatalf("single offline gateway not available: %+v", single)
	}
}

func TestRasterOnlySimulationSharedWithLegacySamples(t *testing.T) {
	original, originalRunner := MyConfiguration, sharedSignalServerRunner
	t.Cleanup(func() {
		MyConfiguration, sharedSignalServerRunner = original, originalRunner
		signalServerRunnerOnce = sync.Once{}
		if originalRunner != nil {
			signalServerRunnerOnce.Do(func() {})
		}
	})
	dir := t.TempDir()
	MyConfiguration.CacheDirSimCov = filepath.Join(dir, "cache")
	MyConfiguration.SignalServerSrtmDir = ""
	MyConfiguration.GatewayParamsFile = ""
	MyConfiguration.SignalServerBin = filepath.Join(dir, "signalserver")
	signalServerRunnerOnce = sync.Once{}
	fixture := filepath.Join(dir, "fixture.tif")
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{190, 190, 190, 255})
		}
	}
	if err := os.WriteFile(fixture, testGeoTIFF(t, img, 4, 52, .0005, .0005), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SIMCOV_FIXTURE", fixture)
	logPath := filepath.Join(dir, "calls")
	t.Setenv("SIMCOV_CALLS", logPath)
	script := "#!/bin/sh\nprintf 'call\\n' >> \"$SIMCOV_CALLS\"\nwhile [ $# -gt 0 ]; do\nif [ \"$1\" = -o ]; then shift; out=\"$1.tif\"; fi\nshift\ndone\ncp \"$SIMCOV_FIXTURE\" \"$out\"\n"
	if err := os.WriteFile(MyConfiguration.SignalServerBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	lock := gatewayEnsureLock("net\x00raw")
	lock.Lock()
	_, err := ensureGatewayRasterLocked("net", "raw", 52, 4, 0)
	lock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(simCovCacheFilePath("net", "raw")); !os.IsNotExist(err) {
		t.Fatal("raster delivery expanded samples")
	}
	ensureGatewaySimCov("net", "raw", 52, 4, 0)
	if samples, _, _, _, ok := LoadSimCovCache("net", "raw"); !ok || len(samples) == 0 {
		t.Fatal("legacy samples not created lazily")
	}
	data, err := os.ReadFile(logPath)
	if err != nil || strings.Count(string(data), "call\n") != 1 {
		t.Fatalf("simulation calls = %q, %v", data, err)
	}
	// A changed radius must rebuild the raster and invalidate old samples.
	MyConfiguration.SimCovRadiusKm++
	lock.Lock()
	_, err = ensureGatewayRasterLocked("net", "raw", 52, 4, 0)
	lock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, ok := LoadSimCovCache("net", "raw"); ok {
		t.Fatal("old samples survived raster replacement")
	}
}

func TestRasterHTTPRangeAndConditionalRequests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverage.tif")
	if err := os.WriteFile(path, []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	metadata := rasterMetadata{Size: info.Size(), Modified: info.ModTime()}
	r := httptest.NewRequest("GET", "/coverage", nil)
	r.Header.Set("Range", "bytes=2-5")
	w := httptest.NewRecorder()
	serveSimCovRaster(w, r, metadata, f)
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" || w.Header().Get("Content-Type") != "image/tiff" {
		t.Fatalf("range response: %d %q", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/coverage", nil)
	r.Header.Set("If-None-Match", w.Header().Get("ETag"))
	w = httptest.NewRecorder()
	serveSimCovRaster(w, r, metadata, f)
	if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("conditional response: %d", w.Code)
	}
}
