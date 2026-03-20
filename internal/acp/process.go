package acpclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// AgentProcess represents a running ACP agent subprocess.
type AgentProcess struct {
	conn      *acp.ClientSideConnection
	client    *ChatbotACPClient
	cmd       *exec.Cmd
	sessionID acp.SessionId
	done      <-chan struct{}
	busy      bool
	mu        sync.Mutex
}

// ProcessManager manages the lifecycle of ACP agent processes keyed by scope.
type ProcessManager struct {
	processes map[string]*AgentProcess
	mu        sync.RWMutex
	log       logger.Logger
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(log logger.Logger) *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*AgentProcess),
		log:       log,
	}
}

// GetOrCreate returns an existing live process or spawns a new one.
func (pm *ProcessManager) GetOrCreate(ctx context.Context, scopeKey string, agentCfg config.ACPAgentConfig, cwd string) (*AgentProcess, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if p, ok := pm.processes[scopeKey]; ok {
		select {
		case <-p.done:
			// Process died — clean up and respawn below.
			pm.log.Warn("acp: process dead, respawning", logger.StringField("scope", scopeKey))
			delete(pm.processes, scopeKey)
		default:
			return p, nil
		}
	}

	p, err := pm.spawn(ctx, scopeKey, agentCfg, cwd)
	if err != nil {
		return nil, err
	}
	pm.processes[scopeKey] = p
	return p, nil
}

func (pm *ProcessManager) spawn(ctx context.Context, scopeKey string, agentCfg config.ACPAgentConfig, cwd string) (*AgentProcess, error) {
	cmd := exec.CommandContext(ctx, agentCfg.Command, agentCfg.Args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for k, v := range agentCfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("acp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("acp: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("acp: start process: %w", err)
	}

	autoApprove := false
	if agentCfg.AutoApprove != nil {
		autoApprove = *agentCfg.AutoApprove
	}

	client := NewChatbotACPClient(autoApprove, pm.log)
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	// Initialize the connection.
	_, err = conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: 1,
		ClientInfo: &acp.Implementation{
			Name:    "chatbot",
			Version: "1.0.0",
		},
	})
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("acp: initialize: %w", err)
	}

	// Create a new session.
	mcpServers := ConvertMCPServers(agentCfg.MCPServers)
	sessResp, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: mcpServers,
	})
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("acp: new session: %w", err)
	}

	// Optionally set session mode.
	if agentCfg.DefaultMode != "" {
		_, err = conn.SetSessionMode(ctx, acp.SetSessionModeRequest{
			SessionId: sessResp.SessionId,
			ModeId:    acp.SessionModeId(agentCfg.DefaultMode),
		})
		if err != nil {
			pm.log.Warn("acp: set session mode failed", logger.ErrorField(err))
		}
	}

	// Optionally set session model.
	if agentCfg.DefaultModel != "" {
		_, err = conn.SetSessionModel(ctx, acp.SetSessionModelRequest{
			SessionId: sessResp.SessionId,
			ModelId:   acp.ModelId(agentCfg.DefaultModel),
		})
		if err != nil {
			pm.log.Warn("acp: set session model failed", logger.ErrorField(err))
		}
	}

	pm.log.Info("acp: process spawned",
		logger.StringField("scope", scopeKey),
		logger.StringField("session", string(sessResp.SessionId)),
	)

	return &AgentProcess{
		conn:      conn,
		client:    client,
		cmd:       cmd,
		sessionID: sessResp.SessionId,
		done:      conn.Done(),
	}, nil
}

// Remove kills and removes a process by scope key.
func (pm *ProcessManager) Remove(scopeKey string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, ok := pm.processes[scopeKey]
	if !ok {
		return fmt.Errorf("acp: no process for scope %q", scopeKey)
	}
	delete(pm.processes, scopeKey)

	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// Shutdown kills all managed processes.
func (pm *ProcessManager) Shutdown() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for key, p := range pm.processes {
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		delete(pm.processes, key)
	}
}
