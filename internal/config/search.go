package config

import "time"

// SearchConfig holds configuration for the web search tool
type SearchConfig struct {
	APIKey  string        `env:"SEARCHAPI_API_KEY" yaml:"-"`
	BaseURL string        `env:"SEARCH_API_URL" yaml:"base_url" default:"https://api.tavily.com"`
	Timeout time.Duration `env:"SEARCH_TIMEOUT" yaml:"timeout" default:"30s"`
}

// Enabled returns true if the search API is configured with an API key
func (c *SearchConfig) Enabled() bool {
	return c.APIKey != ""
}
