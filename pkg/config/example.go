package config

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
)

// ExampleServiceConfig shows how to combine the provided config types
// for a typical web service
type ExampleServiceConfig struct {
	CommonConfig `yaml:",inline"`          // log_level
	Http         HttpServerConfig `yaml:"http,inline"`     // http_port, timeouts, etc.
	Database     DatabaseConfig   `yaml:"database,inline"` // url
	Metrics      MetricsConfig    `yaml:"metrics,inline"`  // metrics_port, enable_http_metrics, etc.
	
	// Custom application-specific fields
	APIKey    string   `env:"API_KEY" yaml:"api_key" required:"true"`
	Debug     bool     `env:"DEBUG" yaml:"debug" default:"false"`
	Features  []string `env:"FEATURES" yaml:"features"`
	MaxWorkers int     `env:"MAX_WORKERS" yaml:"max_workers" default:"10"`
}

// Validate implements the Validator interface
// This ensures all embedded config types are validated
func (c ExampleServiceConfig) Validate() error {
	var result error
	
	// Validate embedded configs
	if err := c.CommonConfig.Validate(); err != nil {
		result = multierror.Append(result, err)
	}
	if err := c.Http.Validate(); err != nil {
		result = multierror.Append(result, err)
	}
	if err := c.Metrics.Validate(); err != nil {
		result = multierror.Append(result, err)
	}
	
	// Custom validation
	if c.MaxWorkers < 1 {
		result = multierror.Append(result, fmt.Errorf("max_workers must be >= 1, got %d", c.MaxWorkers))
	}
	
	return result
}

/*
Example usage in main.go:

package main

import (
	"fmt"
	"net/http"
	
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
)

func main() {
	var cfg config.ExampleServiceConfig
	
	// Load from config.yaml, fallback to env vars if file missing
	if err := config.GetConfig(&cfg, "config.yaml", true); err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}
	
	// Use the config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Http.Port),
		ReadTimeout:  cfg.Http.ReadTimeout(),
		WriteTimeout: cfg.Http.WriteTimeout(),
		IdleTimeout:  cfg.Http.IdleTimeout(),
	}
	
	fmt.Printf("Starting server on port %d with log level %s\n", cfg.Http.Port, cfg.LogLevel)
	server.ListenAndServe()
}

Example config.yaml:
---
log_level: info
http_port: 8080
read_timeout_seconds: 30
write_timeout_seconds: 30
database:
  url: postgres://user:pass@localhost:5432/myapp
metrics:
  expose_metrics: true
  enable_http_metrics: true
  metrics_port: 9090
api_key: your-secret-key
debug: false
max_workers: 5
*/