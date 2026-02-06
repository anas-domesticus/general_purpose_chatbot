// Package executor handles message execution through the AI agent.
package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Executor handles execution of connector operations
type Executor struct {
	sessionService  session.Service
	artifactService artifact.Service
	memoryService   memory.Service
	appName         string
	agentFactory    agents.AgentFactory
	log             logger.Logger
}

// Config holds configuration for the executor.
type Config struct {
	AgentFactory    agents.AgentFactory
	AppName         string
	SessionService  session.Service
	ArtifactService artifact.Service
	MemoryService   memory.Service // Optional: if nil, memory is disabled
	Logger          logger.Logger
}

// NewExecutor creates a new Executor instance (legacy signature for compatibility).
func NewExecutor(
	agentFactory agents.AgentFactory,
	appName string,
	sessionService session.Service,
	artifactService artifact.Service,
) (*Executor, error) {
	return NewExecutorWithConfig(Config{
		AgentFactory:    agentFactory,
		AppName:         appName,
		SessionService:  sessionService,
		ArtifactService: artifactService,
	})
}

// NewExecutorWithConfig creates a new Executor instance with full configuration.
func NewExecutorWithConfig(cfg Config) (*Executor, error) {
	if cfg.AgentFactory == nil {
		return nil, fmt.Errorf("agent factory cannot be nil")
	}

	return &Executor{
		sessionService:  cfg.SessionService,
		artifactService: cfg.ArtifactService,
		memoryService:   cfg.MemoryService,
		appName:         cfg.AppName,
		agentFactory:    cfg.AgentFactory,
		log:             cfg.Logger,
	}, nil
}

// Execute processes a message request and returns the response.
//
//nolint:gocyclo,revive // Message processing requires handling multiple validation and error paths
func (e *Executor) Execute(
	ctx context.Context,
	req MessageRequest,
	guidanceProvider agents.PlatformSpecificGuidanceProvider,
	userInfoFunc agents.UserInfoFunc,
) (MessageResponse, error) {
	// Validate input
	if req.UserID == "" {
		return MessageResponse{}, fmt.Errorf("userID is required")
	}
	if req.SessionID == "" {
		return MessageResponse{}, fmt.Errorf("sessionID is required")
	}
	if req.Message == "" {
		return MessageResponse{}, fmt.Errorf("message is required")
	}

	// Ensure session exists, create if needed
	_, err := e.sessionService.Get(ctx, &session.GetRequest{
		AppName:   e.appName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		// Session doesn't exist, create it
		_, err = e.sessionService.Create(ctx, &session.CreateRequest{
			AppName:   e.appName,
			UserID:    req.UserID,
			SessionID: req.SessionID,
		})
		if err != nil {
			return MessageResponse{}, fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Create content from user message
	content := genai.NewContentFromText(req.Message, "user")

	// Configure run
	runConfig := agent.RunConfig{
		StreamingMode: agent.StreamingModeNone,
	}

	agentInstance, err := e.agentFactory(guidanceProvider, userInfoFunc)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("failed to create agent instance: %w", err)
	}

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:         e.appName,
		SessionService:  e.sessionService,
		ArtifactService: e.artifactService,
		Agent:           agentInstance,
	})
	if err != nil {
		return MessageResponse{}, fmt.Errorf("failed to create runner: %w", err)
	}

	// Execute via runner
	eventIterator := r.Run(ctx, req.UserID, req.SessionID, content, runConfig)

	// Iterate and collect response text
	var responseText strings.Builder
	var lastError error

	for event, err := range eventIterator {
		if err != nil {
			lastError = err
			break
		}

		if event == nil {
			continue
		}

		// Check for error in event
		if event.ErrorMessage != "" {
			lastError = fmt.Errorf("agent error [%s]: %s", event.ErrorCode, event.ErrorMessage)
			break
		}

		// Extract text from content parts
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					responseText.WriteString(part.Text)
				}
			}
		}
	}

	if lastError != nil {
		return MessageResponse{}, fmt.Errorf("failed to execute agent: %w", lastError)
	}

	// Add session to memory after successful execution
	if e.memoryService != nil {
		e.addSessionToMemory(ctx, req.UserID, req.SessionID)
	}

	return MessageResponse{
		Text: responseText.String(),
	}, nil
}

// addSessionToMemory adds the current session to memory storage.
func (e *Executor) addSessionToMemory(ctx context.Context, userID, sessionID string) {
	sess, err := e.sessionService.Get(ctx, &session.GetRequest{
		AppName:   e.appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		if e.log != nil {
			e.log.Warn("Failed to get session for memory",
				logger.StringField("session_id", sessionID),
				logger.ErrorField(err))
		}
		return
	}

	if err := e.memoryService.AddSession(ctx, sess.Session); err != nil {
		if e.log != nil {
			e.log.Warn("Failed to add session to memory",
				logger.StringField("session_id", sessionID),
				logger.ErrorField(err))
		}
	}
}
