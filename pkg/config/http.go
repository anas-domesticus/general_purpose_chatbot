package config

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
)

// HTTPServerConfig holds HTTP server settings
type HTTPServerConfig struct {
	// Port is the TCP port for the HTTP server to listen on
	Port int `env:"HTTP_PORT" yaml:"http_port" default:"8080"`

	// ReadTimeoutSeconds is the maximum duration for reading the entire request, including body
	ReadTimeoutSeconds int `env:"HTTP_READ_TIMEOUT_SECONDS" yaml:"read_timeout_seconds" default:"15"`

	// WriteTimeoutSeconds is the maximum duration before timing out writes of the response
	WriteTimeoutSeconds int `env:"HTTP_WRITE_TIMEOUT_SECONDS" yaml:"write_timeout_seconds" default:"15"`

	// IdleTimeoutSeconds is the maximum amount of time to wait for the next request
	IdleTimeoutSeconds int `env:"HTTP_IDLE_TIMEOUT_SECONDS" yaml:"idle_timeout_seconds" default:"60"`

	// MaxHeaderBytes controls the maximum number of bytes the server will read parsing request headers
	MaxHeaderBytes int `env:"HTTP_MAX_HEADER_BYTES" yaml:"max_header_bytes" default:"1048576"`
}

// Validate checks HTTPServerConfig for valid port range
func (h HTTPServerConfig) Validate() error {
	var result error
	if h.Port < 1 || h.Port > 65535 {
		result = multierror.Append(result, fmt.Errorf("http port must be between 1-65535, got %d", h.Port))
	}
	return result
}

// ReadTimeout returns the ReadTimeoutSeconds as a time.Duration
func (h HTTPServerConfig) ReadTimeout() time.Duration {
	return time.Duration(h.ReadTimeoutSeconds) * time.Second
}

// WriteTimeout returns the WriteTimeoutSeconds as a time.Duration
func (h HTTPServerConfig) WriteTimeout() time.Duration {
	return time.Duration(h.WriteTimeoutSeconds) * time.Second
}

// IdleTimeout returns the IdleTimeoutSeconds as a time.Duration
func (h HTTPServerConfig) IdleTimeout() time.Duration {
	return time.Duration(h.IdleTimeoutSeconds) * time.Second
}
