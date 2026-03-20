package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validACPConfig() ACPConfig {
	return ACPConfig{
		Agents: map[string]ACPAgentConfig{"bot": {Command: "claude"}},
	}
}

func TestAppConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     AppConfig
		wantErr string
	}{
		{
			name: "valid config with slack enabled",
			cfg: AppConfig{
				ACP:   validACPConfig(),
				Slack: SlackConfig{BotToken: "xoxb-1", AppToken: "xapp-1"},
			},
		},
		{
			name: "ACP invalid propagates error",
			cfg: AppConfig{
				ACP:   ACPConfig{Agents: map[string]ACPAgentConfig{}},
				Slack: SlackConfig{BotToken: "xoxb-1", AppToken: "xapp-1"},
			},
			wantErr: "at least one agent",
		},
		{
			name: "neither slack nor telegram enabled",
			cfg: AppConfig{
				ACP: validACPConfig(),
			},
			wantErr: "at least one connector",
		},
		{
			name: "slack enabled only",
			cfg: AppConfig{
				ACP:   validACPConfig(),
				Slack: SlackConfig{BotToken: "xoxb-1", AppToken: "xapp-1"},
			},
		},
		{
			name: "telegram enabled only",
			cfg: AppConfig{
				ACP:      validACPConfig(),
				Telegram: TelegramConfig{BotToken: "123:ABC"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
