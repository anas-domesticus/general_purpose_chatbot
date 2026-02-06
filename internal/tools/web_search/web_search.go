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
	Query             string `json:"query" jsonschema:"required" jsonschema_description:"The search query to execute"`
	MaxResults        int    `json:"max_results,omitempty" jsonschema_description:"Maximum number of results (default: 5, max: 10)"`
	IncludeAnswer     bool   `json:"include_answer,omitempty" jsonschema_description:"Include an AI-generated answer"`
	SearchDepth       string `json:"search_depth,omitempty" jsonschema_description:"'basic' or 'advanced' (default: basic)"`
	IncludeDomains    string `json:"include_domains,omitempty" jsonschema_description:"Comma-separated domains to include"`
	ExcludeDomains    string `json:"exclude_domains,omitempty" jsonschema_description:"Comma-separated domains to exclude"`
	IncludeRawContent bool   `json:"include_raw_content,omitempty" jsonschema_description:"Include raw HTML content"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
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

// searchClient handles the HTTP communication with the search API
type searchClient struct {
	apiKey  string
	baseURL string
	timeout time.Duration
}

func (c *searchClient) search(ctx tool.Context, args Args) Result {
	reqBody := c.buildRequest(args)

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return Result{Query: args.Query, Error: fmt.Sprintf("failed to serialize request: %v", err)}
	}

	body, statusCode, err := c.doRequest(ctx, jsonBody)
	if err != nil {
		return Result{Query: args.Query, Error: err.Error()}
	}

	if statusCode != http.StatusOK {
		return Result{Query: args.Query, Error: fmt.Sprintf("API error (status %d): %s", statusCode, body)}
	}

	return c.parseResponse(args.Query, body)
}

func (c *searchClient) buildRequest(args Args) tavilyRequest {
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

	req := tavilyRequest{
		APIKey:            c.apiKey,
		Query:             args.Query,
		MaxResults:        maxResults,
		IncludeAnswer:     args.IncludeAnswer,
		SearchDepth:       searchDepth,
		IncludeRawContent: args.IncludeRawContent,
	}

	if args.IncludeDomains != "" {
		req.IncludeDomains = parseDomains(args.IncludeDomains)
	}
	if args.ExcludeDomains != "" {
		req.ExcludeDomains = parseDomains(args.ExcludeDomains)
	}

	return req
}

func (c *searchClient) doRequest(ctx tool.Context, jsonBody []byte) ([]byte, int, error) {
	client := &http.Client{Timeout: c.timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

func (c *searchClient) parseResponse(query string, body []byte) Result {
	var tavilyResp tavilyResponse
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return Result{Query: query, Error: fmt.Sprintf("failed to parse response: %v", err)}
	}

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
	}
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

	client := &searchClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		timeout: cfg.Timeout,
	}

	handler := func(ctx tool.Context, args Args) (Result, error) {
		return client.search(ctx, args), nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "web_search",
		Description: "Search the web for current information about any topic, news, or facts.",
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
