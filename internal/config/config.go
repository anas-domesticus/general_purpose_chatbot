package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// AppConfig holds all application configuration
type AppConfig struct {
	// Service configuration
	ServiceName string `env:"SERVICE_NAME" yaml:"service_name" default:"general-purpose-chatbot"`
	Version     string `env:"VERSION" yaml:"version" default:"dev"`
	Environment string `env:"ENVIRONMENT" yaml:"environment" default:"development"`

	// Server configuration
	Port           int           `env:"PORT" yaml:"port" default:"8080"`
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" yaml:"request_timeout" default:"30s"`
	IdleTimeout    time.Duration `env:"IDLE_TIMEOUT" yaml:"idle_timeout" default:"60s"`

	// Anthropic/Claude configuration
	Anthropic AnthropicConfig `yaml:"anthropic,inline"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging,inline"`

	// Monitoring configuration
	Monitoring MonitoringConfig `yaml:"monitoring,inline"`

	// Database configuration (optional)
	Database DatabaseConfig `yaml:"database,inline"`

	// Security configuration
	Security SecurityConfig `yaml:"security,inline"`

	// MCP (Model Context Protocol) configuration
	MCP MCPConfig `yaml:"mcp,inline"`

	// Slack configuration
	Slack SlackConfig `yaml:"slack,inline"`
}

// SlackConfig holds Slack-specific configuration
type SlackConfig struct {
	BotToken string `env:"SLACK_BOT_TOKEN" yaml:"bot_token"`
	AppToken string `env:"SLACK_APP_TOKEN" yaml:"app_token"`
}

// Enabled returns true if Slack is configured with both tokens
func (c *SlackConfig) Enabled() bool {
	return c.BotToken != "" && c.AppToken != ""
}

// AnthropicConfig holds Anthropic-specific configuration
type AnthropicConfig struct {
	APIKey         string        `env:"ANTHROPIC_API_KEY" yaml:"api_key" required:"true"`
	Model          string        `env:"CLAUDE_MODEL" yaml:"model" default:"claude-3-5-sonnet-20241022"`
	APIBaseURL     string        `env:"ANTHROPIC_API_URL" yaml:"api_base_url" default:"https://api.anthropic.com"`
	MaxRetries     int           `env:"ANTHROPIC_MAX_RETRIES" yaml:"max_retries" default:"3"`
	InitialBackoff time.Duration `env:"ANTHROPIC_INITIAL_BACKOFF" yaml:"initial_backoff" default:"1s"`
	MaxBackoff     time.Duration `env:"ANTHROPIC_MAX_BACKOFF" yaml:"max_backoff" default:"10s"`
	Timeout        time.Duration `env:"ANTHROPIC_TIMEOUT" yaml:"timeout" default:"30s"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL" yaml:"level" default:"info"`
	Format string `env:"LOG_FORMAT" yaml:"format" default:"json"`
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	HealthCheckTimeout time.Duration `env:"HEALTH_CHECK_TIMEOUT" yaml:"health_check_timeout" default:"10s"`
	MetricsEnabled     bool          `env:"METRICS_ENABLED" yaml:"metrics_enabled" default:"true"`
	MetricsPort        int           `env:"METRICS_PORT" yaml:"metrics_port" default:"9090"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string        `env:"DATABASE_URL" yaml:"url"`
	MaxConnections  int           `env:"DATABASE_MAX_CONNECTIONS" yaml:"max_connections" default:"25"`
	ConnMaxLifetime time.Duration `env:"DATABASE_CONN_MAX_LIFETIME" yaml:"conn_max_lifetime" default:"5m"`
	ConnMaxIdleTime time.Duration `env:"DATABASE_CONN_MAX_IDLE_TIME" yaml:"conn_max_idle_time" default:"5m"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" yaml:"cors_allowed_origins" default:"http://localhost:3000,http://localhost:8080"`
	MaxRequestSize     int64    `env:"MAX_REQUEST_SIZE" yaml:"max_request_size" default:"10485760"` // 10MB default
	RateLimitEnabled   bool     `env:"RATE_LIMIT_ENABLED" yaml:"rate_limit_enabled" default:"true"`
	RateLimitRPS       int      `env:"RATE_LIMIT_RPS" yaml:"rate_limit_rps" default:"100"`
}

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

// Validate validates the configuration and returns an error if invalid
func (c *AppConfig) Validate() error {
	var result error

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	level := strings.ToLower(c.Logging.Level)
	valid := false
	for _, validLevel := range validLevels {
		if level == validLevel {
			valid = true
			break
		}
	}
	if !valid {
		result = multierror.Append(result, fmt.Errorf("log_level must be one of [debug, info, warn, error], got %q", c.Logging.Level))
	}

	// Validate log format
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		result = multierror.Append(result, fmt.Errorf("log_format must be either 'json' or 'text', got %q", c.Logging.Format))
	}

	// Validate port range
	if c.Port < 1 || c.Port > 65535 {
		result = multierror.Append(result, fmt.Errorf("port must be between 1 and 65535, got %d", c.Port))
	}

	// Validate timeout values
	if c.RequestTimeout <= 0 {
		result = multierror.Append(result, fmt.Errorf("request_timeout must be greater than 0"))
	}

	if c.Anthropic.Timeout <= 0 {
		result = multierror.Append(result, fmt.Errorf("anthropic_timeout must be greater than 0"))
	}

	// Validate retry configuration
	if c.Anthropic.MaxRetries < 0 {
		result = multierror.Append(result, fmt.Errorf("anthropic_max_retries cannot be negative"))
	}

	if c.Anthropic.InitialBackoff <= 0 {
		result = multierror.Append(result, fmt.Errorf("anthropic_initial_backoff must be greater than 0"))
	}

	if c.Anthropic.MaxBackoff < c.Anthropic.InitialBackoff {
		result = multierror.Append(result, fmt.Errorf("anthropic_max_backoff must be greater than or equal to anthropic_initial_backoff"))
	}

	// Validate security config
	if c.Security.MaxRequestSize <= 0 {
		result = multierror.Append(result, fmt.Errorf("max_request_size must be greater than 0"))
	}

	if c.Security.RateLimitRPS <= 0 {
		result = multierror.Append(result, fmt.Errorf("rate_limit_rps must be greater than 0"))
	}

	// Validate database config (if configured)
	if c.Database.URL != "" && c.Database.MaxConnections <= 0 {
		result = multierror.Append(result, fmt.Errorf("database_max_connections must be greater than 0 when database is configured"))
	}

	// Validate MCP config (if enabled)
	if c.MCP.Enabled {
		if c.MCP.Timeout <= 0 {
			result = multierror.Append(result, fmt.Errorf("mcp_timeout must be greater than 0 when MCP is enabled"))
		}

		// Validate each MCP server configuration
		for serverName, serverConfig := range c.MCP.Servers {
			if !serverConfig.Enabled {
				continue
			}

			// Validate transport type
			validTransports := []string{"stdio", "websocket", "sse"}
			validTransport := false
			for _, transport := range validTransports {
				if serverConfig.Transport == transport {
					validTransport = true
					break
				}
			}
			if !validTransport {
				result = multierror.Append(result, fmt.Errorf("MCP server '%s': transport must be one of [stdio, websocket, sse], got %q", serverName, serverConfig.Transport))
			}

			// Validate stdio configuration
			if serverConfig.Transport == "stdio" {
				if serverConfig.Command == "" {
					result = multierror.Append(result, fmt.Errorf("MCP server '%s': command is required for stdio transport", serverName))
				}
			}

			// Validate websocket/sse configuration
			if serverConfig.Transport == "websocket" || serverConfig.Transport == "sse" {
				if serverConfig.URL == "" {
					result = multierror.Append(result, fmt.Errorf("MCP server '%s': url is required for %s transport", serverName, serverConfig.Transport))
				}
			}

			// Validate auth configuration
			if serverConfig.Auth != nil {
				validAuthTypes := []string{"bearer", "basic", "api_key"}
				validAuth := false
				for _, authType := range validAuthTypes {
					if serverConfig.Auth.Type == authType {
						validAuth = true
						break
					}
				}
				if !validAuth {
					result = multierror.Append(result, fmt.Errorf("MCP server '%s': auth type must be one of [bearer, basic, api_key], got %q", serverName, serverConfig.Auth.Type))
				}

				// Validate auth fields based on type
				switch serverConfig.Auth.Type {
				case "bearer":
					if serverConfig.Auth.Token == "" {
						result = multierror.Append(result, fmt.Errorf("MCP server '%s': token is required for bearer auth", serverName))
					}
				case "basic":
					if serverConfig.Auth.User == "" || serverConfig.Auth.Pass == "" {
						result = multierror.Append(result, fmt.Errorf("MCP server '%s': user and pass are required for basic auth", serverName))
					}
				case "api_key":
					if serverConfig.Auth.APIKey == "" || serverConfig.Auth.Header == "" {
						result = multierror.Append(result, fmt.Errorf("MCP server '%s': api_key and header are required for api_key auth", serverName))
					}
				}
			}
		}
	}

	return result
}

// GetLogLevel returns the parsed logger level
func (c *AppConfig) GetLogLevel() logger.Level {
	switch strings.ToLower(c.Logging.Level) {
	case "debug":
		return logger.DebugLevel
	case "warn", "warning":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}

// IsProduction returns true if running in production environment
func (c *AppConfig) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

// IsDevelopment returns true if running in development environment
func (c *AppConfig) IsDevelopment() bool {
	env := strings.ToLower(c.Environment)
	return env == "development" || env == "dev"
}

// GetAnthropicRetryConfig returns retry configuration for Anthropic client
func (c *AppConfig) GetAnthropicRetryConfig() AnthropicRetryConfig {
	return AnthropicRetryConfig{
		MaxRetries:     c.Anthropic.MaxRetries,
		InitialBackoff: c.Anthropic.InitialBackoff,
		MaxBackoff:     c.Anthropic.MaxBackoff,
	}
}

// AnthropicRetryConfig represents retry configuration for Anthropic
type AnthropicRetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// LogConfig logs the current configuration (without sensitive data)
func (c *AppConfig) LogConfig(log logger.Logger) {
	// Count enabled MCP servers
	enabledMCPServers := 0
	mcpServerNames := make([]string, 0)
	for name, server := range c.MCP.Servers {
		if server.Enabled {
			enabledMCPServers++
			mcpServerNames = append(mcpServerNames, name)
		}
	}

	log.Info("Application configuration loaded",
		logger.StringField("service_name", c.ServiceName),
		logger.StringField("version", c.Version),
		logger.StringField("environment", c.Environment),
		logger.IntField("port", c.Port),
		logger.StringField("claude_model", c.Anthropic.Model),
		logger.StringField("log_level", c.Logging.Level),
		logger.StringField("log_format", c.Logging.Format),
		logger.BoolField("metrics_enabled", c.Monitoring.MetricsEnabled),
		logger.BoolField("database_configured", c.Database.URL != ""),
		logger.BoolField("rate_limit_enabled", c.Security.RateLimitEnabled),
		logger.IntField("rate_limit_rps", c.Security.RateLimitRPS),
		logger.BoolField("mcp_enabled", c.MCP.Enabled),
		logger.IntField("mcp_servers_enabled", enabledMCPServers),
	)

	// Log MCP server details if enabled
	if c.MCP.Enabled && len(mcpServerNames) > 0 {
		log.Info("MCP servers configured", logger.StringField("servers", strings.Join(mcpServerNames, ", ")))
	}

	// Log Slack configuration
	if c.Slack.Enabled() {
		log.Info("Slack integration enabled")
	}
}
