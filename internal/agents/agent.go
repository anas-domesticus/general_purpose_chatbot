package agents

import (
	"fmt"
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

// PlatformSpecificGuidanceProvider defines an interface for platform-specific guidance
type PlatformSpecificGuidanceProvider interface {
	PlatformName() string    // Name of the platform (e.g., "Slack", "Telegram")
	FormattingGuide() string // Platform-specific formatting instructions
}

// UserInfoProvider defines an interface for providing user context information
type UserInfoProvider interface {
	UserInfo() string // User context information (e.g., username, display name)
}

// AgentConfig holds configuration for creating a chat agent
type AgentConfig struct {
	Name        string // Agent name (e.g., "slack_assistant", "telegram_assistant")
	Platform    string // Platform name for description (e.g., "Slack", "Telegram")
	Description string // Agent description
}

// UserInfoFunc is a function that returns user information
type UserInfoFunc func() string

// NewChatAgent creates a factory function that returns a new chat agent with Claude model and MCP configuration
func NewChatAgent(llmModel model.LLM, mcpConfig config.MCPConfig, agentConfig AgentConfig) (func(PlatformSpecificGuidanceProvider, UserInfoFunc) (agent.Agent, error), error) {
	// Load agent instructions from system.md in current directory
	instructions := loadInstructionFile("system.md")

	// Create agent info tool with platform-specific handler
	agentInfoTool, err := functiontool.New(functiontool.Config{
		Name:        "get_agent_info",
		Description: "Get information about the current agent and its capabilities",
	}, createAgentInfoHandler(agentConfig, llmModel))
	if err != nil {
		return nil, err
	}

	// Create HTTP request tool
	httpRequestTool, err := functiontool.New(functiontool.Config{
		Name:        "http_request",
		Description: "Make arbitrary HTTP requests to external APIs and services",
	}, handleHTTPRequest)
	if err != nil {
		return nil, err
	}

	// Start with basic tools
	tools := []tool.Tool{
		agentInfoTool,
		httpRequestTool,
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

	// Return a factory function that creates the agent
	return func(guidanceProvider PlatformSpecificGuidanceProvider, userInfoFunc UserInfoFunc) (agent.Agent, error) {
		// Start with base instructions
		agentInstructions := instructions

		// Append platform-specific guidance if provided
		if guidanceProvider != nil {
			platformName := guidanceProvider.PlatformName()
			formattingGuide := guidanceProvider.FormattingGuide()

			if platformName != "" || formattingGuide != "" {
				platformGuidance := "\n\n## Platform Context\n"

				if platformName != "" {
					platformGuidance += fmt.Sprintf("This conversation is happening on %s.\n", platformName)
				}

				if formattingGuide != "" {
					platformGuidance += "\n" + formattingGuide
				}

				agentInstructions += platformGuidance
			}
		}

		// Append user information if provided
		if userInfoFunc != nil {
			userInfo := userInfoFunc()
			if userInfo != "" {
				agentInstructions += fmt.Sprintf("\n\n## User Information\n%s", userInfo)
			}
		}

		// Create the LLM agent with basic tools and MCP toolsets
		chatAgent, err := llmagent.New(llmagent.Config{
			Name:        agentConfig.Name,
			Model:       llmModel,
			Description: agentConfig.Description,
			Instruction: agentInstructions,
			Tools:       tools,
			Toolsets:    toolsets,
		})

		if err != nil {
			return nil, err
		}

		return chatAgent, nil
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
