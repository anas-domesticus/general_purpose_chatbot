// Package acpclient provides the ACP (Agent Client Protocol) client
// implementation for bridging messaging platforms to ACP-compatible agents.
package acpclient

import (
	"context"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

var errNotSupported = &acp.RequestError{Code: -32601, Message: "not supported"}

// ChatbotACPClient implements the acp.Client interface, collecting agent
// output into a buffer and handling permission requests.
type ChatbotACPClient struct {
	autoApprove    bool
	mu             sync.Mutex
	responseBuffer strings.Builder
	log            logger.Logger
}

// NewChatbotACPClient creates a new client with the given settings.
func NewChatbotACPClient(autoApprove bool, log logger.Logger) *ChatbotACPClient {
	return &ChatbotACPClient{
		autoApprove: autoApprove,
		log:         log,
	}
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
		c.log.Debug("acp: tool call",
			logger.StringField("title", u.ToolCall.Title),
			logger.StringField("status", string(u.ToolCall.Status)),
		)
	}

	if u.Plan != nil {
		c.log.Debug("acp: plan update")
	}

	return nil
}

// RequestPermission handles tool permission requests from the agent.
func (c *ChatbotACPClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if c.autoApprove {
		for _, opt := range params.Options {
			if opt.Kind == acp.PermissionOptionKindAllowOnce || opt.Kind == acp.PermissionOptionKindAllowAlways {
				c.log.Debug("acp: auto-approving permission", logger.StringField("option", string(opt.OptionId)))
				return acp.RequestPermissionResponse{
					Outcome: acp.RequestPermissionOutcome{
						Selected: &acp.RequestPermissionOutcomeSelected{
							OptionId: opt.OptionId,
						},
					},
				}, nil
			}
		}
	}

	// Reject by default.
	for _, opt := range params.Options {
		if opt.Kind == acp.PermissionOptionKindRejectOnce {
			c.log.Debug("acp: rejecting permission", logger.StringField("option", string(opt.OptionId)))
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{
						OptionId: opt.OptionId,
					},
				},
			}, nil
		}
	}

	// No suitable option found — cancel.
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
