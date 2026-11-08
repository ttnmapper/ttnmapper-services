package tms

import (
	"encoding/gob"
	"io"
	"os"
	"testing"

	"ttnmapper-services/internal/tilemapserver/types"
)

func TestSimCovMetadataReadsLegacyCacheAndDetectsReplacement(t *testing.T) {
	original := MyConfiguration
	MyConfiguration.CacheDirSimCov = t.TempDir()
	t.Cleanup(func() { MyConfiguration = original })
	path := simCovCacheFilePath("net", "gw")
	CreateDirIfNotExist(MyConfiguration.CacheDirSimCov + "/net")
	// Write the existing format directly, without populating the metadata cache.
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := simCovCacheEntry{Lat: 52, Lon: 4, Params: types.SimulationParams{HeightAgl: 5}, Samples: []types.Sample{{X: 42, Y: 43}}}
	if err := gob.NewEncoder(f).Encode(entry); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		lat, lon, params, ok := loadSimCovMetadata("net", "gw")
		if !ok || lat != 52 || lon != 4 || params.HeightAgl != 5 {
			t.Fatalf("metadata = %v %v %+v %v", lat, lon, params, ok)
		}
	}
	// An externally replaced file must invalidate memory even with identical
	// size and mtime, as can happen when restoring simulation caches.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entry.Lat = 53
	if err := writeSimCovArtifact(path, func(w io.Writer) error { return gob.NewEncoder(w).Encode(entry) }); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if lat, _, _, ok := loadSimCovMetadata("net", "gw"); !ok || lat != 53 {
		t.Fatalf("replacement metadata = %v, %v", lat, ok)
	}
	if err := DeleteSimCovCache("net", "gw"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := loadSimCovMetadata("net", "gw"); ok {
		t.Fatal("deleted cache still considered valid")
	}
}

func BenchmarkSimCovCacheLookup(b *testing.B) {
	original := MyConfiguration
	MyConfiguration.CacheDirSimCov = b.TempDir()
	b.Cleanup(func() { MyConfiguration = original })
	samples := make([]types.Sample, 50000)
	for i := range samples {
		samples[i] = types.Sample{X: i, Y: i, MaxBucketIndex: i % 12}
	}
	SaveSimCovCache("net", "gw", samples, 52, 4, types.SimulationParams{}, "")
	b.Run("FullSamples", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, _, _, _, ok := LoadSimCovCache("net", "gw"); !ok {
				b.Fatal("cache missing")
			}
		}
	})
	b.Run("Metadata", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, _, _, ok := loadSimCovMetadata("net", "gw"); !ok {
				b.Fatal("metadata missing")
			}
		}
	})
}
