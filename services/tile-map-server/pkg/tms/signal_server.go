package tms

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"ttnmapper-tms/pkg/tms/types"

	"golang.org/x/image/tiff"
)

// latLonToZ19Tile converts geographic coordinates to a z19 slippy-map tile index.
func LatLonToZ19Tile(lat, lon float64) (tileX, tileY int) {
	const z = 19
	n := math.Pow(2, float64(z))
	tileX = int(math.Floor((lon + 180.0) / 360.0 * n))
	latRad := lat * math.Pi / 180.0
	tileY = int(math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n))
	return tileX, tileY
}

// signalDbmToBucketIndex maps a received signal strength (dBm) to the 0-12 bucket
// index used by the existing measured-coverage pipeline.
//
//	0  → > -100 dBm  (BucketHigh)
//	1  → -100 to -105
//	2  → -105 to -110
//	3  → -110 to -115
//	4  → -115 to -120
//	5  → -120 to -125
//	6  → -125 to -130
//	7  → -130 to -135
//	8  → -135 to -140
//	9  → -140 to -145
//	10 → -145 to -150
//	11 → < -150 dBm  (BucketLow)
//	12 → no signal
func signalDbmToBucketIndex(dbm float64) int {
	switch {
	case dbm > -100:
		return 0
	case dbm > -105:
		return 1
	case dbm > -110:
		return 2
	case dbm > -115:
		return 3
	case dbm > -120:
		return 4
	case dbm > -125:
		return 5
	case dbm > -130:
		return 6
	case dbm > -135:
		return 7
	case dbm > -140:
		return 8
	case dbm > -145:
		return 9
	case dbm > -150:
		return 10
	case dbm > -300:
		return 11
	default:
		return 12
	}
}

// copernicusDEMBaseURL is the public AWS S3 HTTPS endpoint for Copernicus GLO-30 DEM tiles.
// The bucket stores tiles under the COG_10 prefix; Signal-Server expects COG_30 filenames
// for 1200 ppd mode, so we download the COG_10 file and save it with the COG_30 name.
const copernicusDEMBaseURL = "https://copernicus-dem-30m.s3.amazonaws.com"

// ensureCopernicusDEMTiles downloads any missing Copernicus GLO-30 DEM tiles that cover
// the bounding box defined by the gateway position and simulation radius.
// Tiles are stored in demDir with the filename pattern expected by Signal-Server:
//
//	Copernicus_DSM_COG_30_N{lat}_00_{E|W}{lon}_00_DEM.tif
//
// Ocean tiles (HTTP 404) are replaced with a flat zero-elevation (sea level) tile
// so Signal-Server loads them rather than leaving the region uninitialised.  A tile
// that is never loaded leaves Signal-Server's per-pixel signal buffer at 0, which its
// -dbm renderer draws as maximum received power (a spurious solid-red rectangle over
// open water).  Loading a real 0 m tile initialises that region to "no coverage"
// instead, so uncovered sea renders transparent.
func EnsureCopernicusDEMTiles(demDir string, gatewayLat, gatewayLon float64, radiusKm int) error {
	return ensureCopernicusDEMTiles(demDir, gatewayLat, gatewayLon, radiusKm)
}

func ensureCopernicusDEMTiles(demDir string, gatewayLat, gatewayLon float64, radiusKm int) error {
	if demDir == "" {
		return nil
	}
	if err := os.MkdirAll(demDir, 0755); err != nil {
		return fmt.Errorf("ensureCopernicusDEMTiles: mkdir %s: %w", demDir, err)
	}

	// Compute bounding box in degrees (1° ≈ 111 km).
	latDeg := float64(radiusKm) / 111.0
	lonDeg := float64(radiusKm) / (111.0 * math.Cos(gatewayLat*math.Pi/180.0))

	latMin := int(math.Floor(gatewayLat - latDeg))
	latMax := int(math.Floor(gatewayLat + latDeg))
	lonMin := int(math.Floor(gatewayLon - lonDeg))
	lonMax := int(math.Floor(gatewayLon + lonDeg))

	for latTile := latMin; latTile <= latMax; latTile++ {
		for lonTile := lonMin; lonTile <= lonMax; lonTile++ {
			if err := downloadCopernicusTile(demDir, latTile, lonTile); err != nil {
				return err
			}
		}
	}
	return nil
}

// downloadCopernicusTileOnce downloads a single 1°×1° Copernicus DEM tile into demDir
// if it is not already present.  HTTP 404 responses (ocean / unreleased tiles) are
// replaced with a flat zero-elevation (sea-level) tile via writeFlatDEMTile.
func downloadCopernicusTileOnce(demDir string, latTile, lonTile int) error {
	// Build the hemisphere prefixes.
	latPfx := "N"
	if latTile < 0 {
		latPfx = "S"
		latTile = -latTile
	}
	lonPfx := "E"
	if lonTile < 0 {
		lonPfx = "W"
		lonTile = -lonTile
	}

	// Signal-Server expects COG_30 filenames for 1200 ppd.
	baseName := fmt.Sprintf("Copernicus_DSM_COG_30_%s%02d_00_%s%03d_00_DEM", latPfx, latTile, lonPfx, lonTile)
	destPath := filepath.Join(demDir, baseName+".tif")

	// Skip if already downloaded.
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	// The S3 bucket stores tiles under the COG_10 subdirectory/filename.
	srcName := fmt.Sprintf("Copernicus_DSM_COG_10_%s%02d_00_%s%03d_00_DEM", latPfx, latTile, lonPfx, lonTile)
	url := fmt.Sprintf("%s/%s/%s.tif", copernicusDEMBaseURL, srcName, srcName)

	log.Printf("dem: downloading %s", url)
	resp, err := demHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("dem: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Ocean tile or tile not released.  Write a flat zero-elevation tile so
		// Signal-Server loads the region (initialising it to "no coverage") rather
		// than leaving it uninitialised, which renders as a spurious solid-red block.
		log.Printf("dem: tile %s not found (ocean/unreleased), substituting flat sea-level tile", baseName)
		if err := writeFlatDEMTile(destPath); err != nil {
			return fmt.Errorf("dem: write flat tile %s: %w", destPath, err)
		}
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dem: unexpected HTTP %d for %s", resp.StatusCode, url)
	}

	// Write to a temp file first, then rename atomically.
	tmp, err := os.CreateTemp(demDir, ".dem-download-*")
	if err != nil {
		return fmt.Errorf("dem: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { os.Remove(tmpName) }() // no-op after successful rename

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("dem: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("dem: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("dem: rename to %s: %w", destPath, err)
	}
	log.Printf("dem: saved %s", destPath)
	return nil
}

// writeFlatDEMTile writes a small zero-elevation GeoTIFF to destPath, used in place
// of Copernicus tiles that are unavailable (HTTP 404, typically open ocean).
// Signal-Server georeferences DEM tiles purely from the filename and resamples the
// raster to its working resolution, so the tile only needs to be a GDAL-readable
// single-band raster of zeros — its size and geo-tags are irrelevant.
func writeFlatDEMTile(destPath string) error {
	const size = 64
	img := image.NewGray16(image.Rect(0, 0, size, size)) // zero-valued = 0 m elevation

	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".dem-flat-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { os.Remove(tmpName) }() // no-op after successful rename

	if err := tiff.Encode(tmp, img, &tiff.Options{Compression: tiff.Deflate}); err != nil {
		tmp.Close()
		return fmt.Errorf("encode tiff %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("rename to %s: %w", destPath, err)
	}
	return nil
}

func HasLineOfSight(lat1, lon1, alt1, lat2, lon2, alt2 float64) (bool, error) {
	tmpDir, err := os.MkdirTemp("", "los-*")
	if err != nil {
		return false, fmt.Errorf("HasLineOfSight: mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outBase := filepath.Join(tmpDir, "los")
	lat1Str := strconv.FormatFloat(lat1, 'f', 6, 64)
	lon1Str := strconv.FormatFloat(lon1, 'f', 6, 64)
	alt1Str := strconv.FormatFloat(alt1, 'f', 2, 64)

	lat2Str := strconv.FormatFloat(lat2, 'f', 6, 64)
	lon2Str := strconv.FormatFloat(lon2, 'f', 6, 64)
	alt2Str := strconv.FormatFloat(alt2, 'f', 2, 64)

	// We use path-profile mode (-pm 1) or just check for obstruction.
	// Signal-Server's path profile output includes LOS information.
	args := []string{
		"-lat", lat1Str, "-lon", lon1Str, "-txh", alt1Str,
		"-rla", lat2Str, "-rlo", lon2Str, "-rxh", alt2Str,
		"-f", "868", "-erp", "0.0250", "-dbm",
		"-pm", "1", "-rel", "50", "-conf", "50",
		"-o", outBase,
		"-dem", MyConfiguration.SignalServerSrtmDir,
	}

	if out, err := runSignalServer(args); err != nil {
		return false, fmt.Errorf("HasLineOfSight: signal server: %w; output: %s", err, string(out))
	}

	// Parse the report file for LOS.
	reportPath := outBase + ".txt"
	rf, err := os.Open(reportPath)
	if err != nil {
		return false, fmt.Errorf("HasLineOfSight: open report %s: %w", reportPath, err)
	}
	defer rf.Close()

	scanner := bufio.NewScanner(rf)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Line-of-sight is") {
			if strings.Contains(line, "Line-of-sight is blocked") {
				return false, nil
			}
			return true, nil
		}
	}
	return false, fmt.Errorf("HasLineOfSight: LOS status not found in %s", reportPath)
}

func GetTerrainElevation(lat, lon float64) (float64, error) {
	return queryTerrainElevation(lat, lon, MyConfiguration.SignalServerSrtmDir)
}

// queryTerrainElevation runs Signal-Server in path-profile mode with TX and RX
// both set to the gateway position, then parses the "Ground elevation: X.XX meters AMSL"
// line from the report file.  This is the most reliable way to obtain the DEM
// elevation at a point because Signal-Server handles all TIFF decoding internally.
func queryTerrainElevationUncached(lat, lon float64, demDir string) (float64, error) {
	tmpDir, err := os.MkdirTemp("", "elev-*")
	if err != nil {
		return 0, fmt.Errorf("queryTerrainElevation: mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outBase := filepath.Join(tmpDir, "elev")
	latStr := strconv.FormatFloat(lat, 'f', 6, 64)
	lonStr := strconv.FormatFloat(lon, 'f', 6, 64)

	args := []string{
		"-lat", latStr, "-lon", lonStr, "-txh", "0",
		"-rla", latStr, "-rlo", lonStr, "-rxh", "0",
		"-f", "868", "-erp", "0.0250", "-dbm",
		"-pm", "1", "-rel", "50", "-conf", "50",
		"-o", outBase,
		"-dem", demDir,
	}

	if out, err := runSignalServer(args); err != nil {
		return 0, fmt.Errorf("queryTerrainElevation: signal server: %w; output: %s", err, string(out))
	}

	// Parse "Ground elevation: X.XX meters AMSL" from the report file.
	reportPath := outBase + ".txt"
	rf, err := os.Open(reportPath)
	if err != nil {
		return 0, fmt.Errorf("queryTerrainElevation: open report %s: %w", reportPath, err)
	}
	defer rf.Close()

	scanner := bufio.NewScanner(rf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Only use the Transmitter site block (first occurrence).
		if strings.HasPrefix(line, "Ground elevation:") {
			// Format: "Ground elevation: 5.96 meters AMSL"
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				elev, err := strconv.ParseFloat(fields[2], 64)
				if err == nil {
					return elev, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("queryTerrainElevation: ground elevation not found in %s", reportPath)
}

// signalServerScaleMin defines the dBm threshold passed to Signal-Server via -rt.
// Pixels with received power below this value are transparent (no coverage).
const signalServerScaleMin = -150.0
const signalServerScaleMax = 0.0

// loraErpWatts is the Effective Radiated Power for a LoRa gateway.
// TX power: 14 dBm = 25.12 mW; antenna: 2.15 dBi dipole ≈ 0 dBd → ERP ≈ 0.025 W.
const loraErpWatts = 0.025

// defaultGatewayAGL is the assumed antenna height above ground level (AGL) in metres
// used when running Signal-Server with a DEM.  Signal-Server's -txh flag expects AGL,
// not altitude above sea level (ASL).  The gateway's ASL altitude stored in the
// database cannot be used directly because Signal-Server would add it on top of the
// terrain elevation from the DEM, placing the antenna far too high and producing
// unrealistically wide, flat-earth-like coverage.
const defaultGatewayAGL = 2

// RunSignalServerSimulation invokes the Signal-Server binary to compute an area
// coverage simulation centred on the gateway, then returns the results as a
// slice of z19 samples using the same bucket-index scale as the measured-
// coverage pipeline.
//
// Parameters match a LoRa gateway: 868 MHz, 14 dBm TX, 2.15 dBi dipole antennas.
// Signal-Server is run with:
//   - -pm 1         → ITM (Longley-Rice) propagation model
//   - -color gray   → grayscale output; tinycolormap gray is INVERTED (0→white, 1→black)
//   - -dbm          → received power in dBm
//   - -rt -160      → threshold: pixels below -160 dBm are transparent (alpha=0)
//   - -geotiff      → output a GeoTIFF file
//
// The GeoTIFF is decoded to extract per-pixel geographic coordinates and
// signal values, which are then mapped to z19 tile indices and bucket indices.
//
// gatewayAltM is the gateway altitude above sea level (ASL) as stored in the
// database.  Signal-Server's -txh flag expects height above ground level (AGL).
// If a DEM is configured, the terrain elevation at the gateway position is read
// from the downloaded tile and subtracted from gatewayAltM to obtain AGL.
// If the DEM lookup fails, or the result is unreasonably small, defaultGatewayAGL
// is used as a safe fallback.
func RunSignalServerSimulation(networkID, gatewayID string, gatewayLat, gatewayLon float64, gatewayAltM, radiusKm int) ([]types.Sample, string, error) {
	params := getSimulationParams(gatewayLat, gatewayLon, networkID, gatewayID, gatewayAltM)
	return RunSignalServerSimulationWithParams(networkID, gatewayID, gatewayLat, gatewayLon, gatewayAltM, radiusKm, params)
}

func RunSignalServerSimulationWithParams(networkID, gatewayID string, gatewayLat, gatewayLon float64, gatewayAltM, radiusKm int, params types.SimulationParams) ([]types.Sample, string, error) {
	runner := getSignalServerRunner()
	runner.simulations <- struct{}{}
	defer func() { <-runner.simulations }()
	tifPath, err := runSignalServerRaster(networkID, gatewayID, gatewayLat, gatewayLon, gatewayAltM, radiusKm, params)
	if err != nil {
		return nil, "", err
	}
	samples, err := parseGeoTIFF(tifPath)
	if err != nil {
		os.RemoveAll(filepath.Dir(tifPath))
		return nil, "", fmt.Errorf("parse simulation: %w", err)
	}
	return samples, tifPath, nil
}

// Raster delivery never expands the result into zoom-19 sample cells.
func runSignalServerRasterWithParams(networkID, gatewayID string, gatewayLat, gatewayLon float64, gatewayAltM, radiusKm int, params types.SimulationParams) (string, error) {
	runner := getSignalServerRunner()
	runner.simulations <- struct{}{}
	defer func() { <-runner.simulations }()
	return runSignalServerRaster(networkID, gatewayID, gatewayLat, gatewayLon, gatewayAltM, radiusKm, params)
}

func runSignalServerRaster(networkID, gatewayID string, gatewayLat, gatewayLon float64, gatewayAltM, radiusKm int, params types.SimulationParams) (string, error) {
	if MyConfiguration.SignalServerBin == "" {
		return "", fmt.Errorf("SignalServerBin is not configured")
	}

	// Download any missing Copernicus DEM tiles for the simulation area.
	if MyConfiguration.SignalServerSrtmDir != "" {
		if err := ensureCopernicusDEMTiles(MyConfiguration.SignalServerSrtmDir, gatewayLat, gatewayLon, radiusKm); err != nil {
			log.Printf("SimCov: DEM tile download error (simulation will use flat earth): %v", err)
		}
	}

	// Create a temporary working directory; cleaned up after parsing.
	tmpDir, err := os.MkdirTemp("", "simcov-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	// The caller owns the temporary directory after successful generation.

	outBase := filepath.Join(tmpDir, "simcov")

	simLat := gatewayLat
	simLon := gatewayLon
	if params.Lat != 0 && params.Lon != 0 {
		simLat = params.Lat
		simLon = params.Lon
	}

	// Compute gateway height above ground level (AGL) from the DEM.
	// Signal-Server adds -txh on top of the terrain elevation, so we must pass AGL.
	gatewayAGL := int(math.Round(params.HeightAgl))

	if gatewayAGL < 1 {
		gatewayAGL = 1
	}

	args := []string{
		"-lat", strconv.FormatFloat(simLat, 'f', 6, 64),
		"-lon", strconv.FormatFloat(simLon, 'f', 6, 64),
		"-txh", strconv.Itoa(gatewayAGL), // AGL height derived from ASL minus DEM terrain elevation
		"-rxh", "1", // receiver height 1m above ground
		"-rxg", strconv.FormatFloat(-params.ConstantLoss, 'f', 2, 64), // constant loss modeled as negative receiver gain
		"-f", "868", // LoRa EU frequency in MHz
		"-erp", strconv.FormatFloat(loraErpWatts, 'f', 4, 64), // ERP in watts
		"-dbm",                                         // output in dBm
		"-rt", strconv.Itoa(int(signalServerScaleMin)), // coverage threshold
		"-R", strconv.Itoa(radiusKm),
		"-pm", "1", // ITM (Longley-Rice) propagation model
		"-rel", strconv.FormatFloat(params.Reliability, 'f', 1, 64), // reliability %
		"-conf", "90", // confidence %
		"-cl", strconv.Itoa(params.Climate),
		"-gc", strconv.FormatFloat(params.Clutter, 'f', 2, 64),
		"-tercon", strconv.FormatFloat(params.Conductivity, 'f', 4, 64),
		"-terdic", strconv.FormatFloat(params.Dielectric, 'f', 2, 64),
		"-color", "gray",
		"-geotiff",
		"-o", outBase,
	}

	if MyConfiguration.SignalServerSrtmDir != "" {
		args = append(args, "-dem", MyConfiguration.SignalServerSrtmDir)
	}

	out, err := runSignalServer(args)
	if err != nil {
		os.RemoveAll(tmpDir)
		log.Printf("SimCov: %s/%s signal server failed: %v; output: %s", networkID, gatewayID, err, string(out))
		return "", fmt.Errorf("signal server failed: %w; output: %s", err, string(out))
	}
	// log.Printf("signal_server: %s", string(out)) // Silenced verbose output

	// Signal-Server writes <outBase>.tif
	tifPath := outBase + ".tif"
	if _, _, _, _, _, _, err := readGeoTIFFExtent(tifPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("invalid simulation raster: %w", err)
	}
	return tifPath, nil
}

// parseGeoTIFF reads the GeoTIFF produced by Signal-Server and converts each
// non-transparent pixel into a types.Sample keyed to a z19 tile index.
//
// The GeoTIFF is a grayscale-palette RGBA image.  The alpha channel is non-zero
// where signal coverage exists.  Signal-Server uses tinycolormap's gray palette
// which is INVERTED: R=255 (white) → weakest signal, R=0 (black) → strongest.
// So the normalised strength t = 1.0 - R/255.
func parseGeoTIFF(path string) ([]types.Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	// golang.org/x/image/tiff decodes the pixel data for us.
	img, err := tiff.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode tiff: %w", err)
	}

	// Read GeoTIFF geographic tags from the raw TIFF to get the bounding box.
	topLeftLon, topLeftLat, lonPerPixel, latPerPixel, width, height, geoErr := readGeoTIFFExtent(path)
	if geoErr != nil {
		return nil, fmt.Errorf("read geotiff extent: %w", geoErr)
	}

	bounds := img.Bounds()
	// Use decoded image dimensions if readGeoTIFFExtent failed to find tags 256/257
	if width == 0 {
		width = bounds.Dx()
	}
	if height == 0 {
		height = bounds.Dy()
	}

	// Raster axes are independent. Avoid Mercator projection for every pixel.
	xMins, xMaxs := make([]int, width), make([]int, width)
	yMins, yMaxs := make([]int, height), make([]int, height)
	for col := range width {
		xMins[col], _ = LatLonToZ19Tile(topLeftLat, topLeftLon+float64(col)*lonPerPixel)
		xMaxs[col], _ = LatLonToZ19Tile(topLeftLat, topLeftLon+float64(col+1)*lonPerPixel)
		if xMins[col] > xMaxs[col] {
			xMins[col], xMaxs[col] = xMaxs[col], xMins[col]
		}
	}
	for row := range height {
		_, yMins[row] = LatLonToZ19Tile(topLeftLat-float64(row)*latPerPixel, topLeftLon)
		_, yMaxs[row] = LatLonToZ19Tile(topLeftLat-float64(row+1)*latPerPixel, topLeftLon)
		if yMins[row] > yMaxs[row] {
			yMins[row], yMaxs[row] = yMaxs[row], yMins[row]
		}
	}

	// best[tileX][tileY] = best bucket index seen for that tile
	type cellKey struct{ x, y int }
	best := make(map[cellKey]int)

	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			r, _, _, a := img.At(bounds.Min.X+col, bounds.Min.Y+row).RGBA()
			if a == 0 || r == 0 {
				// No coverage at this pixel, or fill/no-data sentinel (R=0).
				continue
			}

			// tinycolormap gray is INVERTED: R=255→weakest, R=0→strongest.
			// Invert so t=0.0 (weakest) → t=1.0 (strongest).
			t := 1.0 - float64(r>>8)/255.0

			// Recover dBm: scale goes from signalServerScaleMin to signalServerScaleMax (0.0).
			dbm := signalServerScaleMin + t*(signalServerScaleMax-signalServerScaleMin)

			bucketIdx := signalDbmToBucketIndex(dbm)

			// Preserve the original inclusive z19 cell extent for this pixel.
			tileXMin, tileXMax := xMins[col], xMaxs[col]
			tileYMin, tileYMax := yMins[row], yMaxs[row]

			for tx := tileXMin; tx <= tileXMax; tx++ {
				for ty := tileYMin; ty <= tileYMax; ty++ {
					key := cellKey{tx, ty}
					if existing, ok := best[key]; !ok || bucketIdx < existing {
						best[key] = bucketIdx
					}
				}
			}
		}
	}

	samples := make([]types.Sample, 0, len(best))
	for key, bucketIdx := range best {
		samples = append(samples, types.Sample{X: key.x, Y: key.y, MaxBucketIndex: bucketIdx})
	}

	// log.Printf("signal_server: parsed %d z19 samples from geotiff (%dx%d px)", len(samples), width, height)
	return samples, nil
}

// readGeoTIFFExtent reads the ModelPixelScaleTag (33550) and ModelTiepointTag
// (33922) from the raw TIFF byte stream to determine the geographic bounding box.
// Returns (topLeftLon, topLeftLat, lonPerPixel, latPerPixel).
// RenderSimCovTileFromGeoTIFF renders a TMS tile (x, y, z) by sampling the GeoTIFF produced
// by Signal-Server. It uses the same color mapping as the measured-coverage heatmap.
func RenderSimCovTileFromGeoTIFF(tifPath string, x, y, z int) (image.Image, error) {
	// 1. Read GeoTIFF extent.
	topLeftLon, topLeftLat, lonPerPixel, latPerPixel, width, height, err := readGeoTIFFExtent(tifPath)
	if err != nil {
		return nil, fmt.Errorf("read extent: %w", err)
	}

	// 2. Open and decode GeoTIFF image.
	f, err := os.Open(tifPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tifImg, err := tiff.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode tiff: %w", err)
	}

	bounds := tifImg.Bounds()
	if width == 0 {
		width = bounds.Dx()
	}
	if height == 0 {
		height = bounds.Dy()
	}

	// 3. Create destination tile.
	tile := image.NewNRGBA(image.Rect(0, 0, 256, 256))

	// 4. Precompute some constants for coordinate transform.
	n := math.Pow(2, float64(z))

	// Scale constants from Signal-Server's grayscale GeoTIFF.
	scaleMin := signalServerScaleMin
	scaleMax := signalServerScaleMax
	scaleRange := scaleMax - scaleMin

	// Coordinate axes are independent: project each column and row once.
	var imgXs [256]int
	for px := range 256 {
		lon := (float64(x)+float64(px)/256.0)/n*360.0 - 180.0
		imgXs[px] = int(math.Round((lon - topLeftLon) / lonPerPixel))
	}
	for py := range 256 {
		latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*(float64(y)+float64(py)/256.0)/n)))
		lat := latRad * 180.0 / math.Pi
		imgY := int(math.Round((topLeftLat - lat) / latPerPixel))
		for px := range 256 {
			imgX := imgXs[px]
			if imgX < 0 || imgX >= width || imgY < 0 || imgY >= height {
				continue
			}

			// Sample GeoTIFF.
			r, _, _, a := tifImg.At(bounds.Min.X+imgX, bounds.Min.Y+imgY).RGBA()
			if a == 0 {
				continue
			}

			// r is [0, 65535]. Signal-Server output is 8-bit, so we use r >> 8.
			val8 := uint8(r >> 8)

			// Signal-Server uses tinycolormap::Gray which is INVERTED: 0 is black (strongest), 255 is white (weakest).
			// t = 1.0 - GrayValue/255.0
			t := 1.0 - float64(val8)/255.0

			// Signal strength in dBm.
			dbm := t*scaleRange + scaleMin

			// Map dBm to bucket index and then to color.
			tile.Set(px, py, signalDbmToColor(dbm))
		}
	}

	return tile, nil
}

func signalDbmToColor(dbm float64) color.Color {
	bucketIdx := signalDbmToBucketIndex(dbm)

	var cr, cg, cb float64
	if bucketIdx == 12 {
		return color.Transparent
	} else if bucketIdx >= 5 { // 11..5
		cr, cg, cb = 0, 0, 1
	} else if bucketIdx == 4 {
		cr, cg, cb = 0, 1, 1
	} else if bucketIdx == 3 {
		cr, cg, cb = 0, 1, 0
	} else if bucketIdx == 2 {
		cr, cg, cb = 1, 1, 0
	} else if bucketIdx == 1 {
		cr, cg, cb = 1, 0.5, 0
	} else {
		cr, cg, cb = 1, 0, 0
	}

	return color.NRGBA{
		R: uint8(cr * 255),
		G: uint8(cg * 255),
		B: uint8(cb * 255),
		A: 255,
	}
}

type geoTIFFSource struct {
	path                                             string
	img                                              image.Image
	bounds                                           image.Rectangle
	topLeftLon, topLeftLat, lonPerPixel, latPerPixel float64
	minLon, maxLon, minLat, maxLat                   float64
}

// RenderSimCovNetworkTileFromGeoTIFFs renders a network-wide simulation tile by combining
// multiple GeoTIFFs, always taking the strongest signal for each pixel.
func RenderSimCovNetworkTileFromGeoTIFFs(tifPaths []string, x, y, z int) (image.Image, error) {
	if len(tifPaths) == 0 {
		return transparentTile(), nil
	}

	// 0. Calculate tile boundaries.
	n := math.Pow(2, float64(z))
	tileMinLon := float64(x)/n*360.0 - 180.0
	tileMaxLon := float64(x+1)/n*360.0 - 180.0
	tileMaxLatRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	tileMinLatRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y+1)/n)))
	tileMaxLat := tileMaxLatRad * 180.0 / math.Pi
	tileMinLat := tileMinLatRad * 180.0 / math.Pi

	// 1. Prepare and filter GeoTIFF sources.
	sources := make([]geoTIFFSource, 0, len(tifPaths))
	for _, path := range tifPaths {
		lon, lat, lpp, latpp, width, height, err := readGeoTIFFExtent(path)
		if err != nil {
			log.Printf("SimCov network: failed to read extent for %s: %v", path, err)
			continue
		}

		minLon := lon
		maxLon := lon + float64(width)*lpp
		maxLat := lat
		minLat := lat - float64(height)*latpp

		// Check if GeoTIFF overlaps with the tile.
		if maxLon < tileMinLon || minLon > tileMaxLon || maxLat < tileMinLat || minLat > tileMaxLat {
			continue
		}

		sources = append(sources, geoTIFFSource{
			path:        path,
			topLeftLon:  lon,
			topLeftLat:  lat,
			lonPerPixel: lpp,
			latPerPixel: latpp,
			minLon:      minLon,
			maxLon:      maxLon,
			minLat:      minLat,
			maxLat:      maxLat,
		})
	}

	if len(sources) == 0 {
		return transparentTile(), nil
	}

	// 2. Use a dBm buffer to store the strongest signal per pixel.
	// Initialize with a value below signalServerScaleMin.
	dbmBuffer := make([]float64, 256*256)
	for i := range dbmBuffer {
		dbmBuffer[i] = -1000.0
	}

	scaleMin := signalServerScaleMin
	scaleMax := signalServerScaleMax
	scaleRange := scaleMax - scaleMin

	// Merge the strongest signal source-by-source, retaining one decoded raster.

	var tileLats [256]float64
	for py := range 256 {
		tileLats[py] = math.Atan(math.Sinh(math.Pi*(1-2*(float64(y)+float64(py)/256.0)/n))) * 180.0 / math.Pi
	}
	for _, s := range sources {
		f, err := os.Open(s.path)
		if err != nil {
			return nil, err
		}
		s.img, err = tiff.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("decode tiff: %w", err)
		}
		s.bounds = s.img.Bounds()
		// Optimization: calculate pixel bounds.
		pxStart := int(math.Floor(((s.minLon+180.0)*n/360.0 - float64(x)) * 256.0))
		pxEnd := int(math.Ceil(((s.maxLon+180.0)*n/360.0 - float64(x)) * 256.0))
		if pxStart < 0 {
			pxStart = 0
		}
		if pxEnd > 256 {
			pxEnd = 256
		}

		// Pre-calculate imgX step.
		imgXStep := ((1.0 / 256.0) / n * 360.0) / s.lonPerPixel

		for py := range 256 {
			lat := tileLats[py]

			if lat < s.minLat || lat > s.maxLat {
				continue
			}

			var imgY int
			if gray, ok := s.img.(*image.Gray); ok {
				imgY = int(math.Round((s.topLeftLat - lat) / s.latPerPixel))
				if imgY < 0 || imgY >= s.bounds.Dy() {
					continue
				}

				lonStart := (float64(x)+float64(pxStart)/256.0)/n*360.0 - 180.0
				imgXFloat := (lonStart - s.topLeftLon) / s.lonPerPixel

				for px := pxStart; px < pxEnd; px++ {
					imgX := int(math.Round(imgXFloat))
					imgXFloat += imgXStep

					if imgX < 0 || imgX >= s.bounds.Dx() {
						continue
					}

					val8 := gray.GrayAt(s.bounds.Min.X+imgX, s.bounds.Min.Y+imgY).Y
					if val8 == 255 { // Assuming 255 is transparent/weakest background
						continue
					}

					t := 1.0 - float64(val8)/255.0
					dbm := t*scaleRange + scaleMin

					idx := py*256 + px
					if dbm > dbmBuffer[idx] {
						dbmBuffer[idx] = dbm
					}
				}
			} else {
				imgY = int(math.Round((s.topLeftLat - lat) / s.latPerPixel))
				if imgY < 0 || imgY >= s.bounds.Dy() {
					continue
				}

				lonStart := (float64(x)+float64(pxStart)/256.0)/n*360.0 - 180.0
				imgXFloat := (lonStart - s.topLeftLon) / s.lonPerPixel

				for px := pxStart; px < pxEnd; px++ {
					imgX := int(math.Round(imgXFloat))
					imgXFloat += imgXStep

					if imgX < 0 || imgX >= s.bounds.Dx() {
						continue
					}

					r, _, _, a := s.img.At(s.bounds.Min.X+imgX, s.bounds.Min.Y+imgY).RGBA()
					if a == 0 {
						continue
					}

					val8 := uint8(r >> 8)
					t := 1.0 - float64(val8)/255.0
					dbm := t*scaleRange + scaleMin

					idx := py*256 + px
					if dbm > dbmBuffer[idx] {
						dbmBuffer[idx] = dbm
					}
				}
			}
		}
	}

	// 4. Create the final image from the dBm buffer.
	tile := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for py := range 256 {
		for px := range 256 {
			dbm := dbmBuffer[py*256+px]
			if dbm > -999.0 {
				tile.Set(px, py, signalDbmToColor(dbm))
			}
		}
	}

	return tile, nil
}

// readGeoTIFFExtent reads only the TIFF directory and georeferencing tags,
// rather than loading the entire raster just to discover its bounds.
func readGeoTIFFExtent(path string) (float64, float64, float64, float64, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	defer f.Close()
	return readGeoTIFFExtentFromReader(f)
}

func readGeoTIFFExtentFromReader(reader io.ReaderAt) (float64, float64, float64, float64, int, int, error) {
	fail := func(err error) (float64, float64, float64, float64, int, int, error) { return 0, 0, 0, 0, 0, 0, err }
	var header [8]byte
	if _, err := reader.ReadAt(header[:], 0); err != nil {
		return fail(err)
	}
	var order binary.ByteOrder
	switch string(header[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return fail(fmt.Errorf("invalid TIFF byte order"))
	}
	if order.Uint16(header[2:4]) != 42 {
		return fail(fmt.Errorf("not a classic TIFF"))
	}
	offset := int64(order.Uint32(header[4:8]))
	var count [2]byte
	if _, err := reader.ReadAt(count[:], offset); err != nil {
		return fail(err)
	}
	directory := make([]byte, int(order.Uint16(count[:]))*12)
	if _, err := reader.ReadAt(directory, offset+2); err != nil {
		return fail(err)
	}
	var width, height int
	var scaleOffset, tiepointOffset int64
	for pos := 0; pos < len(directory); pos += 12 {
		entry := directory[pos : pos+12]
		tag, kind := order.Uint16(entry[:2]), order.Uint16(entry[2:4])
		count := order.Uint32(entry[4:8])
		value := order.Uint32(entry[8:12])
		switch tag {
		case 256, 257:
			if count != 1 || (kind != 3 && kind != 4) {
				return fail(fmt.Errorf("unsupported TIFF dimension tag %d", tag))
			}
			if kind == 3 {
				value = uint32(order.Uint16(entry[8:10]))
			}
			if tag == 256 {
				width = int(value)
			} else {
				height = int(value)
			}
		case 33550:
			if kind != 12 || count < 3 {
				return fail(fmt.Errorf("invalid pixel scale tag"))
			}
			scaleOffset = int64(value)
		case 33922:
			if kind != 12 || count < 6 {
				return fail(fmt.Errorf("invalid tiepoint tag"))
			}
			tiepointOffset = int64(value)
		}
	}
	if scaleOffset == 0 || tiepointOffset == 0 {
		return fail(fmt.Errorf("missing GeoTIFF tags"))
	}
	var scale [24]byte
	var tiepoint [48]byte
	if _, err := reader.ReadAt(scale[:], scaleOffset); err != nil {
		return fail(err)
	}
	if _, err := reader.ReadAt(tiepoint[:], tiepointOffset); err != nil {
		return fail(err)
	}
	double := func(data []byte) float64 { return math.Float64frombits(order.Uint64(data)) }
	lonPerPixel, latPerPixel := double(scale[:8]), double(scale[8:16])
	lon := double(tiepoint[24:32]) - double(tiepoint[:8])*lonPerPixel
	lat := double(tiepoint[32:40]) + double(tiepoint[8:16])*latPerPixel
	if lonPerPixel <= 0 || latPerPixel <= 0 || math.IsNaN(lonPerPixel) || math.IsNaN(latPerPixel) ||
		math.IsInf(lonPerPixel, 0) || math.IsInf(latPerPixel, 0) || math.IsNaN(lon) || math.IsNaN(lat) || math.IsInf(lon, 0) || math.IsInf(lat, 0) {
		return fail(fmt.Errorf("invalid GeoTIFF georeferencing"))
	}
	return lon, lat, lonPerPixel, latPerPixel, width, height, nil
}

// transparentTile returns a 256×256 fully transparent image.Image.
func transparentTile() image.Image {
	return image.NewNRGBA(image.Rect(0, 0, 256, 256))
}

func getGatewayParams(networkID, gatewayID string, defaultLat, defaultLon float64, defaultAltM int) types.SimulationParams {
	return types.GetGatewayParams(MyConfiguration.GatewayParamsFile, networkID, gatewayID, defaultLat, defaultLon, defaultAltM)
}

func getSimulationParams(lat, lon float64, networkID, gatewayID string, defaultAltM int) types.SimulationParams {
	params := getGatewayParams(networkID, gatewayID, lat, lon, defaultAltM)

	// If the gateway was NOT in gateway_params.json (TriesDone == 0)
	// and we used defaultAltM (ASL) as a fallback, we must convert it to AGL now.
	if params.LocationTriesDone == 0 && params.RadioTriesDone == 0 && defaultAltM != 0 {
		if MyConfiguration.SignalServerSrtmDir != "" {
			if err := ensureCopernicusDEMTiles(MyConfiguration.SignalServerSrtmDir, lat, lon, 1); err == nil {
				if terrainElev, err := queryTerrainElevation(lat, lon, MyConfiguration.SignalServerSrtmDir); err == nil {
					agl := float64(defaultAltM) - terrainElev
					if agl < 1 {
						agl = 1
					}
					params.HeightAgl = agl
					// log.Printf("SimCov: converted DB ASL %dm to AGL %.1fm for %s/%s", defaultAltM, agl, networkID, gatewayID)
				}
			}
		}
	}

	return params
}
