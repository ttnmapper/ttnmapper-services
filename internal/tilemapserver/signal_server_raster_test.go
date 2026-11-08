package tms

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/image/tiff"
)

// Append georeferencing tags to a real encoded TIFF, retaining the encoder's
// pixel offsets and tags. This exercises both tag parsing and raster decoding.
func testGeoTIFF(t testing.TB, img image.Image, lon, lat, lonScale, latScale float64) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	order := binary.LittleEndian
	oldOffset := int(order.Uint32(data[4:8]))
	n := int(order.Uint16(data[oldOffset : oldOffset+2]))
	directory := append([]byte(nil), data[oldOffset+2:oldOffset+2+n*12]...)
	offset := len(data)
	scaleOffset := offset + 2 + (n+2)*12 + 4
	tieOffset := scaleOffset + 24
	data = append(data, make([]byte, 2+(n+2)*12+4+24+48)...)
	order.PutUint32(data[4:8], uint32(offset))
	order.PutUint16(data[offset:offset+2], uint16(n+2))
	copy(data[offset+2:], directory)
	for i, tag := range []struct {
		id            uint16
		count, offset uint32
	}{{33550, 3, uint32(scaleOffset)}, {33922, 6, uint32(tieOffset)}} {
		entry := data[offset+2+(n+i)*12:]
		order.PutUint16(entry[:2], tag.id)
		order.PutUint16(entry[2:4], 12)
		order.PutUint32(entry[4:8], tag.count)
		order.PutUint32(entry[8:12], tag.offset)
	}
	for i, v := range []float64{lonScale, latScale, 0} {
		order.PutUint64(data[scaleOffset+i*8:], math.Float64bits(v))
	}
	for i, v := range []float64{0, 0, 0, lon, lat, 0} {
		order.PutUint64(data[tieOffset+i*8:], math.Float64bits(v))
	}
	return data
}

type countingReaderAt struct {
	io.ReaderAt
	bytes int
}

func (reader *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := reader.ReaderAt.ReadAt(p, off)
	reader.bytes += n
	return n, err
}

func TestGeoTIFFExtentUsesSmallReadsAndRejectsTruncation(t *testing.T) {
	data := testGeoTIFF(t, image.NewGray(image.Rect(0, 0, 512, 512)), 4, 52, .001, .002)
	reader := &countingReaderAt{ReaderAt: bytes.NewReader(data)}
	lon, lat, dx, dy, width, height, err := readGeoTIFFExtentFromReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	if lon != 4 || lat != 52 || dx != .001 || dy != .002 || width != 512 || height != 512 {
		t.Fatalf("unexpected extent: %v %v %v %v %v %v", lon, lat, dx, dy, width, height)
	}
	if reader.bytes > 512 {
		t.Fatalf("metadata read %d bytes from %d-byte raster", reader.bytes, len(data))
	}
	for _, length := range []int{0, 7, len(data) - 1} {
		if _, _, _, _, _, _, err := readGeoTIFFExtentFromReader(bytes.NewReader(data[:length])); err == nil {
			t.Errorf("accepted truncation at %d", length)
		}
	}
}

// Reference the original per-pixel projection to detect boundary/rounding
// changes caused by computing coordinate axes once per row/column.
func referenceSimCovTile(t *testing.T, path string, x, y, z int) image.Image {
	t.Helper()
	lon0, lat0, dx, dy, width, height, err := readGeoTIFFExtent(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := tiff.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	tile := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	n := math.Pow(2, float64(z))
	for py := range 256 {
		for px := range 256 {
			lon := (float64(x)+float64(px)/256)/n*360 - 180
			lat := math.Atan(math.Sinh(math.Pi*(1-2*(float64(y)+float64(py)/256)/n))) * 180 / math.Pi
			ix, iy := int(math.Round((lon-lon0)/dx)), int(math.Round((lat0-lat)/dy))
			if ix < 0 || iy < 0 || ix >= width || iy >= height {
				continue
			}
			r, _, _, a := img.At(ix, iy).RGBA()
			if a == 0 {
				continue
			}
			dbm := (1-float64(r>>8)/255)*(signalServerScaleMax-signalServerScaleMin) + signalServerScaleMin
			tile.Set(px, py, signalDbmToColor(dbm))
		}
	}
	return tile
}

func TestSimCovRasterRenderingPreservesPixels(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	var references []image.Image
	for source := range 2 {
		img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		for y := range 32 {
			for x := range 32 {
				v := uint8(40 + (x*7+y*3+source*53)%210)
				a := uint8(255)
				if (x+y+source)%7 == 0 {
					a = 0
				}
				img.SetNRGBA(x, y, color.NRGBA{v, v, v, a})
			}
		}
		path := filepath.Join(dir, string(rune('a'+source))+".tif")
		data := testGeoTIFF(t, img, 0, 66.51326044311186, 90.0/32, 66.51326044311186/32)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		reference := referenceSimCovTile(t, path, 2, 1, 2)
		references = append(references, reference)
		actual, err := RenderSimCovTileFromGeoTIFF(path, 2, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		for y := range 256 {
			for x := range 256 {
				if color.NRGBAModel.Convert(actual.At(x, y)) != color.NRGBAModel.Convert(reference.At(x, y)) {
					t.Fatalf("gateway %d pixel %d,%d differs", source, x, y)
				}
			}
		}
	}
	actual, err := RenderSimCovNetworkTileFromGeoTIFFs(paths, 2, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Independently select the strongest raw signal, then compare its color.
	for y := range 256 {
		for x := range 256 {
			var want color.Color = color.NRGBA{}
			best := uint32(65536)
			for source, reference := range references {
				// The fixture's values are monotonic in power; derive the raw value
				// at the independently projected source coordinate.
				lon := float64(x) / 256 * 90
				lat := math.Atan(math.Sinh(math.Pi*(1-2*(1+float64(y)/256)/4))) * 180 / math.Pi
				ix := int(math.Round(lon / (90.0 / 32)))
				iy := int(math.Round((66.51326044311186 - lat) / (66.51326044311186 / 32)))
				if ix >= 32 || iy >= 32 || (ix+iy+source)%7 == 0 {
					continue
				}
				v := uint32(40 + (ix*7+iy*3+source*53)%210)
				if v < best {
					best = v
					want = reference.At(x, y)
				}
			}
			if color.NRGBAModel.Convert(actual.At(x, y)) != color.NRGBAModel.Convert(want) {
				t.Fatalf("network pixel %d,%d differs", x, y)
			}
		}
	}
}

func TestParseGeoTIFFPreservesZ19Samples(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.SetGray(x, y, color.Gray{Y: uint8((x*11 + y*7) % 255)})
		}
	}
	path := filepath.Join(t.TempDir(), "coverage.tif")
	const lon, lat, dx, dy = 4.0, 52.0, .0005, .0005
	if err := os.WriteFile(path, testGeoTIFF(t, img, lon, lat, dx, dy), 0600); err != nil {
		t.Fatal(err)
	}
	samples, err := parseGeoTIFF(path)
	if err != nil {
		t.Fatal(err)
	}
	type cell struct{ x, y int }
	want := map[cell]int{}
	for y := range 16 {
		for x := range 16 {
			v := img.GrayAt(x, y).Y
			if v == 0 {
				continue
			}
			bucket := signalDbmToBucketIndex(signalServerScaleMin + (1-float64(v)/255)*(signalServerScaleMax-signalServerScaleMin))
			xmin, ymin := LatLonToZ19Tile(lat-float64(y)*dy, lon+float64(x)*dx)
			xmax, ymax := LatLonToZ19Tile(lat-float64(y+1)*dy, lon+float64(x+1)*dx)
			for tx := xmin; tx <= xmax; tx++ {
				for ty := ymin; ty <= ymax; ty++ {
					key := cell{tx, ty}
					if old, ok := want[key]; !ok || bucket < old {
						want[key] = bucket
					}
				}
			}
		}
	}
	actual := map[cell]int{}
	for _, sample := range samples {
		actual[cell{sample.X, sample.Y}] = sample.MaxBucketIndex
	}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("sample cells differ: got %d, want %d", len(actual), len(want))
	}
}
