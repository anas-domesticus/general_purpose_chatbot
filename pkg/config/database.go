package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
)

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	// URL is the complete database URL (takes precedence if provided)
	URL string `env:"DATABASE_URL" yaml:"url" default:""`

	// Connection components (used if URL is not provided)
	Host     string `env:"DB_HOST" yaml:"host" default:"localhost"`
	Port     int    `env:"DB_PORT" yaml:"port" default:"5432"`
	Database string `env:"DB_NAME" yaml:"database" default:"chatbot"`
	Username string `env:"DB_USER" yaml:"username" default:"postgres"`
	Password string `env:"DB_PASSWORD" yaml:"password" default:"postgres"`
	SSLMode  string `env:"DB_SSLMODE" yaml:"sslmode" default:"disable"`

	// Connection pool settings
	MaxConnections    int    `env:"DB_MAX_CONNECTIONS" yaml:"max_connections" default:"25"`
	MinConnections    int    `env:"DB_MIN_CONNECTIONS" yaml:"min_connections" default:"5"`
	MaxIdleTime       string `env:"DB_MAX_IDLE_TIME" yaml:"max_idle_time" default:"5m"`
	MaxLifetime       string `env:"DB_MAX_LIFETIME" yaml:"max_lifetime" default:"30m"`
	ConnectTimeout    string `env:"DB_CONNECT_TIMEOUT" yaml:"connect_timeout" default:"10s"`
	StatementTimeout  string `env:"DB_STATEMENT_TIMEOUT" yaml:"statement_timeout" default:"30s"`
}

// GetConnectionString returns the database connection string
func (d DatabaseConfig) GetConnectionString() string {
	if d.URL != "" {
		return d.URL
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.Username, d.Password, d.Host, d.Port, d.Database, d.SSLMode)
}

// Validate checks DatabaseConfig for valid settings
func (d DatabaseConfig) Validate() error {
	var result error

	if d.URL == "" {
		// Validate individual components if URL is not provided
		if d.Host == "" {
			result = multierror.Append(result, fmt.Errorf("database host is required"))
		}
		if d.Port < 1 || d.Port > 65535 {
			result = multierror.Append(result, fmt.Errorf("database port must be between 1-65535, got %d", d.Port))
		}
		if d.Database == "" {
			result = multierror.Append(result, fmt.Errorf("database name is required"))
		}
		if d.Username == "" {
			result = multierror.Append(result, fmt.Errorf("database username is required"))
		}
	}

	// Validate pool settings
	if d.MaxConnections < 1 {
		result = multierror.Append(result, fmt.Errorf("max_connections must be positive, got %d", d.MaxConnections))
	}
	if d.MinConnections < 0 {
		result = multierror.Append(result, fmt.Errorf("min_connections must be non-negative, got %d", d.MinConnections))
	}
	if d.MinConnections > d.MaxConnections {
		result = multierror.Append(result, fmt.Errorf("min_connections (%d) cannot exceed max_connections (%d)", d.MinConnections, d.MaxConnections))
	}

	return result
}

// GetConnectionConfig returns the pgxpool configuration string with pool settings
func (d DatabaseConfig) GetConnectionConfig() string {
	baseURL := d.GetConnectionString()

	// Parse durations for timeout calculations
	connectTimeout, _ := time.ParseDuration(d.ConnectTimeout)
	statementTimeout, _ := time.ParseDuration(d.StatementTimeout)

	// Add pool configuration parameters
	poolParams := fmt.Sprintf("pool_max_conns=%s&pool_min_conns=%s&pool_max_conn_idle_time=%s&pool_max_conn_lifetime=%s",
		strconv.Itoa(d.MaxConnections),
		strconv.Itoa(d.MinConnections),
		d.MaxIdleTime,
		d.MaxLifetime)

	// Add timeouts
	timeoutParams := fmt.Sprintf("connect_timeout=%s&statement_timeout=%s",
		strconv.Itoa(int(connectTimeout.Seconds())),
		strconv.Itoa(int(statementTimeout.Milliseconds())))

	// Combine all parameters
	return fmt.Sprintf("%s&%s&%s", baseURL, poolParams, timeoutParams)
}