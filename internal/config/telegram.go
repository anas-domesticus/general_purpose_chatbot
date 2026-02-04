package config

// TelegramConfig holds Telegram-specific configuration
type TelegramConfig struct {
	BotToken string `env:"TELEGRAM_BOT_TOKEN" yaml:"bot_token"`
	Debug    bool   `env:"TELEGRAM_DEBUG" yaml:"debug"`
}

// Enabled returns true if Telegram is configured with a bot token
func (c *TelegramConfig) Enabled() bool {
	return c.BotToken != ""
}
