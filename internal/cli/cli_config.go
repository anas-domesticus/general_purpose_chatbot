package cli

import (
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
)

// Config holds all configuration for the CLI application
type Config struct {
	Common   config.CommonConfig     `yaml:"common,inline"`
	HTTP     config.HttpServerConfig `yaml:"http,inline"`
	Metrics  config.MetricsConfig    `yaml:"metrics,inline"`
	Database config.DatabaseConfig   `yaml:"database,inline"`
}