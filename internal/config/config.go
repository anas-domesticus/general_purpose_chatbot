// Package config provides application configuration types and validation.
package config

import (
	"fmt"

	"go.uber.org/zap"
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

// LogConfig logs the current configuration (without sensitive data).
func (c *AppConfig) LogConfig(log *zap.SugaredLogger) {
	log.Infow("Application configuration loaded",
		"service_name", c.ServiceName,
		"version", c.Version,
		"environment", c.Environment,
		"log_level", c.Logging.Level,
		"log_format", c.Logging.Format,
		"acp_agents", len(c.ACP.Agents),
		"slack_enabled", c.Slack.Enabled(),
		"telegram_enabled", c.Telegram.Enabled(),
	)
}
