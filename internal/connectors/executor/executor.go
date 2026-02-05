// Package executor handles message execution through the AI agent.
package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Executor handles execution of connector operations
type Executor struct {
	sessionService  session.Service
	artifactService artifact.Service
	appName         string
	agentFactory    agents.AgentFactory
}

// NewExecutor creates a new Executor instance
func NewExecutor(
	agentFactory agents.AgentFactory,
	appName string,
	sessionService session.Service,
	artifactService artifact.Service,
) (*Executor, error) {
	if agentFactory == nil {
		return nil, fmt.Errorf("agent factory cannot be nil")
	}

	return &Executor{
		sessionService:  sessionService,
		artifactService: artifactService,
		appName:         appName,
		agentFactory:    agentFactory,
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

	return MessageResponse{
		Text: responseText.String(),
	}, nil
}
