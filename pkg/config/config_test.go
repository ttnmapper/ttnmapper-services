package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationPrecedenceAndTypes(t *testing.T) {
	type settings struct {
		Host      string `env:"TEST_CONFIG_HOST"`
		Port      int    `env:"TEST_CONFIG_PORT"`
		Enabled   bool   `env:"TEST_CONFIG_ENABLED"`
		Unchanged string `env:"TEST_CONFIG_UNCHANGED"`
	}
	file := filepath.Join(t.TempDir(), "conf.json")
	if err := os.WriteFile(file, []byte(`{"Host":"file","Port":5433,"Enabled":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_CONFIG_HOST", "environment")
	t.Setenv("TEST_CONFIG_PORT", "6432")
	t.Setenv("TEST_CONFIG_ENABLED", "true")
	got := settings{Host: "default", Port: 5432, Unchanged: "default"}
	if err := Load(file, &got); err != nil {
		t.Fatal(err)
	}
	if got.Host != "environment" || got.Port != 6432 || !got.Enabled || got.Unchanged != "default" {
		t.Fatalf("configuration=%+v", got)
	}
}
func TestMissingFileStillLoadsEnvironment(t *testing.T) {
	var got struct {
		Value string `env:"TEST_CONFIG_VALUE"`
	}
	t.Setenv("TEST_CONFIG_VALUE", "value")
	if err := Load(filepath.Join(t.TempDir(), "missing"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "value" {
		t.Fatalf("configuration=%+v", got)
	}
}
func TestEnvironmentOnlyDoesNotNeedAFile(t *testing.T) {
	got := struct {
		Port string `env:"TEST_CONFIG_PORT"`
	}{Port: "5432"}
	t.Setenv("TEST_CONFIG_PORT", "6432")
	if err := Environment(&got); err != nil || got.Port != "6432" {
		t.Fatalf("configuration=%+v error=%v", got, err)
	}
}
func TestMalformedConfigurationFails(t *testing.T) {
	var got struct {
		Port int `env:"TEST_CONFIG_PORT"`
	}
	file := filepath.Join(t.TempDir(), "conf.json")
	if err := os.WriteFile(file, []byte(`{"Port":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Load(file, &got); err == nil {
		t.Fatal("invalid file accepted")
	}
	t.Setenv("TEST_CONFIG_PORT", "invalid")
	if err := Environment(&got); err == nil {
		t.Fatal("invalid port accepted")
	}
}
func TestLegacyEnvironmentNamesRemainExact(t *testing.T) {
	var got struct {
		Exchange string `env:"AMQP_EXHANGE"`
	}
	t.Setenv("AMQP_EXHANGE", "legacy")
	t.Setenv("AMQP_EXCHANGE", "different")
	if err := Environment(&got); err != nil || got.Exchange != "legacy" {
		t.Fatalf("configuration=%+v error=%v", got, err)
	}
}
