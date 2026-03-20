package acpclient

import (
	"context"
	"errors"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestClient() *ChatbotACPClient {
	return NewChatbotACPClient(zap.NewNop().Sugar())
}

func textNotification(text string) acp.SessionNotification {
	return acp.SessionNotification{
		Update: acp.UpdateAgentMessageText(text),
	}
}

func TestSessionUpdate(t *testing.T) {
	tests := []struct {
		name       string
		updates    []acp.SessionNotification
		wantBuffer string
	}{
		{
			name:       "agent message chunk with text appended to buffer",
			updates:    []acp.SessionNotification{textNotification("hello")},
			wantBuffer: "hello",
		},
		{
			name: "agent message chunk with nil text content is no-op",
			updates: []acp.SessionNotification{
				{Update: acp.SessionUpdate{AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
					Content: acp.ContentBlock{}, // no Text field set
				}}},
			},
			wantBuffer: "",
		},
		{
			name: "tool call notification does not change buffer",
			updates: []acp.SessionNotification{
				{Update: acp.SessionUpdate{ToolCall: &acp.SessionUpdateToolCall{
					Title: "read file",
				}}},
			},
			wantBuffer: "",
		},
		{
			name: "plan notification does not change buffer",
			updates: []acp.SessionNotification{
				{Update: acp.SessionUpdate{Plan: &acp.SessionUpdatePlan{
					Entries: []acp.PlanEntry{},
				}}},
			},
			wantBuffer: "",
		},
		{
			name: "multiple chunks accumulate",
			updates: []acp.SessionNotification{
				textNotification("hello "),
				textNotification("world"),
			},
			wantBuffer: "hello world",
		},
		{
			name:       "empty update is no-op",
			updates:    []acp.SessionNotification{{Update: acp.SessionUpdate{}}},
			wantBuffer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient()
			for _, n := range tt.updates {
				err := c.SessionUpdate(context.Background(), n)
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantBuffer, c.GetResponse())
		})
	}
}

func TestRequestPermission(t *testing.T) {
	allowOnce := acp.PermissionOption{
		OptionId: "opt-allow-once",
		Name:     "Allow Once",
		Kind:     acp.PermissionOptionKindAllowOnce,
	}
	allowAlways := acp.PermissionOption{
		OptionId: "opt-allow-always",
		Name:     "Allow Always",
		Kind:     acp.PermissionOptionKindAllowAlways,
	}
	rejectOnce := acp.PermissionOption{
		OptionId: "opt-reject",
		Name:     "Reject",
		Kind:     acp.PermissionOptionKindRejectOnce,
	}

	tests := []struct {
		name           string
		permFunc       PermissionFunc
		options        []acp.PermissionOption
		wantSelected   acp.PermissionOptionId
		wantCancelled  bool
		wantCustom     bool
		customOptionID acp.PermissionOptionId
	}{
		{
			name:         "default selects allow_once",
			options:      []acp.PermissionOption{rejectOnce, allowOnce},
			wantSelected: "opt-allow-once",
		},
		{
			name:         "default selects allow_always when no allow_once",
			options:      []acp.PermissionOption{rejectOnce, allowAlways},
			wantSelected: "opt-allow-always",
		},
		{
			name:          "default cancels when no allow options",
			options:       []acp.PermissionOption{rejectOnce},
			wantCancelled: true,
		},
		{
			name: "custom permission func is called",
			permFunc: func(_ context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
				return acp.RequestPermissionResponse{
					Outcome: acp.RequestPermissionOutcome{
						Selected: &acp.RequestPermissionOutcomeSelected{
							OptionId: "custom-id",
						},
					},
				}, nil
			},
			options:        []acp.PermissionOption{allowOnce},
			wantCustom:     true,
			customOptionID: "custom-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient()
			if tt.permFunc != nil {
				c.WithPermissionFunc(tt.permFunc)
			}

			resp, err := c.RequestPermission(context.Background(), acp.RequestPermissionRequest{
				Options: tt.options,
			})
			require.NoError(t, err)

			switch {
			case tt.wantCancelled:
				assert.NotNil(t, resp.Outcome.Cancelled) //nolint:misspell // SDK type name
				assert.Nil(t, resp.Outcome.Selected)
			case tt.wantCustom:
				require.NotNil(t, resp.Outcome.Selected)
				assert.Equal(t, tt.customOptionID, resp.Outcome.Selected.OptionId)
			default:
				require.NotNil(t, resp.Outcome.Selected)
				assert.Equal(t, tt.wantSelected, resp.Outcome.Selected.OptionId)
			}
		})
	}
}

func TestUnsupportedMethods(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()

	unsupported := []struct {
		name string
		fn   func() error
	}{
		{"ReadTextFile", func() error { _, err := c.ReadTextFile(ctx, acp.ReadTextFileRequest{}); return err }},
		{"WriteTextFile", func() error { _, err := c.WriteTextFile(ctx, acp.WriteTextFileRequest{}); return err }},
		{"CreateTerminal", func() error { _, err := c.CreateTerminal(ctx, acp.CreateTerminalRequest{}); return err }},
		{"KillTerminalCommand", func() error {
			_, err := c.KillTerminalCommand(ctx, acp.KillTerminalCommandRequest{})
			return err
		}},
		{"TerminalOutput", func() error { _, err := c.TerminalOutput(ctx, acp.TerminalOutputRequest{}); return err }},
		{"ReleaseTerminal", func() error { _, err := c.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{}); return err }},
		{"WaitForTerminalExit", func() error {
			_, err := c.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{})
			return err
		}},
	}

	for _, tt := range unsupported {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			require.Error(t, err)
			var reqErr *acp.RequestError
			require.True(t, errors.As(err, &reqErr))
			assert.Equal(t, -32601, reqErr.Code)
		})
	}
}

func TestGetResponse_and_ResetBuffer(t *testing.T) {
	c := newTestClient()
	ctx := context.Background()

	err := c.SessionUpdate(ctx, textNotification("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello", c.GetResponse())

	c.ResetBuffer()
	assert.Equal(t, "", c.GetResponse())
}
