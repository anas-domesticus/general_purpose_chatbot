// Package agents provides AI agent creation and management.
package agents

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

// PromptProvider defines the interface for retrieving prompts.
type PromptProvider interface {
	// GetSystemPrompt retrieves the system prompt.
	GetSystemPrompt(ctx context.Context) (string, error)
}

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
	Name           string         // Agent name (e.g., "slack_assistant", "telegram_assistant")
	Platform       string         // Platform name for description (e.g., "Slack", "Telegram")
	Description    string         // Agent description
	Logger         logger.Logger  // Structured logger instance
	PromptProvider PromptProvider // Provider for system prompts
}

// UserInfoFunc is a function that returns user information
type UserInfoFunc func() string

// AgentFactory is a function that creates an agent with platform-specific guidance and user info.
type AgentFactory func(PlatformSpecificGuidanceProvider, UserInfoFunc) (agent.Agent, error)

// NewChatAgent creates a factory function that returns a new chat agent with model and MCP config.
func NewChatAgent(
	ctx context.Context,
	llmModel model.LLM,
	mcpConfig config.MCPConfig,
	agentConfig AgentConfig,
	tools []tool.Tool,
) (AgentFactory, error) {
	if agentConfig.Logger == nil {
		return nil, fmt.Errorf("logger is required in AgentConfig")
	}

	log := agentConfig.Logger.WithFields(logger.StringField("component", "agent"))

	// Load agent instructions from prompt provider
	var instructions string
	if agentConfig.PromptProvider != nil {
		var err error
		instructions, err = agentConfig.PromptProvider.GetSystemPrompt(ctx)
		if err != nil {
			log.Warn("Failed to load system prompt from provider, using default",
				logger.ErrorField(err))
			instructions = getDefaultInstructions()
		} else {
			log.Info("Loaded system prompt from provider")
		}
	} else {
		log.Warn("No prompt provider configured, using default instructions")
		instructions = getDefaultInstructions()
	}

	// Create MCP toolsets if MCP is enabled
	var toolsets []tool.Toolset
	if mcpConfig.Enabled {
		mcpToolsets := createMCPToolsets(mcpConfig, log)
		log.Info("Successfully created MCP toolsets", logger.IntField("count", len(mcpToolsets)))
		toolsets = append(toolsets, mcpToolsets...)
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

		// Create the LLM agent with tools and MCP toolsets
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
func createMCPToolsets(mcpConfig config.MCPConfig, log logger.Logger) []tool.Toolset {
	// Pre-allocate with estimated capacity
	toolsets := make([]tool.Toolset, 0, len(mcpConfig.Servers))

	for serverName, serverConfig := range mcpConfig.Servers {
		// Skip disabled servers
		if !serverConfig.Enabled {
			log.Debug("Skipping disabled MCP server", logger.StringField("server", serverName))
			continue
		}

		log.Debug("Creating MCP toolset",
			logger.StringField("server", serverName),
			logger.StringField("transport", serverConfig.Transport))

		// Create transport based on transport type
		var transport mcp.Transport

		switch serverConfig.Transport {
		case "stdio":
			transport = createStdioTransport(serverConfig)
		case "sse":
			transport = createSSETransport(serverConfig)
		case "http":
			transport = createHTTPTransport(serverConfig)
		case "websocket":
			transport = createWebSocketTransport(serverConfig)
		default:
			log.Warn("Unsupported transport type",
				logger.StringField("transport", serverConfig.Transport),
				logger.StringField("server", serverName))
			continue
		}

		// Create MCP toolset using mcptoolset.New
		mcpToolset, err := mcptoolset.New(mcptoolset.Config{
			Transport: transport,
		})
		if err != nil {
			log.Warn("Failed to create MCP toolset",
				logger.StringField("server", serverName),
				logger.ErrorField(err))
			continue
		}

		toolsets = append(toolsets, mcpToolset)
		log.Info("Successfully created MCP toolset", logger.StringField("server", serverName))
	}

	return toolsets
}

// createStdioTransport creates stdio transport for MCP servers
func createStdioTransport(serverConfig config.MCPServerConfig) mcp.Transport {
	// Build the command
	args := append([]string{}, serverConfig.Args...)
	cmd := exec.Command(serverConfig.Command, args...) //nolint:gosec,noctx // Command comes from trusted config; CommandTransport manages lifecycle

	// Add environment variables if specified
	if len(serverConfig.Environment) > 0 {
		env := make([]string, 0, len(serverConfig.Environment))
		for key, value := range serverConfig.Environment {
			env = append(env, key+"="+value)
		}
		cmd.Env = append(cmd.Env, env...)
	}

	// Create and return CommandTransport
	return &mcp.CommandTransport{
		Command: cmd,
	}
}

// createSSETransport creates SSE transport for MCP servers (2024-11-05 spec)
func createSSETransport(serverConfig config.MCPServerConfig) mcp.Transport {
	return &mcp.SSEClientTransport{
		Endpoint:   serverConfig.URL,
		HTTPClient: createHTTPClient(serverConfig),
	}
}

// createHTTPTransport creates streamable HTTP transport for MCP servers (2025-03-26 spec)
func createHTTPTransport(serverConfig config.MCPServerConfig) mcp.Transport {
	return &mcp.StreamableClientTransport{
		Endpoint:   serverConfig.URL,
		HTTPClient: createHTTPClient(serverConfig),
	}
}

// createWebSocketTransport creates WebSocket transport for MCP servers
func createWebSocketTransport(serverConfig config.MCPServerConfig) mcp.Transport {
	return &webSocketTransport{
		url:     serverConfig.URL,
		headers: serverConfig.Headers,
		auth:    serverConfig.Auth,
	}
}

// createHTTPClient creates an HTTP client with authentication and custom headers
func createHTTPClient(serverConfig config.MCPServerConfig) *http.Client {
	return &http.Client{
		Transport: &authTransport{
			base:    http.DefaultTransport,
			headers: serverConfig.Headers,
			auth:    serverConfig.Auth,
		},
	}
}

// authTransport adds authentication and custom headers to HTTP requests
type authTransport struct {
	base    http.RoundTripper
	headers map[string]string
	auth    *config.MCPAuthConfig
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	if t.auth != nil {
		switch t.auth.Type {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+t.auth.Token)
		case "basic":
			req.SetBasicAuth(t.auth.User, t.auth.Pass)
		case "api_key":
			req.Header.Set(t.auth.Header, t.auth.APIKey)
		}
	}

	return t.base.RoundTrip(req)
}

// webSocketTransport implements mcp.Transport for WebSocket connections
type webSocketTransport struct {
	url     string
	headers map[string]string
	auth    *config.MCPAuthConfig
}

func (t *webSocketTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	// Build headers with auth
	headers := http.Header{}
	for k, v := range t.headers {
		headers.Set(k, v)
	}
	if t.auth != nil {
		switch t.auth.Type {
		case "bearer":
			headers.Set("Authorization", "Bearer "+t.auth.Token)
		case "basic":
			headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
				[]byte(t.auth.User+":"+t.auth.Pass)))
		case "api_key":
			headers.Set(t.auth.Header, t.auth.APIKey)
		}
	}

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.DialContext(ctx, t.url, headers)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	return &wsConnection{conn: conn, done: make(chan struct{})}, nil
}

// wsConnection implements mcp.Connection for WebSocket
type wsConnection struct {
	conn *websocket.Conn
	mu   sync.Mutex
	done chan struct{}
}

func (c *wsConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("connection closed")
	default:
	}

	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return jsonrpc.DecodeMessage(data)
}

func (c *wsConnection) Write(ctx context.Context, msg jsonrpc.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return fmt.Errorf("connection closed")
	default:
	}

	data, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *wsConnection) Close() error {
	close(c.done)
	return c.conn.Close()
}

func (c *wsConnection) SessionID() string {
	return ""
}
