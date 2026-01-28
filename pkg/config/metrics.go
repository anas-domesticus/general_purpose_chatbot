package config

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// MetricsConfig holds metrics collection and exposure settings
type MetricsConfig struct {
	// EnableHttpMetrics enables HTTP request counter and duration metrics
	EnableHttpMetrics bool `env:"METRICS_ENABLE_HTTP" yaml:"enable_http_metrics" default:"false"`

	// EnableJobMetrics enables job processing metrics (total, success, failed, killed)
	EnableJobMetrics bool `env:"METRICS_ENABLE_JOB" yaml:"enable_job_metrics" default:"false"`

	// Port is the HTTP port for the Prometheus /metrics endpoint
	Port int `env:"METRICS_PORT" yaml:"metrics_port" default:"9090"`

	// ExposeMetrics determines whether to start the metrics HTTP server
	ExposeMetrics bool `env:"METRICS_EXPOSE" yaml:"expose_metrics" default:"false"`
}

// Validate checks MetricsConfig for valid port range when metrics are exposed
func (m MetricsConfig) Validate() error {
	var result error
	// Only validate port if metrics are being exposed
	if m.ExposeMetrics && (m.Port < 1 || m.Port > 65535) {
		result = multierror.Append(result, fmt.Errorf("metrics port must be between 1-65535, got %d", m.Port))
	}
	return result
}