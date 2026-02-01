package agents

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

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

// AgentConfig holds configuration for creating a chat agent
type AgentConfig struct {
	Name        string // Agent name (e.g., "slack_assistant", "telegram_assistant")
	Platform    string // Platform name for description (e.g., "Slack", "Telegram")
	Description string // Agent description
}

// NewChatAgent creates a factory function that returns a new chat agent with Claude model and MCP configuration
func NewChatAgent(llmModel model.LLM, mcpConfig config.MCPConfig, agentConfig AgentConfig) (func(PlatformSpecificGuidanceProvider) (agent.Agent, error), error) {
	// Load agent instructions from system.md in current directory
	instructions := loadInstructionFile("system.md")

	// Create agent info tool with platform-specific handler
	agentInfoTool, err := functiontool.New(functiontool.Config{
		Name:        "get_agent_info",
		Description: "Get information about the current agent and its capabilities",
	}, createAgentInfoHandler(agentConfig))
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
	return func(guidanceProvider PlatformSpecificGuidanceProvider) (agent.Agent, error) {
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

// HTTPRequestArgs represents the arguments for the HTTP request tool
type HTTPRequestArgs struct {
	Method  string            `json:"method" jsonschema:"required" jsonschema_description:"HTTP method (GET, POST, PUT, DELETE, etc.)"`
	URL     string            `json:"url" jsonschema:"required" jsonschema_description:"Target URL for the request"`
	Headers map[string]string `json:"headers,omitempty" jsonschema_description:"Optional HTTP headers to include in the request"`
	Body    string            `json:"body,omitempty" jsonschema_description:"Optional request body for POST, PUT, etc."`
}

// HTTPRequestResult represents the result of the HTTP request tool
type HTTPRequestResult struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// handleHTTPRequest is the HTTP request tool handler
func handleHTTPRequest(ctx tool.Context, args HTTPRequestArgs) (HTTPRequestResult, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request body if provided
	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = bytes.NewBufferString(args.Body)
	}

	// Create HTTP request
	req, err := http.NewRequest(args.Method, args.URL, bodyReader)
	if err != nil {
		return HTTPRequestResult{
			Error: "Failed to create request: " + err.Error(),
		}, nil
	}

	// Add headers if provided
	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return HTTPRequestResult{
			Error: "Request failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return HTTPRequestResult{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Error:      "Failed to read response body: " + err.Error(),
		}, nil
	}

	// Convert response headers to map
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return HTTPRequestResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    headers,
		Body:       string(respBody),
	}, nil
}

// AgentInfoArgs represents the arguments for the agent info tool (no args needed)
type AgentInfoArgs struct{}

// AgentInfoResult represents the result of the agent info tool
type AgentInfoResult struct {
	AgentName    string   `json:"agent_name"`
	Model        string   `json:"model"`
	Platform     string   `json:"platform"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Framework    string   `json:"framework"`
}

// createAgentInfoHandler creates a platform-specific agent info handler
func createAgentInfoHandler(agentConfig AgentConfig) func(tool.Context, AgentInfoArgs) (AgentInfoResult, error) {
	return func(ctx tool.Context, args AgentInfoArgs) (AgentInfoResult, error) {
		return AgentInfoResult{
			AgentName:   agentConfig.Name,
			Platform:    agentConfig.Platform,
			Model:       "claude-sonnet-4-5-20250929",
			Description: agentConfig.Description,
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
