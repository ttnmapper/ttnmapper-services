package database

import (
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
)

func TestConnectionSettingsAreQuotedAndApplicationNameBounded(t *testing.T) {
	options := PostgresOptions{Host: "/tmp/socket dir", Port: "55439", User: "test user", Password: "quote' slash\\ space", Database: "test database", ApplicationName: strings.Repeat("x", 80)}
	parsed, err := pgx.ParseConfig(options.DSN())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != options.Host || parsed.Port != 55439 || parsed.User != options.User || parsed.Password != options.Password || parsed.Database != options.Database || len(parsed.RuntimeParams["application_name"]) != 63 || parsed.RuntimeParams["sslmode"] != "" {
		t.Fatalf("connection settings were not preserved")
	}
	if parsed.TLSConfig != nil {
		t.Fatal("legacy sslmode=disable policy changed")
	}
}
