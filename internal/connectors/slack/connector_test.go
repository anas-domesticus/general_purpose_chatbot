package slack

import (
	"testing"

	acpclient "github.com/lewisedginton/general_purpose_chatbot/internal/acp"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRemoveBotMention(t *testing.T) {
	c := &Connector{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "mention at start",
			input: "<@U123> hello",
			want:  "hello",
		},
		{
			name:  "mention in middle",
			input: "hello <@U123> world",
			want:  "hello  world", // double space preserved — TrimSpace only trims edges
		},
		{
			name:  "no mention",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "mention only",
			input: "<@U123>",
			want:  "",
		},
		{
			name:  "two mentions — only first removed",
			input: "<@U123> foo <@U456> bar",
			want:  "foo <@U456> bar",
		},
		{
			name:  "unclosed mention",
			input: "<@",
			want:  "<@",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.removeBotMention(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewConnector_Validation(t *testing.T) {
	log := zap.NewNop().Sugar()
	exec := acpclient.NewExecutor(log)
	router := acpclient.NewRouter(config.ACPConfig{
		DefaultAgent: "test",
		Agents:       map[string]config.ACPAgentConfig{"test": {Command: "echo"}},
	})

	tests := []struct {
		name    string
		config  Config
		exec    *acpclient.Executor
		router  *acpclient.Router
		log     *zap.SugaredLogger
		wantErr string
	}{
		{
			name:   "valid config",
			config: Config{BotToken: "xoxb-test", AppToken: "xapp-test"},
			exec:   exec,
			router: router,
			log:    log,
		},
		{
			name:    "invalid bot token",
			config:  Config{BotToken: "bad-token", AppToken: "xapp-test"},
			exec:    exec,
			router:  router,
			log:     log,
			wantErr: "bot token",
		},
		{
			name:    "invalid app token",
			config:  Config{BotToken: "xoxb-test", AppToken: "bad-token"},
			exec:    exec,
			router:  router,
			log:     log,
			wantErr: "app token",
		},
		{
			name:    "nil executor",
			config:  Config{BotToken: "xoxb-test", AppToken: "xapp-test"},
			exec:    nil,
			router:  router,
			log:     log,
			wantErr: "executor",
		},
		{
			name:    "nil router",
			config:  Config{BotToken: "xoxb-test", AppToken: "xapp-test"},
			exec:    exec,
			router:  nil,
			log:     log,
			wantErr: "router",
		},
		{
			name:    "nil logger",
			config:  Config{BotToken: "xoxb-test", AppToken: "xapp-test"},
			exec:    exec,
			router:  router,
			log:     nil,
			wantErr: "logger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewConnector(tt.config, tt.exec, tt.router, tt.log)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, conn)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, conn)
			}
		})
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		wantErr   bool
	}{
		{
			name:      "not connected — returns error",
			connected: false,
			wantErr:   true,
		},
		{
			name:      "connected — returns nil",
			connected: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Connector{}
			c.setConnected(tt.connected)

			err := c.Ready()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
