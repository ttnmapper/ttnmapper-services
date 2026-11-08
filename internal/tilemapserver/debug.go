package tms

import (
	"embed"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// debugPages holds the static HTML debug pages, compiled into the binary.
//
//go:embed debug/*.html debug/*.js
var debugPages embed.FS

func DebugAsset(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["file"]
	if name != "coverage.js" && name != "coverage-worker.js" {
		http.NotFound(w, r)
		return
	}
	data, err := debugPages.ReadFile("debug/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// serveDebugPage writes an embedded debug HTML page.
func serveDebugPage(w http.ResponseWriter, name string) {
	data, err := debugPages.ReadFile(name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		log.Printf("debug: failed to write %s: %v", name, err)
	}
}

// DebugMapPage serves an interactive Leaflet map that overlays the measured and
// simulated coverage layers for a gateway/network. It is the root of the debug
// UI (served at /debug/). The network_id, gateway_id and initial view can be
// overridden via query parameters.
func DebugMapPage(w http.ResponseWriter, r *http.Request) {
	serveDebugPage(w, "debug/map.html")
}

// ComparisonPage serves the measured-vs-simulated signal-vs-distance chart.
// It expects network_id and gateway_id query parameters.
func ComparisonPage(w http.ResponseWriter, r *http.Request) {
	serveDebugPage(w, "debug/comparison.html")
}
