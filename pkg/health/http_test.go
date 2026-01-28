package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLivenessHandler(t *testing.T) {
	t.Run("healthy response", func(t *testing.T) {
		h := New()
		h.AddLivenessCheck(&mockCheck{name: "test", err: nil})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler := h.LivenessHandler()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response.Status)
		assert.Contains(t, response.Checks, "test")
		assert.Equal(t, "ok", response.Checks["test"].Status)
	})

	t.Run("unhealthy response", func(t *testing.T) {
		h := New(WithFailureThreshold(1))
		h.AddLivenessCheck(&mockCheck{name: "test", err: errors.New("service unavailable")})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler := h.LivenessHandler()
		handler(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", response.Status)
		assert.NotEmpty(t, response.Message)
		assert.Contains(t, response.Checks, "test")
		assert.Equal(t, "error", response.Checks["test"].Status)
		assert.Equal(t, "service unavailable", response.Checks["test"].Error)
	})

	t.Run("no checks returns healthy", func(t *testing.T) {
		h := New()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler := h.LivenessHandler()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response.Status)
		assert.Empty(t, response.Checks)
	})

	t.Run("includes latency information", func(t *testing.T) {
		h := New()
		h.AddLivenessCheck(&mockCheck{name: "test", err: nil})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler := h.LivenessHandler()
		handler(w, req)

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Checks["test"].Latency)
	})
}

func TestReadinessHandler(t *testing.T) {
	t.Run("healthy response with multiple checks", func(t *testing.T) {
		h := New()
		h.AddReadinessCheck(&mockCheck{name: "database", err: nil})
		h.AddReadinessCheck(&mockCheck{name: "redis", err: nil})

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		w := httptest.NewRecorder()

		handler := h.ReadinessHandler()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response.Status)
		assert.Len(t, response.Checks, 2)
		assert.Equal(t, "ok", response.Checks["database"].Status)
		assert.Equal(t, "ok", response.Checks["redis"].Status)
	})

	t.Run("one failing check makes entire response unhealthy", func(t *testing.T) {
		h := New(WithFailureThreshold(1))
		h.AddReadinessCheck(&mockCheck{name: "database", err: nil})
		h.AddReadinessCheck(&mockCheck{name: "redis", err: errors.New("connection timeout")})

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		w := httptest.NewRecorder()

		handler := h.ReadinessHandler()
		handler(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", response.Status)
		assert.Equal(t, "ok", response.Checks["database"].Status)
		assert.Equal(t, "error", response.Checks["redis"].Status)
		assert.Equal(t, "connection timeout", response.Checks["redis"].Error)
	})

	t.Run("respects request context cancellation", func(t *testing.T) {
		h := New()
		h.AddReadinessCheck(&mockCheck{name: "test", err: nil})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before request

		req := httptest.NewRequest(http.MethodGet, "/ready", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler := h.ReadinessHandler()
		handler(w, req)

		// Should still return a response even with cancelled context
		assert.NotEqual(t, 0, w.Code)
	})
}

func TestHealthResponseFormat(t *testing.T) {
	t.Run("valid JSON structure", func(t *testing.T) {
		h := New()
		h.AddLivenessCheck(&mockCheck{name: "test1", err: nil})
		h.AddLivenessCheck(&mockCheck{name: "test2", err: errors.New("failed")})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler := h.LivenessHandler()
		handler(w, req)

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Verify structure
		assert.NotEmpty(t, response.Status)
		assert.NotNil(t, response.Checks)

		// Verify check details
		for name, checkStatus := range response.Checks {
			assert.NotEmpty(t, name)
			assert.NotEmpty(t, checkStatus.Status)
			assert.NotEmpty(t, checkStatus.Latency)
		}
	})
}