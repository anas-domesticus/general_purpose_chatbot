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
//
// The ACP SDK dispatches notifications (SessionUpdate) in separate goroutines
// while response messages (PromptResponse) are delivered synchronously. This
// means the Prompt() call may return before the last SessionUpdate goroutine
// completes. Use WaitForPendingUpdates() after Prompt returns to ensure all
// buffered content has been written before reading GetResponse().
type ChatbotACPClient struct {
	mu             sync.Mutex
	responseBuffer strings.Builder
	log            *zap.SugaredLogger
	permissionFunc PermissionFunc // nil → auto-approve
	pendingUpdates sync.WaitGroup
}

// NewChatbotACPClient creates a new client that auto-approves all permissions.
func NewChatbotACPClient(log *zap.SugaredLogger) *ChatbotACPClient {
	return &ChatbotACPClient{
		log: log,
	}
}

// WaitForPendingUpdates blocks until all in-flight SessionUpdate notification
// handlers have completed. Call this after Prompt() returns to ensure the
// response buffer contains all agent output.
func (c *ChatbotACPClient) WaitForPendingUpdates() {
	c.pendingUpdates.Wait()
}

// WithPermissionFunc sets an interactive permission handler. When set, the
// handler is called for every permission request instead of auto-approving.
// This is the hook for future interactive flows (e.g. Slack approve/deny buttons).
func (c *ChatbotACPClient) WithPermissionFunc(fn PermissionFunc) {
	c.permissionFunc = fn
}

// SessionUpdate handles agent session notifications, buffering text responses.
func (c *ChatbotACPClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.pendingUpdates.Add(1)
	defer c.pendingUpdates.Done()

	u := n.Update

	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			chunk := u.AgentMessageChunk.Content.Text.Text
			c.log.Debugw("acp: agent message chunk",
				"session", string(n.SessionId), "chunk_len", len(chunk))
			c.mu.Lock()
			c.responseBuffer.WriteString(chunk)
			c.mu.Unlock()
		} else {
			c.log.Debugw("acp: agent message chunk with no text content",
				"session", string(n.SessionId),
				"has_image", u.AgentMessageChunk.Content.Image != nil,
				"has_audio", u.AgentMessageChunk.Content.Audio != nil,
				"has_resource", u.AgentMessageChunk.Content.Resource != nil,
				"has_resource_link", u.AgentMessageChunk.Content.ResourceLink != nil,
			)
		}
	case u.AgentThoughtChunk != nil:
		c.log.Debugw("acp: agent thought chunk", "session", string(n.SessionId))
	case u.ToolCall != nil:
		c.log.Debugw("acp: tool call",
			"session", string(n.SessionId),
			"title", u.ToolCall.Title,
			"status", string(u.ToolCall.Status),
		)
	case u.ToolCallUpdate != nil:
		c.log.Debugw("acp: tool call update",
			"session", string(n.SessionId),
			"tool_call_id", string(u.ToolCallUpdate.ToolCallId),
			"status", u.ToolCallUpdate.Status,
		)
	case u.Plan != nil:
		c.log.Debugw("acp: plan update", "session", string(n.SessionId))
	case u.UserMessageChunk != nil:
		c.log.Debugw("acp: user message chunk", "session", string(n.SessionId))
	case u.CurrentModeUpdate != nil:
		c.log.Debugw("acp: mode update", "session", string(n.SessionId))
	case u.AvailableCommandsUpdate != nil:
		c.log.Debugw("acp: available commands update", "session", string(n.SessionId))
	default:
		c.log.Warnw("acp: unknown session update type", "session", string(n.SessionId))
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
	c.log.Warnw("acp: no allow option available, canceling permission request",
		"options_count", len(params.Options))
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Cancelled: &acp.RequestPermissionOutcomeCancelled{}, //nolint:misspell // SDK type name
		},
	}, nil
}

// ReadTextFile is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) ReadTextFile(_ context.Context, req acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	c.log.Warnw("acp: agent called unsupported ReadTextFile", "path", req.Path)
	return acp.ReadTextFileResponse{}, errNotSupported
}

// WriteTextFile is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) WriteTextFile(_ context.Context, req acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.log.Warnw("acp: agent called unsupported WriteTextFile", "path", req.Path)
	return acp.WriteTextFileResponse{}, errNotSupported
}

// CreateTerminal is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	c.log.Warn("acp: agent called unsupported CreateTerminal")
	return acp.CreateTerminalResponse{}, errNotSupported
}

// KillTerminalCommand is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) KillTerminalCommand(context.Context, acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	c.log.Warn("acp: agent called unsupported KillTerminalCommand")
	return acp.KillTerminalCommandResponse{}, errNotSupported
}

// TerminalOutput is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	c.log.Warn("acp: agent called unsupported TerminalOutput")
	return acp.TerminalOutputResponse{}, errNotSupported
}

// ReleaseTerminal is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	c.log.Warn("acp: agent called unsupported ReleaseTerminal")
	return acp.ReleaseTerminalResponse{}, errNotSupported
}

// WaitForTerminalExit is not supported — capabilities advertised as false.
func (c *ChatbotACPClient) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	c.log.Warn("acp: agent called unsupported WaitForTerminalExit")
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
