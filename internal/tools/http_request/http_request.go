// Package http_request provides HTTP request tools for the chatbot.
package http_request

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// Args represents the arguments for the HTTP request tool
type Args struct {
	Method  string            `json:"method" jsonschema:"required" jsonschema_description:"HTTP method (GET, POST, PUT, DELETE, etc.)"`
	URL     string            `json:"url" jsonschema:"required" jsonschema_description:"Target URL for the request"`
	Headers map[string]string `json:"headers,omitempty" jsonschema_description:"Optional HTTP headers to include in the request"`
	Body    string            `json:"body,omitempty" jsonschema_description:"Optional request body for POST, PUT, etc."`
}

// Result represents the result of the HTTP request tool
type Result struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// handler is the HTTP request tool handler
func handler(ctx tool.Context, args Args) (Result, error) {
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
		return Result{
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
		return Result{
			Error: "Request failed: " + err.Error(),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{
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

	return Result{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    headers,
		Body:       string(respBody),
	}, nil
}

// New creates a new HTTP request tool
func New() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "http_request",
		Description: "Make arbitrary HTTP requests to external APIs and services",
	}, handler)
}
