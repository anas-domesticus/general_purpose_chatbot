// Package config provides application configuration types and validation.
package config

import (
	"fmt"
	"strings"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// AppConfig holds all application configuration.
type AppConfig struct {
	ServiceName string         `env:"SERVICE_NAME" yaml:"service_name" default:"general-purpose-chatbot"`
	Version     string         `env:"VERSION" yaml:"version" default:"dev"`
	Environment string         `env:"ENVIRONMENT" yaml:"environment" default:"development"`
	Logging     LoggingConfig  `yaml:"logging"`
	Health      HealthConfig   `yaml:"health"`
	ACP         ACPConfig      `yaml:"acp"`
	Slack       SlackConfig    `yaml:"slack"`
	Telegram    TelegramConfig `yaml:"telegram"`
}

// Validate validates the configuration and returns an error if invalid.
func (c *AppConfig) Validate() error {
	if err := c.ACP.Validate(); err != nil {
		return fmt.Errorf("acp config: %w", err)
	}
	if !c.Slack.Enabled() && !c.Telegram.Enabled() {
		return fmt.Errorf("at least one connector (slack or telegram) must be enabled")
	}
	return nil
}

// GetLogLevel returns the parsed logger level.
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

// LogConfig logs the current configuration (without sensitive data).
func (c *AppConfig) LogConfig(log logger.Logger) {
	log.Info("Application configuration loaded",
		logger.StringField("service_name", c.ServiceName),
		logger.StringField("version", c.Version),
		logger.StringField("environment", c.Environment),
		logger.StringField("log_level", c.Logging.Level),
		logger.StringField("log_format", c.Logging.Format),
		logger.IntField("acp_agents", len(c.ACP.Agents)),
		logger.BoolField("slack_enabled", c.Slack.Enabled()),
		logger.BoolField("telegram_enabled", c.Telegram.Enabled()),
	)
}
