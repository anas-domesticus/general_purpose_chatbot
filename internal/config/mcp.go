package config

import "time"

// MCPConfig holds Model Context Protocol configuration
type MCPConfig struct {
	Enabled   bool                       `env:"MCP_ENABLED" yaml:"enabled" default:"false"`
	Servers   map[string]MCPServerConfig `yaml:"servers"`
	Discovery MCPDiscoveryConfig         `yaml:"discovery"`
	Timeout   time.Duration              `env:"MCP_TIMEOUT" yaml:"timeout" default:"30s"`
}

// MCPServerConfig holds configuration for individual MCP servers
type MCPServerConfig struct {
	Name        string            `yaml:"name"`
	Transport   string            `yaml:"transport"` // stdio, websocket, sse
	Command     string            `yaml:"command,omitempty"`
	Args        []string          `yaml:"args,omitempty"`
	URL         string            `yaml:"url,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	Auth        *MCPAuthConfig    `yaml:"auth,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	ToolFilter  []string          `yaml:"tool_filter,omitempty"`
	Enabled     bool              `yaml:"enabled" default:"true"`
}

// MCPAuthConfig holds authentication configuration for MCP servers
type MCPAuthConfig struct {
	Type   string `yaml:"type"` // bearer, basic, api_key
	Token  string `yaml:"token,omitempty"`
	User   string `yaml:"user,omitempty"`
	Pass   string `yaml:"pass,omitempty"`
	APIKey string `yaml:"api_key,omitempty"`
	Header string `yaml:"header,omitempty"`
}

// MCPDiscoveryConfig holds configuration for MCP server discovery
type MCPDiscoveryConfig struct {
	Enabled         bool          `yaml:"enabled" default:"true"`
	RefreshInterval time.Duration `yaml:"refresh_interval" default:"5m"`
	HealthChecks    bool          `yaml:"health_checks" default:"true"`
}
