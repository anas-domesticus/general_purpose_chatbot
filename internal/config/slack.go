package config

// SlackConfig holds Slack-specific configuration
type SlackConfig struct {
	BotToken string `env:"SLACK_BOT_TOKEN" yaml:"bot_token"`
	AppToken string `env:"SLACK_APP_TOKEN" yaml:"app_token"`
	Debug    bool   `env:"SLACK_DEBUG" yaml:"debug"`
}

// Enabled returns true if Slack is configured with both tokens
func (c *SlackConfig) Enabled() bool {
	return c.BotToken != "" && c.AppToken != ""
}
