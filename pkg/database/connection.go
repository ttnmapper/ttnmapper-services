package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"strings"
)

// PostgresOptions supplies connection settings without coupling a service to the
// root worker's global handle or caches. Port remains a string for existing config.
type PostgresOptions struct {
	Host, Port, User, Password, Database string
	DebugLog                             bool
	ApplicationName                      string
}

func quoteConnectionValue(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "\\'") + "'"
}
func (options PostgresOptions) DSN() string {
	name := options.ApplicationName
	if name == "" {
		name = filepath.Base(os.Args[0])
	}
	if len(name) > 63 {
		name = name[:63]
	}
	return "host=" + quoteConnectionValue(options.Host) + " port=" + quoteConnectionValue(options.Port) + " user=" + quoteConnectionValue(options.User) + " dbname=" + quoteConnectionValue(options.Database) + " password=" + quoteConnectionValue(options.Password) + " sslmode=disable application_name=" + quoteConnectionValue(name)
}
func OpenPostgres(options PostgresOptions) (*gorm.DB, error) {
	level := logger.Silent
	if options.DebugLog {
		level = logger.Info
	}
	return gorm.Open(postgres.Open(options.DSN()), &gorm.Config{Logger: logger.Default.LogMode(level), CreateBatchSize: 1000})
}
