package tms

import "testing"

func TestLoadConfigurationUsesEnvironment(t *testing.T) {
	before := MyConfiguration
	t.Cleanup(func() { MyConfiguration = before })
	t.Setenv("POSTGRES_PORT", "6432")
	if err := LoadConfiguration(); err != nil {
		t.Fatal(err)
	}
	if MyConfiguration.PostgresPort != 6432 {
		t.Fatalf("port=%d", MyConfiguration.PostgresPort)
	}
}
func TestLoadConfigurationRejectsInvalidPort(t *testing.T) {
	before := MyConfiguration
	t.Cleanup(func() { MyConfiguration = before })
	t.Setenv("POSTGRES_PORT", "invalid")
	if err := LoadConfiguration(); err == nil {
		t.Fatal("invalid port accepted")
	}
}
