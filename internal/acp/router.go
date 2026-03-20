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
}

// NewRouter creates a Router from the given ACP configuration.
func NewRouter(cfg config.ACPConfig) *Router {
	return &Router{
		channels:     cfg.Channels,
		agents:       cfg.Agents,
		defaultAgent: cfg.DefaultAgent,
		defaultCwd:   cfg.Cwd,
	}
}

// Resolve returns the agent config and working directory for the given channel.
// If the channel is not explicitly configured, the default agent is used.
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

	return agentCfg, cwd
}

// HasChannel returns true if the channel has an explicit configuration.
func (r *Router) HasChannel(channelID string) bool {
	_, ok := r.channels[channelID]
	return ok
}
