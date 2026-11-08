// Package config applies service defaults, an optional JSON/YAML file, then
// explicitly tagged environment variables. Service-specific settings stay local.
package config

import (
	"errors"
	"github.com/caarlos0/env/v11"
	"github.com/ghodss/yaml"
	"os"
)

// Environment retains the environment-only behavior of the original root workers.
func Environment(target interface{}) error {
	return env.ParseWithOptions(target, env.Options{UseFieldNameByDefault: true})
}

// Load retains the imported services' JSON/YAML compatibility. A missing file
// allows environment-only deployments; malformed/unreadable files return errors.
func Load(filename string, target interface{}) error {
	if filename != "" {
		data, err := os.ReadFile(filename)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			if err := yaml.Unmarshal(data, target); err != nil {
				return err
			}
		}
	}
	return Environment(target)
}
