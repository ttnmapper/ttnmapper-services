package tms

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"strconv"
	"ttnmapper-services/pkg/database"
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
		PromTmsRequestDuration.WithLabelValues(path).Observe(float64(elapsedTime.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
	})
}

func Run() {

	err := LoadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	DB, err = database.OpenPostgres(database.PostgresOptions{Host: MyConfiguration.PostgresHost, Port: strconv.Itoa(MyConfiguration.PostgresPort), User: MyConfiguration.PostgresUser, Password: MyConfiguration.PostgresPassword, Database: MyConfiguration.PostgresDatabase, DebugLog: MyConfiguration.PostgresDebugLog})

	if err != nil {
		panic(err.Error())
	}

	// Get generic database object sql.DB to use its functions
	sqlDB, err := DB.DB()
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDB.SetMaxIdleConns(2)
	// SetMaxOpenConns sets the maximum number of open connections to the database.
	sqlDB.SetMaxOpenConns(20)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(10 * time.Minute)

	// Cache
	AntennaLastHeardCache = NewCache(5*time.Minute, 1*time.Minute)

	// Register prometheus stats
	prometheus.MustRegister(PromAntennaCacheItemCount)
	prometheus.MustRegister(PromTmsRequestDuration)
	prometheus.MustRegister(PromTmsGlobalSelectDuration)
	prometheus.MustRegister(PromTmsGatewaySelectDuration)
	prometheus.MustRegister(PromTmsSimCovRequestDuration)

	log.Println("Starting server")
	router := mux.NewRouter().UseEncodedPath() //.StrictSlash(true)
	router.Use(LoggingMiddleware)
	router.Use(prometheusMiddleware)
	router.HandleFunc("/", Index)
	router.Handle("/metrics", promhttp.Handler())
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)

	// Tile endpoints
	router.HandleFunc("/circles/network/{network_id}/{z}/{x}/{y}", GetCirclesTile)
	router.HandleFunc("/blocks/network/{network_id}/{z}/{x}/{y}", GetBlocksTile)
	router.HandleFunc("/circles/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", GetCirclesTile)
	router.HandleFunc("/blocks/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", GetBlocksTile)
	router.HandleFunc("/simcov/network/{network_id}/{z}/{x}/{y}", GetSimCovNetworkTile)
	router.HandleFunc("/simcov/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", GetSimCovTile)
	router.HandleFunc("/simcov/rasters/network/{network_id}", GetSimCovRasterManifest).Methods("GET")
	router.HandleFunc("/simcov/rasters/gateway/{network_id}/{gateway_id}", GetSimCovGatewayRaster).Methods("GET", "HEAD")

	// Debug endpoints
	router.HandleFunc("/debug/", DebugMapPage)
	router.HandleFunc("/debug/assets/{file}", DebugAsset)
	router.HandleFunc("/debug/gateway/{network_id}/{gateway_id}/comparison", GetComparisonData)
	router.HandleFunc("/debug/gateway/{network_id}/{gateway_id}/analysis", GetAnalysisData)
	router.HandleFunc("/debug/gateway/comparison.html", ComparisonPage)

	routerWithTimeout := http.TimeoutHandler(router, time.Minute*5, "Handler Timeout!")
	log.Fatal(http.ListenAndServe(MyConfiguration.ListenAddress, routerWithTimeout))

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
