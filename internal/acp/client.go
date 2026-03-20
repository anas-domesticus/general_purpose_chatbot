// Package acpclient provides the ACP (Agent Client Protocol) client
// implementation for bridging messaging platforms to ACP-compatible agents.
package acpclient

import (
	"context"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"go.uber.org/zap"
)

var errNotSupported = &acp.RequestError{Code: -32601, Message: "not supported"}

// PermissionFunc is a callback for interactive permission handling.
// When set, it is called instead of the default auto-approve logic.
// Implementations should present the options to the user and return their selection.
// This enables interactive flows like Slack buttons for approve/deny.
type PermissionFunc func(ctx context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)

// ChatbotACPClient implements the acp.Client interface, collecting agent
// output into a buffer and handling permission requests.
// By default all permissions are auto-approved. Use WithPermissionFunc to
// install an interactive handler (e.g. Slack buttons) instead.
type ChatbotACPClient struct {
	mu             sync.Mutex
	responseBuffer strings.Builder
	log            *zap.SugaredLogger
	permissionFunc PermissionFunc // nil → auto-approve
}

// NewChatbotACPClient creates a new client that auto-approves all permissions.
func NewChatbotACPClient(log *zap.SugaredLogger) *ChatbotACPClient {
	return &ChatbotACPClient{
		log: log,
	}
}

// WithPermissionFunc sets an interactive permission handler. When set, the
// handler is called for every permission request instead of auto-approving.
// This is the hook for future interactive flows (e.g. Slack approve/deny buttons).
func (c *ChatbotACPClient) WithPermissionFunc(fn PermissionFunc) {
	c.permissionFunc = fn
}

// SessionUpdate handles agent session notifications, buffering text responses.
func (c *ChatbotACPClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	u := n.Update

	if u.AgentMessageChunk != nil {
		if u.AgentMessageChunk.Content.Text != nil {
			c.mu.Lock()
			c.responseBuffer.WriteString(u.AgentMessageChunk.Content.Text.Text)
			c.mu.Unlock()
		}
	}

	if u.ToolCall != nil {
		c.log.Debugw("acp: tool call",
			"title", u.ToolCall.Title,
			"status", string(u.ToolCall.Status),
		)
	}

	if u.Plan != nil {
		c.log.Debug("acp: plan update")
	}

	return nil
}

// RequestPermission handles tool permission requests from the agent.
// If a PermissionFunc has been set via WithPermissionFunc, it is called to
// handle the request interactively. Otherwise, the first allow option is
// selected automatically.
func (c *ChatbotACPClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// Delegate to interactive handler if configured.
	if c.permissionFunc != nil {
		return c.permissionFunc(ctx, params)
	}

	// Default: auto-approve by selecting the first allow option.
	for _, opt := range params.Options {
		if opt.Kind == acp.PermissionOptionKindAllowOnce || opt.Kind == acp.PermissionOptionKindAllowAlways {
			c.log.Debugw("acp: auto-approving permission", "option", string(opt.OptionId))
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{
						OptionId: opt.OptionId,
					},
				},
			}, nil
		}
	}

	// No allow option available — cancel.
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Cancelled: &acp.RequestPermissionOutcomeCancelled{}, //nolint:misspell // SDK type name
		},
	}, nil
}

// ReadTextFile is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) ReadTextFile(context.Context, acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, errNotSupported
}

// WriteTextFile is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) WriteTextFile(context.Context, acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, errNotSupported
}

// CreateTerminal is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, errNotSupported
}

// KillTerminalCommand is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) KillTerminalCommand(context.Context, acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, errNotSupported
}

// TerminalOutput is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errNotSupported
}

// ReleaseTerminal is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errNotSupported
}

// WaitForTerminalExit is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errNotSupported
}

// GetResponse returns the accumulated response text.
func (c *ChatbotACPClient) GetResponse() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.responseBuffer.String()
}

// ResetBuffer clears the accumulated response text.
func (c *ChatbotACPClient) ResetBuffer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responseBuffer.Reset()
}
