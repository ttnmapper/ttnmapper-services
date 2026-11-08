package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"ttnmapper-services/internal/tilemapserver"

	"strconv"
	"ttnmapper-services/pkg/database"
)

func main() {
	gatewayID := flag.String("gateway", "", "Gateway ID (required)")
	networkID := flag.String("network", "NS_TTS_V3://ttn@000013", "Network ID")
	maxTries := flag.Int("tries", 100, "Maximum number of optimization tries")
	flag.Parse()

	if *gatewayID == "" {
		fmt.Println("Error: -gateway is required")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	err := tms.LoadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	tms.DB, err = database.OpenPostgres(database.PostgresOptions{Host: tms.MyConfiguration.PostgresHost, Port: strconv.Itoa(tms.MyConfiguration.PostgresPort), User: tms.MyConfiguration.PostgresUser, Password: tms.MyConfiguration.PostgresPassword, Database: tms.MyConfiguration.PostgresDatabase, ApplicationName: "optimize-location"})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Printf("Starting location optimization for gateway %s (%d tries)\n", *gatewayID, *maxTries)

	if err := tms.RunLocationOptimization(*networkID, *gatewayID, *maxTries); err != nil {
		log.Fatalf("Location optimization failed: %v", err)
	}

	fmt.Printf("Location optimization finished. Optimized parameters for %s saved to %s\n", *gatewayID, tms.MyConfiguration.GatewayParamsFile)
}
