package acpclient

import (
	"testing"

	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRouter_Resolve(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.ACPConfig
		channelID string
		wantCmd   string
		wantCwd   string
	}{
		{
			name: "channel explicitly mapped returns correct agent and channel cwd",
			cfg: config.ACPConfig{
				DefaultAgent: "default",
				Cwd:          "/global",
				Agents: map[string]config.ACPAgentConfig{
					"default": {Command: "default-cmd"},
					"special": {Command: "special-cmd"},
				},
				Channels: map[string]config.ACPChannelConfig{
					"C1": {Agent: "special", Cwd: "/channel-cwd"},
				},
			},
			channelID: "C1",
			wantCmd:   "special-cmd",
			wantCwd:   "/channel-cwd",
		},
		{
			name: "unmapped channel falls back to default agent and global cwd",
			cfg: config.ACPConfig{
				DefaultAgent: "default",
				Cwd:          "/global",
				Agents: map[string]config.ACPAgentConfig{
					"default": {Command: "default-cmd"},
				},
			},
			channelID: "C-unknown",
			wantCmd:   "default-cmd",
			wantCwd:   "/global",
		},
		{
			name: "cwd cascade: channel overrides agent overrides global",
			cfg: config.ACPConfig{
				DefaultAgent: "a",
				Cwd:          "/global",
				Agents: map[string]config.ACPAgentConfig{
					"a": {Command: "cmd", Cwd: "/agent"},
				},
				Channels: map[string]config.ACPChannelConfig{
					"C1": {Agent: "a", Cwd: "/channel"},
				},
			},
			channelID: "C1",
			wantCmd:   "cmd",
			wantCwd:   "/channel",
		},
		{
			name: "cwd cascade: no channel cwd uses agent cwd",
			cfg: config.ACPConfig{
				DefaultAgent: "a",
				Cwd:          "/global",
				Agents: map[string]config.ACPAgentConfig{
					"a": {Command: "cmd", Cwd: "/agent"},
				},
				Channels: map[string]config.ACPChannelConfig{
					"C1": {Agent: "a"},
				},
			},
			channelID: "C1",
			wantCmd:   "cmd",
			wantCwd:   "/agent",
		},
		{
			name: "cwd cascade: no channel cwd no agent cwd uses global",
			cfg: config.ACPConfig{
				DefaultAgent: "a",
				Cwd:          "/global",
				Agents: map[string]config.ACPAgentConfig{
					"a": {Command: "cmd"},
				},
				Channels: map[string]config.ACPChannelConfig{
					"C1": {Agent: "a"},
				},
			},
			channelID: "C1",
			wantCmd:   "cmd",
			wantCwd:   "/global",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRouter(tt.cfg)
			agentCfg, cwd := r.Resolve(tt.channelID)
			assert.Equal(t, tt.wantCmd, agentCfg.Command)
			assert.Equal(t, tt.wantCwd, cwd)
		})
	}
}

func TestRouter_HasChannel(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		want      bool
	}{
		{"mapped channel", "C1", true},
		{"unmapped channel", "C-other", false},
	}

	cfg := config.ACPConfig{
		Agents:   map[string]config.ACPAgentConfig{"a": {Command: "cmd"}},
		Channels: map[string]config.ACPChannelConfig{"C1": {Agent: "a"}},
	}
	r := NewRouter(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.HasChannel(tt.channelID))
		})
	}
}
