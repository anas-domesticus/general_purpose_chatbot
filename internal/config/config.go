// Package config provides application configuration types and validation.
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
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" yaml:"request_timeout" default:"30s"`

	// LLM Provider configuration
	LLM LLMConfig `yaml:"llm"`

	// Anthropic/Claude configuration
	Anthropic AnthropicConfig `yaml:"anthropic"`

	// Gemini configuration
	Gemini GeminiConfig `yaml:"gemini"`

	// OpenAI configuration
	OpenAI OpenAIConfig `yaml:"openai"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`

	// Monitoring configuration
	Monitoring MonitoringConfig `yaml:"monitoring"`

	// Database configuration (optional)
	Database DatabaseConfig `yaml:"database"`

	// Security configuration
	Security SecurityConfig `yaml:"security"`

	// MCP (Model Context Protocol) configuration
	MCP MCPConfig `yaml:"mcp"`

	// Slack configuration
	Slack SlackConfig `yaml:"slack"`

	// Telegram configuration
	Telegram TelegramConfig `yaml:"telegram"`

	// Storage configuration (persistence layer)
	Storage StorageConfig `yaml:"storage"`

	// Health check configuration
	Health HealthConfig `yaml:"health"`
}

// Validate validates the configuration and returns an error if invalid
//
//nolint:gocyclo,gocognit,revive // Config validation inherently requires checking many fields
func (c *AppConfig) Validate() error {
	var result error

	// Validate LLM provider
	provider := strings.ToLower(c.LLM.Provider)
	if provider != ProviderClaude && provider != ProviderGemini && provider != ProviderOpenAI {
		result = multierror.Append(result, fmt.Errorf("llm_provider must be 'claude', 'gemini', or 'openai', got %q", c.LLM.Provider))
	}

	// Validate provider-specific configuration
	if provider == ProviderClaude {
		if c.Anthropic.APIKey == "" {
			result = multierror.Append(result, fmt.Errorf("anthropic_api_key is required when using claude provider"))
		}
	}
	if provider == ProviderGemini {
		if c.Gemini.APIKey == "" {
			result = multierror.Append(result, fmt.Errorf("gemini_api_key is required when using gemini provider"))
		}
	}
	if provider == ProviderOpenAI {
		if c.OpenAI.APIKey == "" {
			result = multierror.Append(result, fmt.Errorf("openai_api_key is required when using openai provider"))
		}
	}

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

	// Validate timeout values
	if c.RequestTimeout <= 0 {
		result = multierror.Append(result, fmt.Errorf("request_timeout must be greater than 0"))
	}

	// Validate Anthropic-specific config if using Claude
	if provider == "claude" {
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
			validTransports := []string{"stdio", "websocket", "sse", "http"}
			validTransport := false
			for _, transport := range validTransports {
				if serverConfig.Transport == transport {
					validTransport = true
					break
				}
			}
			if !validTransport {
				result = multierror.Append(result, fmt.Errorf(
					"MCP server '%s': transport must be one of [stdio, websocket, sse, http], got %q",
					serverName, serverConfig.Transport))
			}

			// Validate stdio configuration
			if serverConfig.Transport == "stdio" {
				if serverConfig.Command == "" {
					result = multierror.Append(result, fmt.Errorf("MCP server '%s': command is required for stdio transport", serverName))
				}
			}

			// Validate websocket/sse/http configuration
			if serverConfig.Transport == "websocket" || serverConfig.Transport == "sse" || serverConfig.Transport == "http" {
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
					result = multierror.Append(result, fmt.Errorf(
						"MCP server '%s': auth type must be one of [bearer, basic, api_key], got %q",
						serverName, serverConfig.Auth.Type))
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

	// Validate health config (if enabled)
	if c.Health.Enabled {
		if c.Health.Port < 1 || c.Health.Port > 65535 {
			result = multierror.Append(result, fmt.Errorf("health_port must be between 1 and 65535, got %d", c.Health.Port))
		}

		if c.Health.Timeout <= 0 {
			result = multierror.Append(result, fmt.Errorf("health_timeout must be greater than 0"))
		}

		if c.Health.FailureThreshold <= 0 {
			result = multierror.Append(result, fmt.Errorf("health_failure_threshold must be greater than 0"))
		}

		if c.Health.LivenessPath == "" {
			result = multierror.Append(result, fmt.Errorf("health_liveness_path cannot be empty"))
		}

		if c.Health.ReadinessPath == "" {
			result = multierror.Append(result, fmt.Errorf("health_readiness_path cannot be empty"))
		}

		if c.Health.CombinedPath == "" {
			result = multierror.Append(result, fmt.Errorf("health_combined_path cannot be empty"))
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

// GetLLMModel returns the model name for the configured LLM provider
func (c *AppConfig) GetLLMModel() string {
	switch strings.ToLower(c.LLM.Provider) {
	case "gemini":
		return c.Gemini.Model
	case "openai":
		return c.OpenAI.Model
	default:
		return c.Anthropic.Model
	}
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
		logger.StringField("llm_provider", c.LLM.Provider),
		logger.StringField("llm_model", c.GetLLMModel()),
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

	// Log Telegram configuration
	if c.Telegram.Enabled() {
		log.Info("Telegram integration enabled")
	}

	// Log storage configuration
	log.Info("Storage configured",
		logger.StringField("backend", c.Storage.Backend),
	)

	// Log health check configuration
	if c.Health.Enabled {
		log.Info("Health checks enabled",
			logger.IntField("port", c.Health.Port),
			logger.StringField("liveness_path", c.Health.LivenessPath),
			logger.StringField("readiness_path", c.Health.ReadinessPath),
			logger.StringField("combined_path", c.Health.CombinedPath),
			logger.DurationField("timeout", c.Health.Timeout),
			logger.IntField("failure_threshold", c.Health.FailureThreshold),
		)
	}
}
