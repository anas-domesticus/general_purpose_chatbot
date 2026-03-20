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
	proc, err := e.processManager.GetOrCreate(ctx, req.ScopeKey, agentCfg, cwd)
	if err != nil {
		return Response{}, fmt.Errorf("acp executor: get or create process: %w", err)
	}

	// Check if process is dead and recreate.
	select {
	case <-proc.done:
		e.log.Warnw("acp executor: process died, recreating", "scope", req.ScopeKey)
		_ = e.processManager.Remove(req.ScopeKey)
		proc, err = e.processManager.GetOrCreate(ctx, req.ScopeKey, agentCfg, cwd)
		if err != nil {
			return Response{}, fmt.Errorf("acp executor: recreate process: %w", err)
		}
	default:
	}

	// Check busy flag.
	proc.mu.Lock()
	if proc.busy {
		proc.mu.Unlock()
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

	promptResp, err := proc.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: proc.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(req.Message)},
	})
	if err != nil {
		return Response{}, fmt.Errorf("acp executor: prompt: %w", err)
	}

	e.log.Debugw("acp executor: prompt completed",
		"scope", req.ScopeKey,
		"stop_reason", string(promptResp.StopReason),
	)

	text := proc.client.GetResponse()
	if text == "" && promptResp.StopReason != acp.StopReasonEndTurn {
		text = fmt.Sprintf("[agent stopped: %s]", promptResp.StopReason)
	}

	return Response{Text: text}, nil
}

// Shutdown stops all managed agent processes.
func (e *Executor) Shutdown() {
	e.processManager.Shutdown()
}
