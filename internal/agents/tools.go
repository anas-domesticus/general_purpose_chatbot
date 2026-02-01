package agents

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// HTTPRequestArgs represents the arguments for the HTTP request tool
type HTTPRequestArgs struct {
	Method  string            `json:"method" jsonschema:"required" jsonschema_description:"HTTP method (GET, POST, PUT, DELETE, etc.)"`
	URL     string            `json:"url" jsonschema:"required" jsonschema_description:"Target URL for the request"`
	Headers map[string]string `json:"headers,omitempty" jsonschema_description:"Optional HTTP headers to include in the request"`
	Body    string            `json:"body,omitempty" jsonschema_description:"Optional request body for POST, PUT, etc."`
}

// HTTPRequestResult represents the result of the HTTP request tool
type HTTPRequestResult struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// handleHTTPRequest is the HTTP request tool handler
func handleHTTPRequest(ctx tool.Context, args HTTPRequestArgs) (HTTPRequestResult, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request body if provided
	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = bytes.NewBufferString(args.Body)
	}

	// Create HTTP request
	req, err := http.NewRequest(args.Method, args.URL, bodyReader)
	if err != nil {
		return HTTPRequestResult{
			Error: "Failed to create request: " + err.Error(),
		}, nil
	}

	// Add headers if provided
	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return HTTPRequestResult{
			Error: "Request failed: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return HTTPRequestResult{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Error:      "Failed to read response body: " + err.Error(),
		}, nil
	}

	// Convert response headers to map
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return HTTPRequestResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    headers,
		Body:       string(respBody),
	}, nil
}

// AgentInfoArgs represents the arguments for the agent info tool (no args needed)
type AgentInfoArgs struct{}

// AgentInfoResult represents the result of the agent info tool
type AgentInfoResult struct {
	AgentName    string   `json:"agent_name"`
	Model        string   `json:"model"`
	Platform     string   `json:"platform"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Framework    string   `json:"framework"`
}

// createAgentInfoHandler creates a platform-specific agent info handler
func createAgentInfoHandler(agentConfig AgentConfig, llmModel model.LLM) func(tool.Context, AgentInfoArgs) (AgentInfoResult, error) {
	return func(ctx tool.Context, args AgentInfoArgs) (AgentInfoResult, error) {
		return AgentInfoResult{
			AgentName:   agentConfig.Name,
			Platform:    agentConfig.Platform,
			Model:       llmModel.Name(),
			Description: agentConfig.Description,
			Capabilities: []string{
				"General conversation and Q&A",
				"Code analysis and programming help",
				"Technical discussions",
				"Creative writing assistance",
				"Problem solving and reasoning",
				"HTTP requests to external APIs and services",
			},
			Status:    "operational",
			Framework: "Google ADK Go v0.3.0",
		}, nil
	}
}
