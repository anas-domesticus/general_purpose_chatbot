package executor

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

type Executor struct {
	runner         *runner.Runner
	sessionService session.Service
	appName        string
}

func NewExecutor(agent agent.Agent) (*Executor, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent cannot be nil")
	}

	// Create in-memory session service
	sessionService := session.InMemoryService()

	appName := "slack_chatbot"

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          agent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	return &Executor{
		runner:         r,
		sessionService: sessionService,
		appName:        appName,
	}, nil
}

func (e *Executor) Execute(ctx context.Context, req MessageRequest) (MessageResponse, error) {
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

	// Execute via runner
	eventIterator := e.runner.Run(ctx, req.UserID, req.SessionID, content, runConfig)

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
