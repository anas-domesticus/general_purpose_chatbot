package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Bridge handles communication between external services and ADK agents
type Bridge struct {
	baseURL   string
	agentName string
	client    *http.Client
	logger    *log.Logger
}

// MessageRequest represents a request to send to an ADK agent
type MessageRequest struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
	Channel   string `json:"channel,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// MessageResponse represents the response from an ADK agent
type MessageResponse struct {
	Text   string                 `json:"text"`
	Events []Event                `json:"events,omitempty"`
	Error  string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Event represents an ADK event
type Event struct {
	ID        string    `json:"id"`
	Timestamp float64   `json:"timestamp"`
	Author    string    `json:"author"`
	Content   Content   `json:"content"`
	Actions   Actions   `json:"actions,omitempty"`
}

// Content represents the content of an ADK message
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"`
}

// Part represents a part of the content (text or function call)
type Part struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *FunctionCall `json:"functionCall,omitempty"`
}

// FunctionCall represents a function call in the content
type FunctionCall struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// Actions represents actions in an ADK event
type Actions struct {
	StateDelta   map[string]interface{} `json:"stateDelta,omitempty"`
	ArtifactDelta map[string]interface{} `json:"artifactDelta,omitempty"`
}

// ADKRequest represents the request format for ADK /run endpoint
type ADKRequest struct {
	AppName    string  `json:"appName"`
	UserID     string  `json:"userId"`
	SessionID  string  `json:"sessionId"`
	NewMessage Content `json:"newMessage"`
	Streaming  bool    `json:"streaming,omitempty"`
}

// NewBridge creates a new ADK bridge
func NewBridge(baseURL, agentName string) *Bridge {
	return &Bridge{
		baseURL:   baseURL,
		agentName: agentName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: log.New(os.Stdout, "[ADK-BRIDGE] ", log.LstdFlags|log.Lshortfile),
	}
}

// SendMessage sends a message to the ADK agent and returns the response
func (b *Bridge) SendMessage(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	// Create session if it doesn't exist
	err := b.createSessionIfNotExists(ctx, req.UserID, req.SessionID)
	if err != nil {
		b.logger.Printf("Warning: Could not create session (may already exist): %v", err)
		// Continue anyway - session might already exist
	}

	// Prepare ADK request
	adkReq := ADKRequest{
		AppName:   b.agentName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
		NewMessage: Content{
			Role: "user",
			Parts: []Part{
				{
					Text: req.Message,
				},
			},
		},
		Streaming: false,
	}

	// Marshal request
	reqBody, err := json.Marshal(adkReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	b.logger.Printf("Sending message to ADK: user=%s, session=%s, message=%s", req.UserID, req.SessionID, req.Message)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/run", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to ADK: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ADK returned error status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response as array of events
	var events []Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		b.logger.Printf("Failed to unmarshal ADK response as events: %v, raw response: %s", err, string(body))
		return &MessageResponse{
			Error: "Failed to parse ADK response",
		}, nil
	}

	b.logger.Printf("Received %d events from ADK", len(events))

	// Extract text response from the last model response event
	var responseText string
	for _, event := range events {
		if event.Content.Role == "model" {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					responseText += part.Text
				}
			}
		}
	}

	return &MessageResponse{
		Text:   responseText,
		Events: events,
	}, nil
}

// createSessionIfNotExists creates a session if it doesn't already exist
func (b *Bridge) createSessionIfNotExists(ctx context.Context, userID, sessionID string) error {
	url := fmt.Sprintf("%s/apps/%s/users/%s/sessions/%s", b.baseURL, b.agentName, userID, sessionID)
	
	// Create initial state (empty)
	reqBody, _ := json.Marshal(map[string]interface{}{})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create session request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	// Read response for logging
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		b.logger.Printf("Session created successfully: %s", sessionID)
		return nil
	} else if resp.StatusCode == http.StatusConflict {
		// Session already exists, that's fine
		b.logger.Printf("Session already exists: %s", sessionID)
		return nil
	} else {
		return fmt.Errorf("failed to create session, status: %d, body: %s", resp.StatusCode, string(body))
	}
}

// GetAgentList retrieves the list of available agents from ADK
func (b *Bridge) GetAgentList(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/list-apps", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ADK returned error status %d: %s", resp.StatusCode, string(body))
	}

	var agents []string
	err = json.NewDecoder(resp.Body).Decode(&agents)
	if err != nil {
		return nil, fmt.Errorf("failed to decode agent list: %w", err)
	}

	return agents, nil
}

// GetSession retrieves session information from ADK
func (b *Bridge) GetSession(ctx context.Context, userID, sessionID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/apps/%s/users/%s/sessions/%s", b.baseURL, b.agentName, userID, sessionID)
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ADK returned error status %d: %s", resp.StatusCode, string(body))
	}

	var session map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&session)
	if err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return session, nil
}

// HealthCheck verifies the ADK server is accessible
func (b *Bridge) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/list-apps", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ADK server is not accessible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ADK server returned status %d", resp.StatusCode)
	}

	b.logger.Println("ADK health check passed")
	return nil
}