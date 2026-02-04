package checkers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPChecker(t *testing.T) {
	t.Run("successful check with 200 OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHTTPChecker(server.URL, "test-api")
		assert.Equal(t, "test-api", checker.Name())

		err := checker.Check(context.Background())
		assert.NoError(t, err)
	})

	t.Run("uses URL as name when name is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHTTPChecker(server.URL, "")
		assert.Equal(t, server.URL, checker.Name())
	})

	t.Run("successful check with 2xx status codes", func(t *testing.T) {
		statusCodes := []int{200, 201, 204, 299}

		for _, code := range statusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
				}))
				defer server.Close()

				checker := NewHTTPChecker(server.URL, "test")
				err := checker.Check(context.Background())
				assert.NoError(t, err)
			})
		}
	})

	t.Run("successful check with 4xx status codes", func(t *testing.T) {
		// 4xx errors are considered "healthy" - the endpoint is responding
		statusCodes := []int{400, 401, 403, 404}

		for _, code := range statusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
				}))
				defer server.Close()

				checker := NewHTTPChecker(server.URL, "test")
				err := checker.Check(context.Background())
				assert.NoError(t, err)
			})
		}
	})

	t.Run("fails with 5xx status codes", func(t *testing.T) {
		statusCodes := []int{500, 502, 503, 504}

		for _, code := range statusCodes {
			t.Run(http.StatusText(code), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
				}))
				defer server.Close()

				checker := NewHTTPChecker(server.URL, "test")
				err := checker.Check(context.Background())
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unhealthy status code")
			})
		}
	})

	t.Run("fails when server is unreachable", func(t *testing.T) {
		checker := NewHTTPChecker("http://localhost:1", "unreachable")
		err := checker.Check(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http request failed")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHTTPChecker(server.URL, "slow")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := checker.Check(ctx)
		require.Error(t, err)
	})

	t.Run("uses custom HTTP client", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create client with very short timeout
		client := &http.Client{
			Timeout: 10 * time.Millisecond,
		}

		checker := NewHTTPCheckerWithClient(server.URL, "test", client)
		err := checker.Check(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http request failed")
	})

	t.Run("sends GET request", func(t *testing.T) {
		var receivedMethod string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHTTPChecker(server.URL, "test")
		err := checker.Check(context.Background())
		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, receivedMethod)
	})
}
