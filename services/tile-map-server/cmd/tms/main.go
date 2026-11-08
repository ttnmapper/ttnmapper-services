package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ttnmapper-services/services/tile-map-server/pkg/tms"
)

/*
Technopark https://tile.openstreetmap.org/15/18098/19674.png
*/

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()

		startTime := time.Now()

		next.ServeHTTP(w, r)

		elapsedTime := time.Since(startTime)
		tms.PromTmsRequestDuration.WithLabelValues(path).Observe(float64(elapsedTime.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
	})
}

func main() {

	err := tms.LoadConfiguration("conf.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("[Configuration]\n%s\n", tms.PrettyPrint(tms.MyConfiguration)) // output: [UserA, UserB]

	var gormLogLevel = logger.Silent
	if tms.MyConfiguration.PostgresDebugLog {
		log.Println("Database debug logging enabled")
		gormLogLevel = logger.Info
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=disable application_name=%s",
		tms.MyConfiguration.PostgresHost,
		tms.MyConfiguration.PostgresPort,
		tms.MyConfiguration.PostgresUser,
		tms.MyConfiguration.PostgresDatabase,
		tms.MyConfiguration.PostgresPassword,
		filepath.Base(os.Args[0]),
	)
	tms.DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		panic(err.Error())
	}

	// Get generic database object sql.DB to use its functions
	sqlDB, err := tms.DB.DB()
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDB.SetMaxIdleConns(2)
	// SetMaxOpenConns sets the maximum number of open connections to the database.
	sqlDB.SetMaxOpenConns(20)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(10 * time.Minute)

	// Cache
	tms.AntennaLastHeardCache = tms.NewCache(5*time.Minute, 1*time.Minute)

	// Register prometheus stats
	prometheus.MustRegister(tms.PromAntennaCacheItemCount)
	prometheus.MustRegister(tms.PromTmsRequestDuration)
	prometheus.MustRegister(tms.PromTmsGlobalSelectDuration)
	prometheus.MustRegister(tms.PromTmsGatewaySelectDuration)
	prometheus.MustRegister(tms.PromTmsSimCovRequestDuration)

	log.Println("Starting server")
	router := mux.NewRouter().UseEncodedPath() //.StrictSlash(true)
	router.Use(tms.LoggingMiddleware)
	router.Use(prometheusMiddleware)
	router.HandleFunc("/", Index)
	router.Handle("/metrics", promhttp.Handler())
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)

	// Tile endpoints
	router.HandleFunc("/circles/network/{network_id}/{z}/{x}/{y}", tms.GetCirclesTile)
	router.HandleFunc("/blocks/network/{network_id}/{z}/{x}/{y}", tms.GetBlocksTile)
	router.HandleFunc("/circles/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", tms.GetCirclesTile)
	router.HandleFunc("/blocks/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", tms.GetBlocksTile)
	router.HandleFunc("/simcov/network/{network_id}/{z}/{x}/{y}", tms.GetSimCovNetworkTile)
	router.HandleFunc("/simcov/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", tms.GetSimCovTile)
	router.HandleFunc("/simcov/rasters/network/{network_id}", tms.GetSimCovRasterManifest).Methods("GET")
	router.HandleFunc("/simcov/rasters/gateway/{network_id}/{gateway_id}", tms.GetSimCovGatewayRaster).Methods("GET", "HEAD")

	// Debug endpoints
	router.HandleFunc("/debug/", tms.DebugMapPage)
	router.HandleFunc("/debug/assets/{file}", tms.DebugAsset)
	router.HandleFunc("/debug/gateway/{network_id}/{gateway_id}/comparison", tms.GetComparisonData)
	router.HandleFunc("/debug/gateway/{network_id}/{gateway_id}/analysis", tms.GetAnalysisData)
	router.HandleFunc("/debug/gateway/comparison.html", tms.ComparisonPage)

	routerWithTimeout := http.TimeoutHandler(router, time.Minute*5, "Handler Timeout!")
	log.Fatal(http.ListenAndServe(tms.MyConfiguration.ListenAddress, routerWithTimeout))

}

func Index(w http.ResponseWriter, r *http.Request) {
	name, err := os.Hostname()
	if err != nil {
		log.Println(err.Error())
		return
	}

	_, err = fmt.Fprintf(w, "TMS root\n%s\n%s", name, time.Now().String())
	if err != nil {
		log.Println(err.Error())
		return
	}
}
