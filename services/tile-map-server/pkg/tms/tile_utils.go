package tms

import (
	"math"
	"os"
	"time"
	"ttnmapper-tms/pkg/tms/types"
)

func GetCacheDurationForZoom(z int) time.Duration {
	//if z >= 18 {
	//	return 0 * time.Second // always redraw
	//}
	//if z >= 16 {
	//	return 10 * time.Minute
	//}
	//if z >= 14 {
	//	return 60 * time.Minute
	//}
	//if z >= 12 {
	//	return 2 * time.Hour
	//}
	//if z >= 10 {
	//	return 4 * time.Hour
	//}
	//if z >= 8 {
	//	return 8 * time.Hour
	//}
	//if z >= 6 {
	//	return 10 * time.Hour
	//}
	//if z >= 4 {
	//	return 12 * time.Hour
	//}
	//if z <= 2 {
	//	return 14*time.Hour
	//}
	if z >= 0 {
		return 24 * time.Hour
	}
	return 0
}

func GetZ19TileRangeBuffer(xOuter int, yOuter int, z int, buffer float64) (xMin int, yMin int, xMax int, yMax int) {

	/*
		+-----------+-----------+
		|   2x,2y   |  2x+1,2y  |
		+-----------+-----------+
		|  2x,2y+1  | 2x+1,2y+1 |
		+-----------+-----------+
	*/
	// Select one tile size extra to all sides. ie 9 tile
	xNw := float64(xOuter) - buffer
	yNw := float64(yOuter) - buffer
	xSe := float64(xOuter) + 1 + buffer
	ySe := float64(yOuter) + 1 + buffer

	for ; z < 19; z++ {
		xNw *= 2
		yNw *= 2

		xSe *= 2
		ySe *= 2
	}

	xNw = math.Floor(xNw)
	yNw = math.Floor(yNw)
	xSe = math.Ceil(xSe)
	ySe = math.Ceil(ySe)
	return int(xNw), int(yNw), int(xSe), int(ySe)
}

func getMaxBucket(gridCell types.GridCell) int {
	maxBucketIndex := 12 // Use NoSignal as default
	maxBucketCount := gridCell.BucketNoSignal

	if gridCell.BucketHigh > maxBucketCount {
		maxBucketCount = gridCell.BucketHigh
		maxBucketIndex = 0
	}
	if gridCell.Bucket100 > maxBucketCount {
		maxBucketCount = gridCell.Bucket100
		maxBucketIndex = 1
	}
	if gridCell.Bucket105 > maxBucketCount {
		maxBucketCount = gridCell.Bucket105
		maxBucketIndex = 2
	}
	if gridCell.Bucket110 > maxBucketCount {
		maxBucketCount = gridCell.Bucket110
		maxBucketIndex = 3
	}
	if gridCell.Bucket115 > maxBucketCount {
		maxBucketCount = gridCell.Bucket115
		maxBucketIndex = 4
	}
	if gridCell.Bucket120 > maxBucketCount {
		maxBucketCount = gridCell.Bucket120
		maxBucketIndex = 5
	}
	if gridCell.Bucket125 > maxBucketCount {
		maxBucketCount = gridCell.Bucket125
		maxBucketIndex = 6
	}
	if gridCell.Bucket130 > maxBucketCount {
		maxBucketCount = gridCell.Bucket130
		maxBucketIndex = 7
	}
	if gridCell.Bucket135 > maxBucketCount {
		maxBucketCount = gridCell.Bucket135
		maxBucketIndex = 8
	}
	if gridCell.Bucket140 > maxBucketCount {
		maxBucketCount = gridCell.Bucket140
		maxBucketIndex = 9
	}
	if gridCell.Bucket145 > maxBucketCount {
		maxBucketCount = gridCell.Bucket145
		maxBucketIndex = 10
	}
	if gridCell.BucketLow > maxBucketCount {
		maxBucketCount = gridCell.BucketLow
		maxBucketIndex = 11
	}

	return maxBucketIndex
}

// TileCenterLatLon returns the geographic centre of the tile at (x, y, z).
func TileCenterLatLon(x, y, z int) (lat, lon float64) {
	n := math.Pow(2, float64(z))
	lon = (float64(x)+0.5)/n*360.0 - 180.0
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*(float64(y)+0.5)/n)))
	lat = latRad * 180.0 / math.Pi
	return lat, lon
}

// TileCornersLatLon returns the geographic coordinates of the four corners of
// the tile at (x, y, z): NW, NE, SE, SW.
func TileCornersLatLon(x, y, z int) [4][2]float64 {
	n := math.Pow(2, float64(z))
	lonW := float64(x)/n*360.0 - 180.0
	lonE := float64(x+1)/n*360.0 - 180.0
	latNRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	latSRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y+1)/n)))
	latN := latNRad * 180.0 / math.Pi
	latS := latSRad * 180.0 / math.Pi
	return [4][2]float64{
		{latN, lonW}, // NW
		{latN, lonE}, // NE
		{latS, lonE}, // SE
		{latS, lonW}, // SW
	}
}

func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}
