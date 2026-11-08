// Package config overlays explicitly tagged environment variables on the
// hardcoded defaults supplied by each service. It never reads configuration files.
package config

import "github.com/caarlos0/env/v11"

func Environment(target interface{}) error {
	return env.ParseWithOptions(target, env.Options{UseFieldNameByDefault: true})
}
