package checkers

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HTTPChecker checks the health of an HTTP endpoint.
type HTTPChecker struct {
	url    string
	client *http.Client
	name   string
}

// NewHTTPChecker creates a new HTTP endpoint health checker.
// The url parameter is the endpoint to check (e.g., "http://api.example.com/health").
// The name parameter allows customization of the check name (e.g., "external-api").
// If name is empty, defaults to the URL.
func NewHTTPChecker(url string, name string) *HTTPChecker {
	if name == "" {
		name = url
	}

	return &HTTPChecker{
		url:  url,
		name: name,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewHTTPCheckerWithClient creates a new HTTP endpoint health checker with a custom HTTP client.
func NewHTTPCheckerWithClient(url string, name string, client *http.Client) *HTTPChecker {
	if name == "" {
		name = url
	}

	return &HTTPChecker{
		url:    url,
		name:   name,
		client: client,
	}
}

// Name returns the name of this health check.
func (h *HTTPChecker) Name() string {
	return h.name
}

// Check performs an HTTP GET request to the configured endpoint.
// Returns an error if the request fails or returns a 5xx status code.
func (h *HTTPChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("unhealthy status code: %d", resp.StatusCode)
	}

	return nil
}