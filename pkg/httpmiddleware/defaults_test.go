package httpmiddleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check defaults
	if config.Timeout != 60*time.Second {
		t.Errorf("Expected timeout to be 60s, got %v", config.Timeout)
	}

	if config.CORS == nil {
		t.Error("Expected CORS config to be set")
	}

	if !config.EnableCorrelationID {
		t.Error("Expected correlation ID to be enabled by default")
	}

	if !config.EnableRecovery {
		t.Error("Expected recovery to be enabled by default")
	}

	if config.EnableLogging {
		t.Error("Expected logging to be disabled by default (requires logger)")
	}
}

func TestApplyWithDefaults(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(logger.Config{
		Level:   logger.DebugLevel,
		Format:  "json",
		Service: "test-service",
		Output:  &buf,
	})

	config := DefaultConfig()
	config.Logger = testLogger
	config.EnableLogging = true

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Apply middleware using Chi router
	router := chi.NewRouter()
	ApplyToRouter(router, config)
	router.Get("/test", testHandler.ServeHTTP)

	t.Run("middleware stack processes request successfully", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "test response" {
			t.Errorf("Expected 'test response', got %s", recorder.Body.String())
		}
	})

	t.Run("correlation ID is added to requests", func(t *testing.T) {
		var capturedID string
		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = r.Header.Get("X-Correlation-ID")
			w.WriteHeader(http.StatusOK)
		})

		router := chi.NewRouter()
		ApplyToRouter(router, config)
		router.Get("/capture", captureHandler.ServeHTTP)

		req := httptest.NewRequest("GET", "/capture", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if capturedID == "" {
			t.Error("Expected correlation ID to be set")
		}
	})

	t.Run("ping endpoint is available", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ping", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected ping endpoint to return 200, got %d", recorder.Code)
		}
	})
}

func TestApplyWithCustomConfig(t *testing.T) {
	config := Config{
		EnableCorrelationID: true,
		EnableRecovery:      false,
		EnableLogging:       false,
		EnableCORS:          false,
		EnableSecurity:      false,
		EnableCompression:   false,
		EnableHeartbeat:     false,
		EnableRealIP:        false,
		EnableTimeout:       false,
		EnableStripPrefix:   false,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router := chi.NewRouter()
	ApplyToRouter(router, config)
	router.Get("/test", testHandler.ServeHTTP)

	t.Run("minimal middleware stack works", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("correlation ID still works when enabled", func(t *testing.T) {
		var capturedID string
		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = r.Header.Get("X-Correlation-ID")
			w.WriteHeader(http.StatusOK)
		})

		router := chi.NewRouter()
		ApplyToRouter(router, config)
		router.Get("/capture", captureHandler.ServeHTTP)

		req := httptest.NewRequest("GET", "/capture", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if capturedID == "" {
			t.Error("Expected correlation ID to be set")
		}
	})

	t.Run("heartbeat disabled means no ping endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ping", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		// Without heartbeat middleware, ping endpoint should return 404 (no route defined)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", recorder.Code)
		}
	})
}

func TestStripPrefixIntegration(t *testing.T) {
	config := Config{
		StripPrefix:       "/api/v1",
		EnableStripPrefix: true,
		// Disable other middleware for cleaner test
		EnableCorrelationID: false,
		EnableRecovery:      false,
		EnableLogging:       false,
		EnableCORS:          false,
		EnableSecurity:      false,
		EnableCompression:   false,
		EnableHeartbeat:     false,
		EnableRealIP:        false,
		EnableTimeout:       false,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	})

	router := chi.NewRouter()
	ApplyToRouter(router, config)
	router.Get("/users", testHandler.ServeHTTP) // Route for the stripped path

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Body.String() != "/users" {
		t.Errorf("Expected '/users', got '%s'", recorder.Body.String())
	}
}

func TestWithLogger(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewLogger(logger.Config{
		Level:   logger.DebugLevel,
		Format:  "json",
		Service: "test-service",
		Output:  &buf,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Apply middleware with logger
	router := chi.NewRouter()
	WithLogger(router, testLogger)
	router.Get("/test", testHandler.ServeHTTP)

	t.Run("applies all default middleware with logging enabled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		if recorder.Body.String() != "test response" {
			t.Errorf("Expected 'test response', got %s", recorder.Body.String())
		}

		// Check that logging occurred
		if buf.Len() == 0 {
			t.Error("Expected log output, but got none")
		}
	})

	t.Run("correlation ID is automatically added", func(t *testing.T) {
		var capturedID string
		captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = r.Header.Get("X-Correlation-ID")
			w.WriteHeader(http.StatusOK)
		})

		router := chi.NewRouter()
		WithLogger(router, testLogger)
		router.Get("/capture", captureHandler.ServeHTTP)

		req := httptest.NewRequest("GET", "/capture", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if capturedID == "" {
			t.Error("Expected correlation ID to be set")
		}
	})

	t.Run("heartbeat endpoint is available", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ping", nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected ping endpoint to return 200, got %d", recorder.Code)
		}
	})
}
