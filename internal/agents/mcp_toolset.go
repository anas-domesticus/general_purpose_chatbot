// Package agents provides AI agent creation and management.
package agents

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// mcpToolset is a custom MCP toolset that properly handles all MCP content types
// in tool call results, including EmbeddedResource, ImageContent, AudioContent,
// and ResourceLink.
//
// This replaces google.golang.org/adk/tool/mcptoolset which only processes
// TextContent and silently drops all other content types. This causes the GitHub
// MCP server's get_file_contents tool to appear broken, since it returns file
// content as EmbeddedResource (type: "resource").
//
// See: https://github.com/github/github-mcp-server/issues/782
type mcpToolset struct {
	transport mcp.Transport
	client    *mcp.Client
	log       logger.Logger

	mu      sync.Mutex
	session *mcp.ClientSession
}

// newMCPToolset creates a new MCP toolset with the given transport.
func newMCPToolset(transport mcp.Transport, log logger.Logger) *mcpToolset {
	return &mcpToolset{
		transport: transport,
		client:    mcp.NewClient(&mcp.Implementation{Name: "provo-mcp-client", Version: "1.0.0"}, nil),
		log:       log,
	}
}

func (s *mcpToolset) Name() string {
	return "mcp_tool_set"
}

func (s *mcpToolset) getSession(ctx context.Context) (*mcp.ClientSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		return s.session, nil
	}

	session, err := s.client.Connect(ctx, s.transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	s.session = session
	return s.session, nil
}

// mcpRefreshableErrors is a list of errors that should trigger a reconnection.
var mcpRefreshableErrors = []error{
	mcp.ErrConnectionClosed,
	io.ErrClosedPipe,
	io.EOF,
}

func isMCPRefreshableError(err error) bool {
	for _, target := range mcpRefreshableErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return strings.Contains(err.Error(), "session not found")
}

func (s *mcpToolset) refreshSession(ctx context.Context) (*mcp.ClientSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		if err := s.session.Ping(ctx, &mcp.PingParams{}); err == nil {
			return s.session, nil
		}
		if err := s.session.Close(); err != nil {
			s.log.Debug("Failed to close MCP session during refresh", logger.ErrorField(err))
		}
		s.session = nil
	}

	session, err := s.client.Connect(ctx, s.transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh MCP session: %w", err)
	}

	s.session = session
	return s.session, nil
}

func (s *mcpToolset) callTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	session, err := s.getSession(ctx)
	if err != nil {
		return nil, err
	}

	result, err := session.CallTool(ctx, params)
	if err != nil {
		if !isMCPRefreshableError(err) {
			return nil, err
		}
		session, refreshErr := s.refreshSession(ctx)
		if refreshErr != nil {
			return nil, fmt.Errorf("%w (reconnection also failed: %v)", err, refreshErr)
		}
		return session.CallTool(ctx, params)
	}
	return result, nil
}

func (s *mcpToolset) listTools(ctx context.Context) ([]*mcp.Tool, error) {
	session, err := s.getSession(ctx)
	if err != nil {
		return nil, err
	}

	var tools []*mcp.Tool
	cursor := ""
	hasReconnected := false

	for {
		resp, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			if !isMCPRefreshableError(err) {
				return nil, fmt.Errorf("failed to list MCP tools: %w", err)
			}
			if hasReconnected {
				return nil, fmt.Errorf("failed to list MCP tools: connection lost after reconnection")
			}
			newSession, refreshErr := s.refreshSession(ctx)
			if refreshErr != nil {
				return nil, fmt.Errorf("%w (reconnection also failed: %v)", err, refreshErr)
			}
			session = newSession
			hasReconnected = true
			tools = nil
			cursor = ""
			continue
		}

		tools = append(tools, resp.Tools...)
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	return tools, nil
}

// Tools returns the list of tools from the MCP server, converted to ADK tools.
func (s *mcpToolset) Tools(ctx agent.ReadonlyContext) ([]tool.Tool, error) {
	mcpTools, err := s.listTools(ctx)
	if err != nil {
		return nil, err
	}

	adkTools := make([]tool.Tool, 0, len(mcpTools))
	for _, mt := range mcpTools {
		t := &mcpToolImpl{
			name:        mt.Name,
			description: mt.Description,
			funcDeclaration: &genai.FunctionDeclaration{
				Name:        mt.Name,
				Description: mt.Description,
			},
			toolset: s,
		}
		// Avoid typed-nil interface problem that crashes genai converter.
		if mt.InputSchema != nil {
			t.funcDeclaration.ParametersJsonSchema = mt.InputSchema
		}
		if mt.OutputSchema != nil {
			t.funcDeclaration.ResponseJsonSchema = mt.OutputSchema
		}
		adkTools = append(adkTools, t)
	}

	return adkTools, nil
}

// mcpToolImpl implements tool.Tool with proper handling of all MCP content types.
// It also satisfies the internal ADK interfaces (FunctionTool, RequestProcessor)
// via structural typing, which the prefixedTool wrapper discovers through
// type assertions.
type mcpToolImpl struct {
	name            string
	description     string
	funcDeclaration *genai.FunctionDeclaration
	toolset         *mcpToolset
}

func (t *mcpToolImpl) Name() string        { return t.name }
func (t *mcpToolImpl) Description() string  { return t.description }
func (t *mcpToolImpl) IsLongRunning() bool  { return false }

func (t *mcpToolImpl) Declaration() *genai.FunctionDeclaration {
	return t.funcDeclaration
}

// Run executes the MCP tool and processes all content types in the response.
// Unlike the ADK's mcptoolset which only handles TextContent, this processes:
//   - TextContent: plain text
//   - EmbeddedResource: inline file content (text or binary)
//   - ImageContent: base64 image data
//   - AudioContent: base64 audio data
//   - ResourceLink: URI references to resources
func (t *mcpToolImpl) Run(ctx tool.Context, args any) (map[string]any, error) {
	res, err := t.toolset.callTool(ctx, &mcp.CallToolParams{
		Name:      t.name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP tool %q: %w", t.name, err)
	}

	if res.IsError {
		details := extractTextFromContent(res.Content)
		errMsg := "Tool execution failed."
		if details != "" {
			errMsg += " Details: " + details
		}
		return nil, errors.New(errMsg)
	}

	if res.StructuredContent != nil {
		return map[string]any{
			"output": res.StructuredContent,
		}, nil
	}

	output := extractAllContent(res.Content)
	if output == "" {
		return nil, errors.New("no content in tool response")
	}

	return map[string]any{
		"output": output,
	}, nil
}

// extractTextFromContent extracts text from content blocks for error messages.
func extractTextFromContent(content []mcp.Content) string {
	var b strings.Builder
	for _, c := range content {
		switch v := c.(type) {
		case *mcp.TextContent:
			b.WriteString(v.Text)
		case *mcp.EmbeddedResource:
			if v.Resource != nil && v.Resource.Text != "" {
				b.WriteString(v.Resource.Text)
			}
		}
	}
	return b.String()
}

// extractAllContent processes all MCP content types and returns combined text output.
func extractAllContent(content []mcp.Content) string {
	var b strings.Builder
	for _, c := range content {
		switch v := c.(type) {
		case *mcp.TextContent:
			b.WriteString(v.Text)

		case *mcp.EmbeddedResource:
			if v.Resource == nil {
				continue
			}
			if v.Resource.Text != "" {
				b.WriteString(v.Resource.Text)
			} else if v.Resource.Blob != nil {
				if isTextMIMEType(v.Resource.MIMEType) {
					b.WriteString(string(v.Resource.Blob))
				} else {
					fmt.Fprintf(&b, "[Binary resource: %s, URI: %s, %d bytes, base64: %s]",
						v.Resource.MIMEType, v.Resource.URI, len(v.Resource.Blob),
						base64.StdEncoding.EncodeToString(v.Resource.Blob))
				}
			}

		case *mcp.ImageContent:
			fmt.Fprintf(&b, "[Image: %s, %d bytes]", v.MIMEType, len(v.Data))

		case *mcp.AudioContent:
			fmt.Fprintf(&b, "[Audio: %s, %d bytes]", v.MIMEType, len(v.Data))

		case *mcp.ResourceLink:
			fmt.Fprintf(&b, "[Resource link: %s (%s)]", v.URI, v.Name)
		}
	}
	return b.String()
}

// isTextMIMEType checks if a MIME type represents text-like content that can
// be safely converted from bytes to a string.
func isTextMIMEType(mimeType string) bool {
	base := strings.SplitN(mimeType, ";", 2)[0]
	base = strings.TrimSpace(base)
	if strings.HasPrefix(base, "text/") {
		return true
	}
	switch base {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-yaml",
		"application/yaml",
		"application/xhtml+xml",
		"application/toml":
		return true
	}
	return false
}
