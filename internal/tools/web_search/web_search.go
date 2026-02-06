// Package web_search provides a web search tool for the chatbot.
package web_search //nolint:revive // var-naming: using underscores for domain clarity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	Query      string `json:"query" jsonschema:"required" jsonschema_description:"The search query to execute"`
	NumResults int    `json:"num_results,omitempty" jsonschema_description:"Number of results (default: 10, max: 100)"`
	Page       int    `json:"page,omitempty" jsonschema_description:"Page number for pagination (default: 1)"`
	Location   string `json:"location,omitempty" jsonschema_description:"Location for localized results (e.g. 'New York')"`
	SafeSearch string `json:"safe_search,omitempty" jsonschema_description:"Safe search: 'active' or 'off' (default: off)"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Result represents the result of the web search tool
type Result struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

// searchAPIResponse represents the response from SearchAPI.io
type searchAPIResponse struct {
	OrganicResults []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic_results"`
	SearchInformation struct {
		TotalResults string `json:"total_results"`
	} `json:"search_information"`
}

// searchClient handles the HTTP communication with the search API
type searchClient struct {
	apiKey  string
	baseURL string
	timeout time.Duration
}

func (c *searchClient) search(ctx tool.Context, args Args) Result {
	reqURL, err := c.buildRequestURL(args)
	if err != nil {
		return Result{Query: args.Query, Error: fmt.Sprintf("failed to build request: %v", err)}
	}

	body, statusCode, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return Result{Query: args.Query, Error: err.Error()}
	}

	if statusCode != http.StatusOK {
		return Result{Query: args.Query, Error: fmt.Sprintf("API error (status %d): %s", statusCode, body)}
	}

	return c.parseResponse(args.Query, body)
}

func (c *searchClient) buildRequestURL(args Args) (string, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/search")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("api_key", c.apiKey)
	q.Set("engine", "google")
	q.Set("q", args.Query)

	if args.NumResults > 0 {
		num := args.NumResults
		if num > 100 {
			num = 100
		}
		q.Set("num", strconv.Itoa(num))
	}

	if args.Page > 0 {
		q.Set("page", strconv.Itoa(args.Page))
	}

	if args.Location != "" {
		q.Set("location", args.Location)
	}

	if args.SafeSearch != "" {
		q.Set("safe", args.SafeSearch)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *searchClient) doRequest(ctx tool.Context, reqURL string) ([]byte, int, error) {
	client := &http.Client{Timeout: c.timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

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
	var apiResp searchAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return Result{Query: query, Error: fmt.Sprintf("failed to parse response: %v", err)}
	}

	results := make([]SearchResult, len(apiResp.OrganicResults))
	for i, r := range apiResp.OrganicResults {
		results[i] = SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: r.Snippet,
		}
	}

	return Result{
		Query:   query,
		Results: results,
	}
}

// New creates a new web search tool
func New(cfg Config) (tool.Tool, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("search API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://www.searchapi.io"
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
