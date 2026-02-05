// Package agent_info provides agent information tools.
package agent_info //nolint:revive // var-naming: using underscores for domain clarity

import (
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// Args represents the arguments for the agent info tool (no args needed)
type Args struct{}

// Result represents the result of the agent info tool
type Result struct {
	AgentName    string   `json:"agent_name"`
	Model        string   `json:"model"`
	Platform     string   `json:"platform"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Framework    string   `json:"framework"`
}

// Config holds configuration for creating the agent info tool
type Config struct {
	AgentName   string
	Platform    string
	Description string
	Model       model.LLM
}

// createHandler creates a platform-specific agent info handler
func createHandler(config Config) func(tool.Context, Args) (Result, error) {
	return func(ctx tool.Context, args Args) (Result, error) {
		return Result{
			AgentName:   config.AgentName,
			Platform:    config.Platform,
			Model:       config.Model.Name(),
			Description: config.Description,
			Capabilities: []string{
				"General conversation and Q&A",
				"Code analysis and programming help",
				"Technical discussions",
				"Creative writing assistance",
				"Problem solving and reasoning",
				"HTTP requests to external APIs and services",
			},
			Status:    "operational",
			Framework: "Google ADK Go v0.3.0",
		}, nil
	}
}

// New creates a new agent info tool
func New(config Config) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "get_agent_info",
		Description: "Get information about the current agent and its capabilities",
	}, createHandler(config))
}
