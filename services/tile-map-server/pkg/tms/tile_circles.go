package tms

import (
	"fmt"
	"github.com/fogleman/gg"
	"github.com/gorilla/mux"
	"image"
	"image/png"
	"io"
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

func GetCirclesTile(w http.ResponseWriter, r *http.Request) {
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
		log.Println(w, "Z invalid")
		http.Error(w, "z invalid", http.StatusBadRequest)
		return
	}

	x, err := strconv.Atoi(vars["x"])
	if err != nil {
		log.Println(w, "X invalid")
		http.Error(w, "x invalid", http.StatusBadRequest)
		return
	}

	if strings.HasSuffix(vars["y"], ".png") {
		vars["y"] = strings.TrimSuffix(vars["y"], ".png")
	}
	y, err := strconv.Atoi(vars["y"])
	if err != nil {
		log.Println(w, "Y invalid")
		http.Error(w, "y invalid", http.StatusBadRequest)
		return
	}

	log.Printf("Circles tile %s - %s: %d/%d/%d\t", networkId, gatewayId, z, x, y)

	tileFileName := ""
	if singleGateway {
		tileFileName = fmt.Sprintf("%s/gateway/%s/%s/%d/%d/%d.png", MyConfiguration.CacheDirCircles, url.QueryEscape(networkId), url.QueryEscape(gatewayId), z, x, y)
	} else {
		tileFileName = fmt.Sprintf("%s/network/%s/%d/%d/%d.png", MyConfiguration.CacheDirCircles, url.QueryEscape(networkId), z, x, y)
	}

	tileInCacheOutdated := true
	tileExistInCache := false
	if file, err := os.Stat(tileFileName); err == nil {
		tileExistInCache = true

		modifiedTime := file.ModTime()

		// Check the last modified time of the file to see if the time is still new enough
		if modifiedTime.Add(GetCacheDurationForZoom(z) * 2).After(time.Now()) {
			tileInCacheOutdated = false
		}
	}

	//log.Println("Cache enabled: ", MyConfiguration.CacheEnabled)
	//log.Println("Tile in cache: ", tileExistInCache)
	//log.Println("Tile outdated: ", tileInCacheOutdated)
	//log.Println("Single gateway: ", singleGateway)

	// Only cache global tiles, not per gateway tiles
	if MyConfiguration.CacheEnabled && tileExistInCache && !tileInCacheOutdated && !singleGateway {
		log.Printf("serving from cache\n")
		//promTmsCirclesCacheCount.Inc()

		//Check if file exists and open
		tileFile, err := os.Open(tileFileName)
		if err != nil {
			//File not found, send 404
			http.Error(w, "File not found.", 404)
			return
		}

		_, err = io.Copy(w, tileFile) //'Copy' the file to the client
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tileFile.Close() //Close after function returns
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else {
		log.Printf("generating tile\n")
		//promTmsCirclesCreateCount.Inc()
		//tileStart := time.Now()

		// Circles can overlap tiles. Generate a list of z19 tiles in the current tile, with a buffer around this tile.
		// Z  - buffer for one z19 tile
		// 19 - 1
		// 18 - 0.5
		// 17 - 0.25
		// 16 - 0.125
		// 15 - 0.0625
		// 14 - 0.03125
		// min circle radius is 6*1.6=9.6
		// 9.6 / 256 = 0.0375 = min buffer
		buffer := 1 / math.Pow(2, float64(19-z))
		if buffer < 0.0375 {
			buffer = 0.0375
		}
		xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(x, y, z, buffer)

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
		tile := CreateCirclesTile(x, y, z, samples)
		if MyConfiguration.CacheEnabled && !singleGateway {
			StoreTileInFile(tile, tileFileName)
		}

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
		//promTmsCirclesDuration.Observe(float64(gatewayElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
	}
}

func CreateCirclesTile(x int, y int, z int, samples []types.Sample) image.Image {

	// x, y, z is for outer tile
	// Draw image for x-1, y-1 to x+2, y+2

	// Z19 indexes are:
	xMin, yMin, _, _ := GetZ19TileRangeBuffer(x-1, y-1, z, 0)
	_, _, xMax, yMax := GetZ19TileRangeBuffer(x+1, y+1, z, 0)
	xWidth := float64(xMax - xMin)
	yWidth := float64(yMax - yMin)

	// For a z19 tile we need to draw a circle that fills the whole tile. That is a circle with radius half the diagonal of the tile.
	nominalRadius := 181.0 //math.Sqrt(256^2*256^2) / 2
	// For every zoom level higher than 19 we can half the radius
	zDiff := float64(19 - z)
	nominalRadius = nominalRadius / (math.Pow(2, zDiff))
	//log.Println("Equivalent radius ", nominalRadius)
	nominalRadius = math.Max(nominalRadius, 6.0) // minimum is 3 pixels radius

	dc := gg.NewContext(768, 768)

	for _, sample := range samples {

		// Add 0.5 because the tile index is the NW corner, but we want to draw it in the middle of the z19 tile
		pixelY := ((float64(sample.Y-yMin) + 0.5) / yWidth) * 768.0 // pixels from top
		pixelX := ((float64(sample.X-xMin) + 0.5) / xWidth) * 768.0 // pixels from left

		signal := sample.MaxBucketIndex
		if signal == 12 {
			// Out of range, do not draw
			continue
		} else if signal >= 5 { //11..5
			dc.SetRGB(0, 0, 1)
			dc.DrawCircle(pixelX, pixelY, nominalRadius*1.5)
		} else if signal == 4 {
			dc.SetRGB(0, 1, 1)
			dc.DrawCircle(pixelX, pixelY, nominalRadius*1.4)
		} else if signal == 3 {
			dc.SetRGB(0, 1, 0)
			dc.DrawCircle(pixelX, pixelY, nominalRadius*1.3)
		} else if signal == 2 {
			dc.SetRGB(1, 1, 0)
			dc.DrawCircle(pixelX, pixelY, nominalRadius*1.2)
		} else if signal == 1 {
			dc.SetRGB(1, 0.5, 0)
			dc.DrawCircle(pixelX, pixelY, nominalRadius*1.1)
		} else {
			dc.SetRGB(1, 0, 0)
			dc.DrawCircle(pixelX, pixelY, nominalRadius)
		}
		dc.Fill()
	}

	srcImage := dc.Image()

	// Crop out tile
	tile := srcImage.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(256, 256, 2*256, 2*256))

	return tile
}

func StoreTileInFile(tile image.Image, filename string) {
	tileFolderName := filename[:strings.LastIndex(filename, "/")]
	CreateDirIfNotExist(tileFolderName)

	newImage, err := os.Create(filename)
	if err != nil {
		log.Println(err.Error())
	}

	err = png.Encode(newImage, tile)
	if err != nil {
		log.Println(err.Error())
	}

	err = newImage.Close()
	if err != nil {
		log.Println(err.Error())
	}
}
