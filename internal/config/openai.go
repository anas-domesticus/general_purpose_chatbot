package config

import "time"

// OpenAIConfig holds OpenAI-specific configuration
type OpenAIConfig struct {
	APIKey     string        `env:"OPENAI_API_KEY" yaml:"api_key"`
	Model      string        `env:"OPENAI_MODEL" yaml:"model" default:"gpt-4"`
	APIBaseURL string        `env:"OPENAI_API_URL" yaml:"api_base_url" default:"https://api.openai.com/v1"`
	MaxRetries int           `env:"OPENAI_MAX_RETRIES" yaml:"max_retries" default:"3"`
	Timeout    time.Duration `env:"OPENAI_TIMEOUT" yaml:"timeout" default:"30s"`
}
