// Package web_search provides a web search tool for the chatbot.
package web_search //nolint:revive // var-naming: using underscores for domain clarity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// Config holds configuration for the web search tool
type Config struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// Args represents the arguments for the web search tool
type Args struct {
	Query            string `json:"query" jsonschema:"required" jsonschema_description:"The search query to execute"`
	MaxResults       int    `json:"max_results,omitempty" jsonschema_description:"Maximum number of results to return (default: 5, max: 10)"`
	IncludeAnswer    bool   `json:"include_answer,omitempty" jsonschema_description:"Include an AI-generated answer summarizing the results"`
	SearchDepth      string `json:"search_depth,omitempty" jsonschema_description:"Search depth: 'basic' for quick results or 'advanced' for comprehensive results (default: basic)"`
	IncludeDomains   string `json:"include_domains,omitempty" jsonschema_description:"Comma-separated list of domains to include in search results"`
	ExcludeDomains   string `json:"exclude_domains,omitempty" jsonschema_description:"Comma-separated list of domains to exclude from search results"`
	IncludeRawContent bool  `json:"include_raw_content,omitempty" jsonschema_description:"Include raw HTML content of the pages"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Score   float64 `json:"score,omitempty"`
}

// Result represents the result of the web search tool
type Result struct {
	Query   string         `json:"query"`
	Answer  string         `json:"answer,omitempty"`
	Results []SearchResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

// tavilyRequest represents the request body for Tavily API
type tavilyRequest struct {
	APIKey            string   `json:"api_key"`
	Query             string   `json:"query"`
	MaxResults        int      `json:"max_results,omitempty"`
	IncludeAnswer     bool     `json:"include_answer,omitempty"`
	SearchDepth       string   `json:"search_depth,omitempty"`
	IncludeDomains    []string `json:"include_domains,omitempty"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`
	IncludeRawContent bool     `json:"include_raw_content,omitempty"`
}

// tavilyResponse represents the response from Tavily API
type tavilyResponse struct {
	Answer  string `json:"answer,omitempty"`
	Query   string `json:"query"`
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

// New creates a new web search tool
func New(cfg Config) (tool.Tool, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("search API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.tavily.com"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	handler := func(ctx tool.Context, args Args) (Result, error) {
		// Set default values
		maxResults := args.MaxResults
		if maxResults <= 0 {
			maxResults = 5
		}
		if maxResults > 10 {
			maxResults = 10
		}

		searchDepth := args.SearchDepth
		if searchDepth == "" {
			searchDepth = "basic"
		}

		// Build request
		reqBody := tavilyRequest{
			APIKey:            cfg.APIKey,
			Query:             args.Query,
			MaxResults:        maxResults,
			IncludeAnswer:     args.IncludeAnswer,
			SearchDepth:       searchDepth,
			IncludeRawContent: args.IncludeRawContent,
		}

		// Parse domain filters
		if args.IncludeDomains != "" {
			reqBody.IncludeDomains = parseDomains(args.IncludeDomains)
		}
		if args.ExcludeDomains != "" {
			reqBody.ExcludeDomains = parseDomains(args.ExcludeDomains)
		}

		// Serialize request
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return Result{
				Query: args.Query,
				Error: fmt.Sprintf("failed to serialize request: %v", err),
			}, nil
		}

		// Create HTTP client
		client := &http.Client{
			Timeout: cfg.Timeout,
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/search", bytes.NewBuffer(jsonBody))
		if err != nil {
			return Result{
				Query: args.Query,
				Error: fmt.Sprintf("failed to create request: %v", err),
			}, nil
		}

		req.Header.Set("Content-Type", "application/json")

		// Execute request
		resp, err := client.Do(req)
		if err != nil {
			return Result{
				Query: args.Query,
				Error: fmt.Sprintf("request failed: %v", err),
			}, nil
		}
		defer func() { _ = resp.Body.Close() }()

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return Result{
				Query: args.Query,
				Error: fmt.Sprintf("failed to read response: %v", err),
			}, nil
		}

		// Check for errors
		if resp.StatusCode != http.StatusOK {
			return Result{
				Query: args.Query,
				Error: fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
			}, nil
		}

		// Parse response
		var tavilyResp tavilyResponse
		if err := json.Unmarshal(body, &tavilyResp); err != nil {
			return Result{
				Query: args.Query,
				Error: fmt.Sprintf("failed to parse response: %v", err),
			}, nil
		}

		// Convert to result
		results := make([]SearchResult, len(tavilyResp.Results))
		for i, r := range tavilyResp.Results {
			results[i] = SearchResult{
				Title:   r.Title,
				URL:     r.URL,
				Content: r.Content,
				Score:   r.Score,
			}
		}

		return Result{
			Query:   tavilyResp.Query,
			Answer:  tavilyResp.Answer,
			Results: results,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "web_search",
		Description: "Search the web for current information. Use this tool to find up-to-date information about any topic, news, facts, or data that may not be in your training data.",
	}, handler)
}

// parseDomains splits a comma-separated domain string into a slice
func parseDomains(domains string) []string {
	var result []string
	for _, d := range bytes.Split([]byte(domains), []byte(",")) {
		trimmed := bytes.TrimSpace(d)
		if len(trimmed) > 0 {
			result = append(result, string(trimmed))
		}
	}
	return result
}
