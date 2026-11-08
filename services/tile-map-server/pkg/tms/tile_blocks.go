package tms

import (
	"fmt"
	"github.com/fogleman/gg"
	"github.com/gorilla/mux"
	"image"
	"image/png"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"ttnmapper-services/services/tile-map-server/pkg/tms/types"
)

func GetBlocksTile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	networkId := vars["network_id"]
	gatewayId, singleGateway := vars["gateway_id"]

	// We've chosen to use mux.NewRouter().UseEncodedPath() which will return the path variables in encoded form.
	// This is necessary to correctly pass NS_TTS:// (two forward slashes).
	// Decode now.
	networkId, _ = url.QueryUnescape(networkId)
	gatewayId, _ = url.QueryUnescape(gatewayId)

	z, err := strconv.Atoi(vars["z"])
	if err != nil {
		fmt.Fprintln(w, "Z invalid")
		http.Error(w, "z invalid", http.StatusBadRequest)
		return
	}

	x, err := strconv.Atoi(vars["x"])
	if err != nil {
		fmt.Fprintln(w, "X invalid")
		http.Error(w, "x invalid", http.StatusBadRequest)
		return
	}

	if strings.HasSuffix(vars["y"], ".png") {
		vars["y"] = strings.TrimSuffix(vars["y"], ".png")
	}
	y, err := strconv.Atoi(vars["y"])
	if err != nil {
		fmt.Fprintln(w, "Y invalid")
		http.Error(w, "y invalid", http.StatusBadRequest)
		return
	}

	log.Printf("Blocks tile: %d/%d/%d\t", z, x, y)

	//tileFileName := fmt.Sprintf("%s/%d/%d/%d.png", MyConfiguration.CacheDirBlocks, z, x, y)
	//
	//tileInCacheOutdated := true
	//tileExistInCache := false
	//if file, err := os.Stat(tileFileName); err == nil {
	//	tileExistInCache = true
	//
	//	modifiedTime := file.ModTime()
	//
	//	// Check the last modified time of the file to see if the time is still new enough
	//	if modifiedTime.Add(GetCacheDurationForZoom(z)).After(time.Now()) {
	//		tileInCacheOutdated = false
	//	}
	//}
	//
	//// Only cache global tiles, not per gateway tiles
	//if MyConfiguration.CacheEnabled && tileExistInCache && !tileInCacheOutdated && !singleGateway {
	//	fmt.Printf("serving from cache\n")
	//	//promTmsBlocksCacheCount.Inc()
	//
	//	//Check if file exists and open
	//	Openfile, err := os.Open(tileFileName)
	//	defer Openfile.Close() //Close after function returns
	//	if err != nil {
	//		//File not found, send 404
	//		http.Error(w, "File not found.", 404)
	//		return
	//	}
	//
	//	io.Copy(w, Openfile) //'Copy' the file to the client
	//
	//} else {
	log.Printf("generating new tile\n")
	//promTmsBlocksCreateCount.Inc()
	//tileStart := time.Now()

	// Circles can overlap tiles, so select data for 1 tile buffer on all sides
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(x, y, z, 0)

	//log.Println("Selecting data")
	var samples []types.Sample
	if singleGateway {
		samples, err = GetGatewaySamplesInRange(networkId, gatewayId, xMin, yMin, xMax, yMax)
	} else {
		samples, err = GetNetworkSamplesInRange(networkId, xMin, yMin, xMax, yMax)
	}

	// Database error
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Sort by RSSI ascending
	sort.Sort(types.ByRssi(samples))

	// Do something with tile data
	tile := CreateGlobalBlocksTile(x, y, z, samples)

	// Set cache headers in the response
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

	err = png.Encode(w, tile)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prometheus stats
	//gatewayElapsed := time.Since(tileStart)
	//promTmsBlocksDuration.Observe(float64(gatewayElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
	//}
}

func CreateGlobalBlocksTile(x int, y int, z int, samples []types.Sample) image.Image {

	// x, y, z is for outer tile
	// Draw image for x-1, y-1 to x+2, y+2

	// Z19 indexes are:
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(x, y, z, 0)
	xWidth := float64(xMax - xMin)
	yWidth := float64(yMax - yMin) // always the same ??

	// For a z19 tile we need to draw a circle that fills the whole tile. That is a circle with radius half the diagonal of the tile.
	nominalRadius := 256.0
	// For every zoom level higher than 19 we can half the radius
	zDiff := float64(19 - z)
	nominalRadius = nominalRadius / (math.Pow(2, zDiff))
	nominalRadius = math.Max(nominalRadius, 8.0) // minimum is 1 pixels radius

	dc := gg.NewContext(256, 256)

	for _, sample := range samples {

		// Add 0.5 because the tile index is the NW corner, but we want to draw it in the middle of the z19 tile
		pixelY := ((float64(sample.Y - yMin)) / yWidth) * 256.0 // pixels from top
		pixelX := ((float64(sample.X - xMin)) / xWidth) * 256.0 // pixels from left

		if nominalRadius == 8.0 {
			pixelY = math.Floor(pixelY/8.0) * 8.0
			pixelX = math.Floor(pixelX/8.0) * 8.0
		}

		signal := sample.MaxBucketIndex
		if signal == 12 {
			// Out of range, do not draw
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(0, 0, 0)
		} else if signal >= 5 {
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(0, 0, 1)
		} else if signal == 4 {
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(0, 1, 1)
		} else if signal == 3 {
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(0, 1, 0)
		} else if signal == 2 {
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(1, 1, 0)
		} else if signal == 1 {
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(1, 0.5, 0)
		} else {
			dc.DrawRectangle(pixelX, pixelY, nominalRadius, nominalRadius)
			dc.SetRGB(1, 0, 0)
		}
		dc.Fill()
	}

	srcImage := dc.Image()

	tileFileName := fmt.Sprintf("%s/%d/%d/%d.png", MyConfiguration.CacheDirBlocks, z, x, y)
	tileFolderName := fmt.Sprintf("%s/%d/%d/", MyConfiguration.CacheDirBlocks, z, x)
	CreateDirIfNotExist(tileFolderName)

	newImage, _ := os.Create(tileFileName)

	err := png.Encode(newImage, srcImage)
	if err != nil {
		log.Print(err.Error())
	}

	_ = newImage.Close()

	return srcImage
}
