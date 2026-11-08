package tms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigurationAcceptsNumericPostgresPort(t *testing.T) {
	originalConfiguration := MyConfiguration
	t.Cleanup(func() {
		MyConfiguration = originalConfiguration
	})

	originalPort, hadPort := os.LookupEnv("POSTGRES_PORT")
	if err := os.Unsetenv("POSTGRES_PORT"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadPort {
			_ = os.Setenv("POSTGRES_PORT", originalPort)
		} else {
			_ = os.Unsetenv("POSTGRES_PORT")
		}
	})

	filename := filepath.Join(t.TempDir(), "conf.json")
	if err := os.WriteFile(filename, []byte(`{"PostgresPort": 6432}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadConfiguration(filename); err != nil {
		t.Fatalf("LoadConfiguration() error = %v", err)
	}
	if MyConfiguration.PostgresPort != 6432 {
		t.Fatalf("PostgresPort = %d, want 6432", MyConfiguration.PostgresPort)
	}
}
