package tms

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type Configuration struct {
	CacheDirCircles string `env:"CACHE_DIR_CIRCLES"`
	CacheDirBlocks  string `env:"CACHE_DIR_BLOCKS"`
	CacheDirSimCov  string `env:"CACHE_DIR_SIMCOV"`

	CacheEnabled bool `env:"CACHE_ENABLED"`

	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     int    `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`

	ListenAddress string `env:"LISTEN_ADDRESS"`

	SignalServerBin            string `env:"SIGNAL_SERVER_BIN"`
	SignalServerSrtmDir        string `env:"SIGNAL_SERVER_SRTM_DIR"`
	GatewayParamsFile          string `env:"GATEWAY_PARAMS_FILE"`
	SimCovRadiusKm             int    `env:"SIMCOV_RADIUS_KM"`
	SignalServerMaxProcesses   int    `env:"SIGNAL_SERVER_MAX_PROCESSES"`
	SignalServerThreads        int    `env:"SIGNAL_SERVER_THREADS"`
	SignalServerTimeoutSeconds int    `env:"SIGNAL_SERVER_TIMEOUT_SECONDS"`
	SimCovMaxRasterGateways    int    `env:"SIMCOV_MAX_RASTER_GATEWAYS"`
}

var MyConfiguration = Configuration{
	CacheDirCircles: "./tile_cache/global_circles",
	CacheDirBlocks:  "./tile_cache/global_blocks",
	CacheDirSimCov:  "./tile_cache/simcov",

	CacheEnabled: false,

	PostgresHost:     "localhost",
	PostgresPort:     5432,
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	ListenAddress: ":8080",

	SignalServerBin: "/usr/local/bin/signalserver",
	// Stored under tile_cache so downloaded DEM tiles persist across container
	// restarts when tile_cache is mounted as a volume, instead of being
	// re-downloaded every time.
	SignalServerSrtmDir: "./tile_cache/srtm",
	GatewayParamsFile:   "gateway_params.json",
	SimCovRadiusKm:      50,
	// One process uses the CPU budget by default; threads=0 divides GOMAXPROCS
	// across the configured processes. Set GOMAXPROCS to the container CPU budget.
	SignalServerMaxProcesses:   1,
	SignalServerThreads:        0,
	SignalServerTimeoutSeconds: 300,
	SimCovMaxRasterGateways:    128,
}

// LoadConfiguration populates MyConfiguration from, in order of increasing priority:
//  1. the hardcoded defaults above
//  2. the JSON file at filename, if it exists (missing file is not an error)
//  3. environment variables, as tagged on the Configuration struct
//
// This allows the service to start up using only environment variables (or even
// just the defaults) when no config file is present, e.g. in a container image
// that does not ship a conf.json.
func LoadConfiguration(filename string) error {
	if _, err := os.Stat(filename); err == nil {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&MyConfiguration); err != nil {
			return err
		}
	} else {
		log.Printf("Config file %q not found, using defaults/environment variables", filename)
	}

	// Environment variables always take precedence over the file/defaults.
	if err := env.Parse(&MyConfiguration); err != nil {
		return err
	}

	return nil
}

var (
	PromTmsRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_request_duration",
		Help:    "Duration to serve a request",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	},
		[]string{"path"},
	)

	PromAntennaCacheItemCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ttnmapper_tms_antenna_cache_size",
		Help: "Size of the memory cache that holds antennas previously read from the database",
	})

	PromTmsGlobalSelectDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_select_global_duration",
		Help:    "Duration of selecting global data for one tile from the database",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})
	PromTmsGatewaySelectDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_select_gateway_duration",
		Help:    "Duration of selecting gateway data for one tile from the database",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})

	PromTmsSimCovRequestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_simcov_request_duration",
		Help:    "Duration to serve a simulated coverage tile request",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})

	// Other global vars
	DB *gorm.DB

	AntennaLastHeardCache *cache.Cache
)

func NewCache(defaultExpiration, cleanupInterval time.Duration) *cache.Cache {
	return cache.New(defaultExpiration, cleanupInterval)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func PrettyPrint(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
