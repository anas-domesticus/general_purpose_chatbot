package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// AppConfig holds all application configuration
type AppConfig struct {
	// Service configuration
	ServiceName string
	Version     string
	Environment string

	// Server configuration
	Port           int
	RequestTimeout time.Duration
	IdleTimeout    time.Duration

	// Anthropic/Claude configuration
	Anthropic AnthropicConfig

	// Logging configuration
	Logging LoggingConfig

	// Monitoring configuration
	Monitoring MonitoringConfig

	// Database configuration (optional)
	Database DatabaseConfig

	// Redis configuration (optional)
	Redis RedisConfig

	// Security configuration
	Security SecurityConfig
}

// AnthropicConfig holds Anthropic-specific configuration
type AnthropicConfig struct {
	APIKey         string
	Model          string
	APIBaseURL     string
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Timeout        time.Duration
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  logger.Level
	Format string // "json" or "text"
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	HealthCheckTimeout time.Duration
	MetricsEnabled     bool
	MetricsPort        int
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string
	MaxConnections  int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL      string
	Password string
	Database int
	Timeout  time.Duration
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	CORSAllowedOrigins []string
	MaxRequestSize     int64 // in bytes
	RateLimitEnabled   bool
	RateLimitRPS       int // requests per second
}

// LoadConfig loads configuration from environment variables with validation
func LoadConfig() (*AppConfig, error) {
	config := &AppConfig{
		ServiceName: getEnvWithDefault("SERVICE_NAME", "general-purpose-chatbot"),
		Version:     getEnvWithDefault("VERSION", "dev"),
		Environment: getEnvWithDefault("ENVIRONMENT", "development"),
		Port:        getEnvInt("PORT", 8080),
		RequestTimeout: getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
		IdleTimeout:    getEnvDuration("IDLE_TIMEOUT", 60*time.Second),

		Anthropic: AnthropicConfig{
			APIKey:         os.Getenv("ANTHROPIC_API_KEY"),
			Model:          getEnvWithDefault("CLAUDE_MODEL", "claude-3-5-sonnet-20241022"),
			APIBaseURL:     getEnvWithDefault("ANTHROPIC_API_URL", "https://api.anthropic.com"),
			MaxRetries:     getEnvInt("ANTHROPIC_MAX_RETRIES", 3),
			InitialBackoff: getEnvDuration("ANTHROPIC_INITIAL_BACKOFF", 1*time.Second),
			MaxBackoff:     getEnvDuration("ANTHROPIC_MAX_BACKOFF", 10*time.Second),
			Timeout:        getEnvDuration("ANTHROPIC_TIMEOUT", 30*time.Second),
		},

		Logging: LoggingConfig{
			Format: getEnvWithDefault("LOG_FORMAT", "json"),
		},

		Monitoring: MonitoringConfig{
			HealthCheckTimeout: getEnvDuration("HEALTH_CHECK_TIMEOUT", 10*time.Second),
			MetricsEnabled:     getEnvBool("METRICS_ENABLED", true),
			MetricsPort:        getEnvInt("METRICS_PORT", 9090),
		},

		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxConnections:  getEnvInt("DATABASE_MAX_CONNECTIONS", 25),
			ConnMaxLifetime: getEnvDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DATABASE_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},

		Redis: RedisConfig{
			URL:      os.Getenv("REDIS_URL"),
			Password: os.Getenv("REDIS_PASSWORD"),
			Database: getEnvInt("REDIS_DATABASE", 0),
			Timeout:  getEnvDuration("REDIS_TIMEOUT", 5*time.Second),
		},

		Security: SecurityConfig{
			CORSAllowedOrigins: getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:8080"}),
			MaxRequestSize:     getEnvInt64("MAX_REQUEST_SIZE", 10*1024*1024), // 10MB default
			RateLimitEnabled:   getEnvBool("RATE_LIMIT_ENABLED", true),
			RateLimitRPS:       getEnvInt("RATE_LIMIT_RPS", 100),
		},
	}

	// Parse log level
	logLevelStr := getEnvWithDefault("LOG_LEVEL", "info")
	switch strings.ToLower(logLevelStr) {
	case "debug":
		config.Logging.Level = logger.DebugLevel
	case "warn", "warning":
		config.Logging.Level = logger.WarnLevel
	case "error":
		config.Logging.Level = logger.ErrorLevel
	default:
		config.Logging.Level = logger.InfoLevel
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration and returns an error if invalid
func (c *AppConfig) Validate() error {
	var errors []string

	// Required fields
	if c.Anthropic.APIKey == "" {
		errors = append(errors, "ANTHROPIC_API_KEY is required")
	}

	if c.ServiceName == "" {
		errors = append(errors, "SERVICE_NAME cannot be empty")
	}

	// Port validation
	if c.Port < 1 || c.Port > 65535 {
		errors = append(errors, fmt.Sprintf("PORT must be between 1 and 65535, got %d", c.Port))
	}

	// Timeout validation
	if c.RequestTimeout <= 0 {
		errors = append(errors, "REQUEST_TIMEOUT must be greater than 0")
	}

	if c.Anthropic.Timeout <= 0 {
		errors = append(errors, "ANTHROPIC_TIMEOUT must be greater than 0")
	}

	// Retry configuration validation
	if c.Anthropic.MaxRetries < 0 {
		errors = append(errors, "ANTHROPIC_MAX_RETRIES cannot be negative")
	}

	if c.Anthropic.InitialBackoff <= 0 {
		errors = append(errors, "ANTHROPIC_INITIAL_BACKOFF must be greater than 0")
	}

	if c.Anthropic.MaxBackoff < c.Anthropic.InitialBackoff {
		errors = append(errors, "ANTHROPIC_MAX_BACKOFF must be greater than or equal to ANTHROPIC_INITIAL_BACKOFF")
	}

	// Log format validation
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		errors = append(errors, "LOG_FORMAT must be either 'json' or 'text'")
	}

	// Security validation
	if c.Security.MaxRequestSize <= 0 {
		errors = append(errors, "MAX_REQUEST_SIZE must be greater than 0")
	}

	if c.Security.RateLimitRPS <= 0 {
		errors = append(errors, "RATE_LIMIT_RPS must be greater than 0")
	}

	// Database validation (if configured)
	if c.Database.URL != "" {
		if c.Database.MaxConnections <= 0 {
			errors = append(errors, "DATABASE_MAX_CONNECTIONS must be greater than 0")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
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

// Helper functions for environment variable parsing
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.ParseBool(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := time.ParseDuration(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if valueStr := os.Getenv(key); valueStr != "" {
		return strings.Split(valueStr, ",")
	}
	return defaultValue
}

// LogConfig logs the current configuration (without sensitive data)
func (c *AppConfig) LogConfig(log logger.Logger) {
	log.Info("Application configuration loaded",
		logger.StringField("service_name", c.ServiceName),
		logger.StringField("version", c.Version),
		logger.StringField("environment", c.Environment),
		logger.IntField("port", c.Port),
		logger.StringField("claude_model", c.Anthropic.Model),
		logger.StringField("log_level", c.Logging.Level.String()),
		logger.StringField("log_format", c.Logging.Format),
		logger.BoolField("metrics_enabled", c.Monitoring.MetricsEnabled),
		logger.BoolField("database_configured", c.Database.URL != ""),
		logger.BoolField("redis_configured", c.Redis.URL != ""),
		logger.BoolField("rate_limit_enabled", c.Security.RateLimitEnabled),
		logger.IntField("rate_limit_rps", c.Security.RateLimitRPS),
	)
}