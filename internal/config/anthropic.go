package config

import "time"

// AnthropicConfig holds Anthropic-specific configuration
type AnthropicConfig struct {
	APIKey         string        `env:"ANTHROPIC_API_KEY" yaml:"api_key"`
	Model          string        `env:"CLAUDE_MODEL" yaml:"model" default:"claude-sonnet-4-5-20250929"`
	APIBaseURL     string        `env:"ANTHROPIC_API_URL" yaml:"api_base_url" default:"https://api.anthropic.com"`
	MaxRetries     int           `env:"ANTHROPIC_MAX_RETRIES" yaml:"max_retries" default:"3"`
	InitialBackoff time.Duration `env:"ANTHROPIC_INITIAL_BACKOFF" yaml:"initial_backoff" default:"1s"`
	MaxBackoff     time.Duration `env:"ANTHROPIC_MAX_BACKOFF" yaml:"max_backoff" default:"10s"`
	Timeout        time.Duration `env:"ANTHROPIC_TIMEOUT" yaml:"timeout" default:"30s"`
}

// AnthropicRetryConfig represents retry configuration for Anthropic
type AnthropicRetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}
