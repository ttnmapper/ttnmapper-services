package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"ttnmapper-tms/pkg/tms"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	err := tms.LoadConfiguration("conf.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=disable application_name=optimize-location",
		tms.MyConfiguration.PostgresHost,
		tms.MyConfiguration.PostgresPort,
		tms.MyConfiguration.PostgresUser,
		tms.MyConfiguration.PostgresDatabase,
		tms.MyConfiguration.PostgresPassword,
	)
	tms.DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Printf("Starting location optimization for gateway %s (%d tries)\n", *gatewayID, *maxTries)

	if err := tms.RunLocationOptimization(*networkID, *gatewayID, *maxTries); err != nil {
		log.Fatalf("Location optimization failed: %v", err)
	}

	fmt.Printf("Location optimization finished. Optimized parameters for %s saved to %s\n", *gatewayID, tms.MyConfiguration.GatewayParamsFile)
}
