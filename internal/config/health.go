package config

import "time"

// HealthConfig holds health check configuration
type HealthConfig struct {
	Enabled          bool          `env:"HEALTH_ENABLED" yaml:"enabled" default:"true"`
	Port             int           `env:"HEALTH_PORT" yaml:"port" default:"8080"`
	LivenessPath     string        `env:"HEALTH_LIVENESS_PATH" yaml:"liveness_path" default:"/health/live"`
	ReadinessPath    string        `env:"HEALTH_READINESS_PATH" yaml:"readiness_path" default:"/health/ready"`
	CombinedPath     string        `env:"HEALTH_COMBINED_PATH" yaml:"combined_path" default:"/health"`
	Timeout          time.Duration `env:"HEALTH_TIMEOUT" yaml:"timeout" default:"10s"`
	FailureThreshold int           `env:"HEALTH_FAILURE_THRESHOLD" yaml:"failure_threshold" default:"3"`
}
