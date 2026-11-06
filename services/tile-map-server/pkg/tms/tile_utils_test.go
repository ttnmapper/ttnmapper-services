package tms

import (
	"log"
	"testing"
)

func TestGetZ19TileRangeBuffer(t *testing.T) {
	xNw, yNw, xSe, ySe := GetZ19TileRangeBuffer(100, 100, 17, 0.5)
	log.Println(xNw, yNw, xSe, ySe)
}
