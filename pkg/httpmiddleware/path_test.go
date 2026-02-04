package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripPrefix(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	})

	t.Run("strips matching prefix", func(t *testing.T) {
		middleware := StripPrefix("/api/v1")
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Body.String() != "/users" {
			t.Errorf("Expected '/users', got '%s'", recorder.Body.String())
		}
	})

	t.Run("does not modify path without matching prefix", func(t *testing.T) {
		middleware := StripPrefix("/api/v1")
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v2/users", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Body.String() != "/api/v2/users" {
			t.Errorf("Expected '/api/v2/users', got '%s'", recorder.Body.String())
		}
	})

	t.Run("empty prefix does nothing", func(t *testing.T) {
		middleware := StripPrefix("")
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Body.String() != "/api/v1/users" {
			t.Errorf("Expected '/api/v1/users', got '%s'", recorder.Body.String())
		}
	})

	t.Run("partial prefix match is not stripped", func(t *testing.T) {
		middleware := StripPrefix("/api/v1")
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/api/v11/users", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Body.String() != "/api/v11/users" {
			t.Errorf("Expected '/api/v11/users', got '%s'", recorder.Body.String())
		}
	})
}
