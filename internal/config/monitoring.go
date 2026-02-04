package config

import "time"

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	HealthCheckTimeout time.Duration `env:"HEALTH_CHECK_TIMEOUT" yaml:"health_check_timeout" default:"10s"`
	MetricsEnabled     bool          `env:"METRICS_ENABLED" yaml:"metrics_enabled" default:"true"`
	MetricsPort        int           `env:"METRICS_PORT" yaml:"metrics_port" default:"9090"`
}
