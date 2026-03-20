package acpclient

import (
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
)

// Router resolves which agent config and working directory to use for a given channel.
type Router struct {
	channels     map[string]config.ACPChannelConfig
	agents       map[string]config.ACPAgentConfig
	defaultAgent string
	defaultCwd   string
	autoApprove  bool
}

// NewRouter creates a Router from the given ACP configuration.
func NewRouter(cfg config.ACPConfig) *Router {
	return &Router{
		channels:     cfg.Channels,
		agents:       cfg.Agents,
		defaultAgent: cfg.DefaultAgent,
		defaultCwd:   cfg.Cwd,
		autoApprove:  cfg.AutoApprove,
	}
}

// Resolve returns the agent config and working directory for the given channel.
// If the channel is not explicitly configured, the default agent is used.
// The AutoApprove field on the returned config is resolved from the agent-level
// override (if set) or the global default.
func (r *Router) Resolve(channelID string) (config.ACPAgentConfig, string) {
	agentName := r.defaultAgent
	var channelCwd string

	if ch, ok := r.channels[channelID]; ok {
		agentName = ch.Agent
		channelCwd = ch.Cwd
	}

	agentCfg := r.agents[agentName]

	// Resolve cwd: channel > agent > global.
	cwd := r.defaultCwd
	if agentCfg.Cwd != "" {
		cwd = agentCfg.Cwd
	}
	if channelCwd != "" {
		cwd = channelCwd
	}

	// Resolve auto-approve: agent override > global.
	if agentCfg.AutoApprove == nil {
		approve := r.autoApprove
		agentCfg.AutoApprove = &approve
	}

	return agentCfg, cwd
}

// HasChannel returns true if the channel has an explicit configuration.
func (r *Router) HasChannel(channelID string) bool {
	_, ok := r.channels[channelID]
	return ok
}
