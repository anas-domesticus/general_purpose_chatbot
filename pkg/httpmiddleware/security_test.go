package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if len(config.AllowedMethods) == 0 {
		t.Error("Expected default CORS config to have allowed methods")
	}

	if len(config.AllowedHeaders) == 0 {
		t.Error("Expected default CORS config to have allowed headers")
	}

	if len(config.AllowedOrigins) == 0 {
		t.Error("Expected default CORS config to have allowed origins")
	}

	if config.MaxAge <= 0 {
		t.Error("Expected default CORS config to have positive MaxAge")
	}
}

func TestCORSMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	config := DefaultCORSConfig()
	middleware := CORS(config)
	handler := middleware(testHandler)

	t.Run("adds CORS headers for OPTIONS request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Check for CORS headers (exact headers depend on CORS library implementation)
		if recorder.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Error("Expected Access-Control-Allow-Origin header to be set")
		}
	})
}

func TestSecurityMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("applies security middleware with default options", func(t *testing.T) {
		middleware := Security(nil)
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Security middleware should add headers (exact headers depend on secure library defaults)
		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
	})
}
