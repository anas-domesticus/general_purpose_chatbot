package config

import "time"

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string        `env:"DATABASE_URL" yaml:"-"`
	MaxConnections  int           `env:"DATABASE_MAX_CONNECTIONS" yaml:"max_connections" default:"25"`
	ConnMaxLifetime time.Duration `env:"DATABASE_CONN_MAX_LIFETIME" yaml:"conn_max_lifetime" default:"5m"`
	ConnMaxIdleTime time.Duration `env:"DATABASE_CONN_MAX_IDLE_TIME" yaml:"conn_max_idle_time" default:"5m"`
}
