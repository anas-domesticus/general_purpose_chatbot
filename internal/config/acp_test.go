package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestACPConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ACPConfig
		wantErr string // "" means no error
	}{
		{
			name: "valid config with one agent and default_agent",
			cfg: ACPConfig{
				DefaultAgent: "bot",
				Agents:       map[string]ACPAgentConfig{"bot": {Command: "claude"}},
				Channels:     map[string]ACPChannelConfig{"C1": {Agent: "bot"}},
			},
		},
		{
			name:    "no agents",
			cfg:     ACPConfig{Agents: map[string]ACPAgentConfig{}},
			wantErr: "at least one agent",
		},
		{
			name: "default_agent references missing agent",
			cfg: ACPConfig{
				DefaultAgent: "missing",
				Agents:       map[string]ACPAgentConfig{"bot": {Command: "claude"}},
			},
			wantErr: "does not reference",
		},
		{
			name: "agent with empty command",
			cfg: ACPConfig{
				Agents: map[string]ACPAgentConfig{"bot": {Command: ""}},
			},
			wantErr: "non-empty command",
		},
		{
			name: "channel references unknown agent",
			cfg: ACPConfig{
				Agents:   map[string]ACPAgentConfig{"bot": {Command: "claude"}},
				Channels: map[string]ACPChannelConfig{"C1": {Agent: "unknown"}},
			},
			wantErr: "unknown agent",
		},
		{
			name: "multiple agents and channels all valid",
			cfg: ACPConfig{
				DefaultAgent: "a",
				Agents: map[string]ACPAgentConfig{
					"a": {Command: "cmd-a"},
					"b": {Command: "cmd-b"},
				},
				Channels: map[string]ACPChannelConfig{
					"C1": {Agent: "a"},
					"C2": {Agent: "b"},
				},
			},
		},
		{
			name: "empty default_agent is allowed",
			cfg: ACPConfig{
				DefaultAgent: "",
				Agents:       map[string]ACPAgentConfig{"bot": {Command: "claude"}},
			},
		},
		{
			name: "empty channels map is allowed",
			cfg: ACPConfig{
				Agents:   map[string]ACPAgentConfig{"bot": {Command: "claude"}},
				Channels: map[string]ACPChannelConfig{},
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
