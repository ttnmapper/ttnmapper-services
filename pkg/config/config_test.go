package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentOverridesDefaultsAndIgnoresConfigFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("conf.json", []byte(`{"Port":"this must not be read"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got := struct {
		Host      string `env:"TEST_CONFIG_HOST"`
		Port      int    `env:"TEST_CONFIG_PORT"`
		Enabled   bool   `env:"TEST_CONFIG_ENABLED"`
		Unchanged string `env:"TEST_CONFIG_UNCHANGED"`
	}{Host: "default", Port: 5432, Unchanged: "default"}
	t.Setenv("TEST_CONFIG_HOST", "environment")
	t.Setenv("TEST_CONFIG_PORT", "6432")
	t.Setenv("TEST_CONFIG_ENABLED", "true")
	if err := Environment(&got); err != nil {
		t.Fatal(err)
	}
	if got.Host != "environment" || got.Port != 6432 || !got.Enabled || got.Unchanged != "default" {
		t.Fatalf("settings=%+v", got)
	}
}

func TestInvalidSecretIsNotIncludedInError(t *testing.T) {
	var got struct {
		AprsRelayPassword int `env:"TEST_APRS_PASSWORD"`
	}
	t.Setenv("TEST_APRS_PASSWORD", "private-passcode")
	err := Environment(&got)
	if err == nil || !strings.Contains(err.Error(), "AprsRelayPassword") {
		t.Fatalf("expected an error identifying the invalid field, got %v", err)
	}
	if strings.Contains(err.Error(), "private-passcode") {
		t.Fatal("configuration error exposes secret")
	}
}
func TestInvalidEnvironmentReturnsError(t *testing.T) {
	var got struct {
		Port int `env:"TEST_CONFIG_PORT"`
	}
	t.Setenv("TEST_CONFIG_PORT", "invalid")
	if err := Environment(&got); err == nil {
		t.Fatal("invalid port accepted")
	}
}
func TestNoFileIsRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	var got struct {
		Value string `env:"TEST_CONFIG_VALUE"`
	}
	t.Setenv("TEST_CONFIG_VALUE", "environment")
	if err := Environment(&got); err != nil || got.Value != "environment" {
		t.Fatalf("settings=%+v error=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(".", "conf.json")); !os.IsNotExist(err) {
		t.Fatal("loader created a file")
	}
}
func TestLegacyEnvironmentNamesRemainExact(t *testing.T) {
	var got struct {
		Exchange string `env:"AMQP_EXHANGE"`
	}
	t.Setenv("AMQP_EXHANGE", "legacy")
	t.Setenv("AMQP_EXCHANGE", "different")
	if err := Environment(&got); err != nil || got.Exchange != "legacy" {
		t.Fatalf("settings=%+v error=%v", got, err)
	}
}
