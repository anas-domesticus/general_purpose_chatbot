package acpclient

import (
	"context"
	"fmt"

	acp "github.com/coder/acp-go-sdk"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"go.uber.org/zap"
)

// Executor sends prompts to ACP agent processes and collects responses.
type Executor struct {
	processManager *ProcessManager
	log            *zap.SugaredLogger
}

// NewExecutor creates a new Executor.
func NewExecutor(log *zap.SugaredLogger) *Executor {
	return &Executor{
		processManager: NewProcessManager(log),
		log:            log,
	}
}

// Execute sends a message to the appropriate ACP agent and returns the response.
func (e *Executor) Execute(ctx context.Context, req Request, agentCfg config.ACPAgentConfig, cwd string) (Response, error) {
	e.log.Debugw("acp executor: handling request",
		"scope", req.ScopeKey, "command", agentCfg.Command, "cwd", cwd)

	proc, err := e.processManager.GetOrCreate(ctx, req.ScopeKey, agentCfg, cwd)
	if err != nil {
		e.log.Errorw("acp executor: failed to get or create process",
			"scope", req.ScopeKey, "command", agentCfg.Command, "error", err)
		return Response{}, fmt.Errorf("acp executor: get or create process: %w", err)
	}

	// Check busy flag.
	proc.mu.Lock()
	if proc.busy {
		proc.mu.Unlock()
		e.log.Warnw("acp executor: agent is busy, rejecting request", "scope", req.ScopeKey)
		return Response{}, fmt.Errorf("acp executor: agent is busy for scope %q", req.ScopeKey)
	}
	proc.busy = true
	proc.mu.Unlock()

	defer func() {
		proc.mu.Lock()
		proc.busy = false
		proc.mu.Unlock()
	}()

	proc.client.ResetBuffer()

	e.log.Debugw("acp executor: sending prompt",
		"scope", req.ScopeKey, "message_len", len(req.Message))

	promptResp, err := proc.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: proc.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(req.Message)},
	})
	if err != nil {
		e.log.Errorw("acp executor: prompt failed",
			"scope", req.ScopeKey, "error", err)
		return Response{}, fmt.Errorf("acp executor: prompt for scope %q: %w", req.ScopeKey, err)
	}

	// The ACP SDK dispatches SessionUpdate notifications in separate goroutines
	// while delivering the PromptResponse synchronously. Wait for any in-flight
	// notification handlers to finish writing to the response buffer.
	proc.client.WaitForPendingUpdates()

	text := FormatResponse(proc.client.GetResponse(), promptResp.StopReason)

	e.log.Infow("acp executor: prompt completed",
		"scope", req.ScopeKey,
		"stop_reason", string(promptResp.StopReason),
		"response_len", len(text),
	)

	return Response{Text: text}, nil
}

// FormatResponse returns the response text, adding a synthetic message if the
// agent stopped for a reason other than end_turn with no text output.
func FormatResponse(text string, stopReason acp.StopReason) string {
	if text == "" && stopReason != acp.StopReasonEndTurn {
		return fmt.Sprintf("[agent stopped: %s]", stopReason)
	}
	return text
}

// Shutdown stops all managed agent processes.
func (e *Executor) Shutdown() {
	e.log.Info("acp executor: shutting down")
	e.processManager.Shutdown()
}
