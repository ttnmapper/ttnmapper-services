// Package config overlays explicitly tagged environment variables on the
// hardcoded defaults supplied by each service. It never reads configuration files.
package config

import (
	"errors"

	"github.com/caarlos0/env/v11"
)

func Environment(target interface{}) error {
	err := env.ParseWithOptions(target, env.Options{UseFieldNameByDefault: true})
	// Parser errors can include the supplied value (including numeric APRS
	// passwords). Keep the field and type for diagnosis, but omit the value.
	var aggregate env.AggregateError
	if errors.As(err, &aggregate) {
		for i, item := range aggregate.Errors {
			if parse, ok := item.(env.ParseError); ok {
				parse.Err = errors.New("invalid value")
				aggregate.Errors[i] = parse
			}
		}
		return aggregate
	}
	return err
}
