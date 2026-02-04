package config

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// CommonConfig holds common configuration shared across all services
type CommonConfig struct {
	// LogLevel specifies the minimum log level to output
	// Valid values: debug, info, warn, error
	LogLevel string `env:"LOG_LEVEL" yaml:"log_level" default:"info"`
}

// Validate checks CommonConfig for valid log level
func (c CommonConfig) Validate() error {
	var result error
	validLevels := []string{"debug", "info", "warn", "error"}
	level := strings.ToLower(c.LogLevel)

	valid := false
	for _, validLevel := range validLevels {
		if level == validLevel {
			valid = true
			break
		}
	}

	if !valid {
		result = multierror.Append(result, fmt.Errorf("log_level must be one of [debug, info, warn, error], got %q", c.LogLevel))
	}

	return result
}
