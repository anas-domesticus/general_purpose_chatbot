package agents

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// NewSlackAgent creates a new Slack workspace agent with Claude model
func NewSlackAgent(llmModel model.LLM) (agent.Agent, error) {
	// Load agent instructions from config file
	instructions := loadInstructionFile("config/agents/slack.txt")

	// Create echo tool
	echoTool, err := functiontool.New(functiontool.Config{
		Name:        "echo",
		Description: "Echo back text for testing the agent functionality",
	}, handleEcho)
	if err != nil {
		return nil, err
	}

	// Create agent info tool
	agentInfoTool, err := functiontool.New(functiontool.Config{
		Name:        "get_agent_info",
		Description: "Get information about the current agent and its capabilities",
	}, handleGetAgentInfo)
	if err != nil {
		return nil, err
	}

	// Create the LLM agent with basic tools
	slackAgent, err := llmagent.New(llmagent.Config{
		Name:        "slack_assistant",
		Model:       llmModel,
		Description: "Claude-powered assistant for Slack workspace interactions",
		Instruction: instructions,
		Tools: []tool.Tool{
			echoTool,
			agentInfoTool,
		},
	})

	if err != nil {
		return nil, err
	}

	return slackAgent, nil
}

// EchoArgs represents the arguments for the echo tool
type EchoArgs struct {
	Text string `json:"text" jsonschema:"required" jsonschema_description:"Text to echo back"`
}

// EchoResult represents the result of the echo tool
type EchoResult struct {
	EchoedText string `json:"echoed_text"`
	Message    string `json:"message"`
}

// handleEcho is the echo tool handler
func handleEcho(ctx tool.Context, args EchoArgs) (EchoResult, error) {
	return EchoResult{
		EchoedText: args.Text,
		Message:    "Echo: " + args.Text,
	}, nil
}

// AgentInfoArgs represents the arguments for the agent info tool (no args needed)
type AgentInfoArgs struct{}

// AgentInfoResult represents the result of the agent info tool
type AgentInfoResult struct {
	AgentName    string   `json:"agent_name"`
	Model        string   `json:"model"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Framework    string   `json:"framework"`
}

// handleGetAgentInfo is the agent info tool handler
func handleGetAgentInfo(ctx tool.Context, args AgentInfoArgs) (AgentInfoResult, error) {
	return AgentInfoResult{
		AgentName:   "slack_assistant",
		Model:       "claude-3-5-sonnet-20241022",
		Description: "Claude-powered assistant for Slack workspace interactions",
		Capabilities: []string{
			"General conversation and Q&A",
			"Code analysis and programming help",
			"Technical discussions",
			"Creative writing assistance",
			"Problem solving and reasoning",
			"Echo functionality for testing",
		},
		Status:    "operational",
		Framework: "Google ADK Go v0.3.0",
	}, nil
}