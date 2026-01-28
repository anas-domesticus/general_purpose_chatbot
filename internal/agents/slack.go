package agents

import (
	"log"
	"os/exec"

	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/mcptoolset"
)

// NewSlackAgent creates a new Slack workspace agent with Claude model and MCP configuration
func NewSlackAgent(llmModel model.LLM, mcpConfig config.MCPConfig) (agent.Agent, error) {
	// Load agent instructions from system.md in current directory
	instructions := loadInstructionFile("system.md")

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

	// Start with basic tools
	tools := []tool.Tool{
		echoTool,
		agentInfoTool,
	}

	// Create MCP toolsets if MCP is enabled
	var toolsets []tool.Toolset
	if mcpConfig.Enabled {
		mcpToolsets, err := createMCPToolsets(mcpConfig)
		if err != nil {
			log.Printf("Warning: Failed to create MCP toolsets: %v", err)
			// Continue with basic tools if MCP setup fails
		} else {
			log.Printf("Successfully created %d MCP toolsets", len(mcpToolsets))
			toolsets = append(toolsets, mcpToolsets...)
		}
	}

	// Create the LLM agent with basic tools and MCP toolsets
	slackAgent, err := llmagent.New(llmagent.Config{
		Name:        "slack_assistant",
		Model:       llmModel,
		Description: "Claude-powered assistant for Slack workspace interactions with MCP capabilities",
		Instruction: instructions,
		Tools:       tools,
		Toolsets:    toolsets,
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

// createMCPToolsets creates MCP toolsets based on configuration
func createMCPToolsets(mcpConfig config.MCPConfig) ([]tool.Toolset, error) {
	var toolsets []tool.Toolset

	for serverName, serverConfig := range mcpConfig.Servers {
		// Skip disabled servers
		if !serverConfig.Enabled {
			log.Printf("Skipping disabled MCP server: %s", serverName)
			continue
		}

		log.Printf("Creating MCP toolset for server: %s (transport: %s)", serverName, serverConfig.Transport)

		// Create transport based on transport type
		var transport mcp.Transport
		var err error

		switch serverConfig.Transport {
		case "stdio":
			transport, err = createStdioTransport(serverConfig)
		case "websocket":
			log.Printf("Warning: WebSocket transport not yet implemented for MCP server '%s'", serverName)
			continue
		case "sse":
			log.Printf("Warning: SSE transport not yet implemented for MCP server '%s'", serverName)
			continue
		default:
			log.Printf("Warning: Unsupported transport type '%s' for MCP server '%s'", serverConfig.Transport, serverName)
			continue
		}

		if err != nil {
			log.Printf("Warning: Failed to create transport for MCP server '%s': %v", serverName, err)
			continue
		}

		// Create MCP toolset using mcptoolset.New
		mcpToolset, err := mcptoolset.New(mcptoolset.Config{
			Transport: transport,
		})
		if err != nil {
			log.Printf("Warning: Failed to create MCP toolset for server '%s': %v", serverName, err)
			continue
		}

		toolsets = append(toolsets, mcpToolset)
		log.Printf("Successfully created MCP toolset for server: %s", serverName)
	}

	return toolsets, nil
}

// createStdioTransport creates stdio transport for MCP servers
func createStdioTransport(serverConfig config.MCPServerConfig) (mcp.Transport, error) {
	// Build the command
	args := append([]string{}, serverConfig.Args...)
	cmd := exec.Command(serverConfig.Command, args...)

	// Add environment variables if specified
	if len(serverConfig.Environment) > 0 {
		env := make([]string, 0, len(serverConfig.Environment))
		for key, value := range serverConfig.Environment {
			env = append(env, key+"="+value)
		}
		cmd.Env = append(cmd.Env, env...)
	}

	// Create and return CommandTransport
	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	return transport, nil
}