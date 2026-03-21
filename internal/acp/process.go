package acpclient

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"go.uber.org/zap"
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
	log       *zap.SugaredLogger
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(log *zap.SugaredLogger) *ProcessManager {
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
			pm.log.Warnw("acp: existing process dead, will respawn",
				"scope", scopeKey, "pid", p.cmd.Process.Pid)
			delete(pm.processes, scopeKey)
		default:
			pm.log.Debugw("acp: reusing existing process",
				"scope", scopeKey, "pid", p.cmd.Process.Pid)
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
	pm.log.Infow("acp: spawning agent process",
		"scope", scopeKey,
		"command", agentCfg.Command,
		"args", agentCfg.Args,
		"cwd", cwd,
	)

	cmd := exec.CommandContext(ctx, agentCfg.Command, agentCfg.Args...) //nolint:gosec // command is from trusted config
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for k, v := range agentCfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		pm.log.Errorw("acp: failed to create stdin pipe",
			"scope", scopeKey, "command", agentCfg.Command, "error", err)
		return nil, fmt.Errorf("acp: stdin pipe for %q: %w", agentCfg.Command, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		pm.log.Errorw("acp: failed to create stdout pipe",
			"scope", scopeKey, "command", agentCfg.Command, "error", err)
		return nil, fmt.Errorf("acp: stdout pipe for %q: %w", agentCfg.Command, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		pm.log.Errorw("acp: failed to create stderr pipe",
			"scope", scopeKey, "command", agentCfg.Command, "error", err)
		return nil, fmt.Errorf("acp: stderr pipe for %q: %w", agentCfg.Command, err)
	}

	if err := cmd.Start(); err != nil {
		pm.log.Errorw("acp: failed to start agent process",
			"scope", scopeKey, "command", agentCfg.Command, "cwd", cwd, "error", err)
		return nil, fmt.Errorf("acp: start %q in %q: %w", agentCfg.Command, cwd, err)
	}

	pid := cmd.Process.Pid
	pm.log.Infow("acp: agent process started, initializing ACP connection",
		"scope", scopeKey, "pid", pid)

	// Forward agent stderr to our logger in a background goroutine.
	go pm.drainStderr(stderr, scopeKey, pid)

	client := NewChatbotACPClient(pm.log)
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	// Enable SDK-level connection diagnostics (JSON-RPC parse errors,
	// peer disconnects, notification handler errors).
	conn.SetLogger(newZapSlogAdapter(pm.log, scopeKey, pid))

	// Initialize the ACP connection.
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientInfo: &acp.Implementation{
			Name:    "chatbot",
			Version: "1.0.0",
		},
		ClientCapabilities: acp.ClientCapabilities{
			Fs:       acp.FileSystemCapability{ReadTextFile: false, WriteTextFile: false},
			Terminal: false,
		},
	})
	if err != nil {
		pm.log.Errorw("acp: ACP initialize handshake failed",
			"scope", scopeKey, "pid", pid, "error", err, "error_detail", acpErrorDetail(err))
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("acp: initialize with %q (pid %d): %w", agentCfg.Command, pid, err)
	}

	pm.log.Infow("acp: ACP connection initialized",
		"scope", scopeKey, "pid", pid,
		"agent_protocol_version", initResp.ProtocolVersion,
		"agent_capabilities", fmt.Sprintf("%+v", initResp.AgentCapabilities),
	)

	// Create a new session.
	mcpServers := ConvertMCPServers(agentCfg.MCPServers)
	sessResp, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: mcpServers,
	})
	if err != nil {
		pm.log.Errorw("acp: failed to create ACP session",
			"scope", scopeKey, "pid", pid, "cwd", cwd, "error", err, "error_detail", acpErrorDetail(err))
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("acp: new session with %q (pid %d): %w", agentCfg.Command, pid, err)
	}

	// Optionally set session mode.
	if agentCfg.DefaultMode != "" {
		_, err = conn.SetSessionMode(ctx, acp.SetSessionModeRequest{
			SessionId: sessResp.SessionId,
			ModeId:    acp.SessionModeId(agentCfg.DefaultMode),
		})
		if err != nil {
			pm.log.Warnw("acp: set session mode failed (non-fatal)",
				"scope", scopeKey, "mode", agentCfg.DefaultMode, "error", err)
		}
	}

	// Optionally set session model.
	if agentCfg.DefaultModel != "" {
		_, err = conn.SetSessionModel(ctx, acp.SetSessionModelRequest{
			SessionId: sessResp.SessionId,
			ModelId:   acp.ModelId(agentCfg.DefaultModel),
		})
		if err != nil {
			pm.log.Warnw("acp: set session model failed (non-fatal)",
				"scope", scopeKey, "model", agentCfg.DefaultModel, "error", err)
		}
	}

	pm.log.Infow("acp: process ready",
		"scope", scopeKey,
		"pid", pid,
		"session", string(sessResp.SessionId),
	)

	return &AgentProcess{
		conn:      conn,
		client:    client,
		cmd:       cmd,
		sessionID: sessResp.SessionId,
		done:      conn.Done(),
	}, nil
}

// killAndReap kills a process and waits for it to exit to avoid zombies.
func killAndReap(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// drainStderr reads agent stderr line-by-line and logs each line.
func (pm *ProcessManager) drainStderr(stderr io.Reader, scope string, pid int) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		pm.log.Warnw("acp: agent stderr",
			"scope", scope, "pid", pid, "line", scanner.Text())
	}
}

// acpErrorDetail extracts structured error info from *acp.RequestError, if present.
func acpErrorDetail(err error) string {
	if re, ok := err.(*acp.RequestError); ok { //nolint:errorlint // acp SDK returns concrete *RequestError
		return fmt.Sprintf("code=%d message=%q data=%v", re.Code, re.Message, re.Data)
	}
	return ""
}

// newZapSlogAdapter creates an slog.Logger that bridges to a zap.SugaredLogger
// with scope and pid context. This is used for conn.SetLogger() to get
// SDK-level connection diagnostics.
func newZapSlogAdapter(log *zap.SugaredLogger, scope string, pid int) *slog.Logger {
	return slog.New(&zapSlogHandler{
		log: log.With("scope", scope, "pid", pid, "component", "acp-sdk"),
	})
}

// zapSlogHandler is a minimal slog.Handler that delegates to zap.SugaredLogger.
type zapSlogHandler struct {
	log   *zap.SugaredLogger
	attrs []slog.Attr
}

func (h *zapSlogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *zapSlogHandler) Handle(_ context.Context, r slog.Record) error {
	kvs := make([]interface{}, 0, 2*len(h.attrs)+2*r.NumAttrs())
	for _, a := range h.attrs {
		kvs = append(kvs, a.Key, a.Value.Any())
	}
	r.Attrs(func(a slog.Attr) bool {
		kvs = append(kvs, a.Key, a.Value.Any())
		return true
	})
	switch {
	case r.Level >= slog.LevelError:
		h.log.Errorw(r.Message, kvs...)
	case r.Level >= slog.LevelWarn:
		h.log.Warnw(r.Message, kvs...)
	case r.Level >= slog.LevelInfo:
		h.log.Infow(r.Message, kvs...)
	default:
		h.log.Debugw(r.Message, kvs...)
	}
	return nil
}

func (h *zapSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &zapSlogHandler{log: h.log, attrs: append(h.attrs, attrs...)}
}

func (h *zapSlogHandler) WithGroup(name string) slog.Handler {
	return &zapSlogHandler{log: h.log.Named(name), attrs: h.attrs}
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

	pm.log.Infow("acp: killing process", "scope", scopeKey, "pid", p.cmd.Process.Pid)
	killAndReap(p.cmd)
	return nil
}

// Shutdown kills all managed processes.
func (pm *ProcessManager) Shutdown() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.log.Infow("acp: shutting down all agent processes", "count", len(pm.processes))
	for key, p := range pm.processes {
		pm.log.Infow("acp: killing process on shutdown", "scope", key, "pid", p.cmd.Process.Pid)
		killAndReap(p.cmd)
		delete(pm.processes, key)
	}
}
