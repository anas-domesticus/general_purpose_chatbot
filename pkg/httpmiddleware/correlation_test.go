package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func TestCorrelationIdMiddleware(t *testing.T) {
	// Create test handler that captures correlation ID from both header and context
	var capturedHeaderID, capturedContextID string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaderID = r.Header.Get("X-Correlation-ID")
		capturedContextID = logger.GetCorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with correlation middleware
	handler := CorrelationID()(testHandler)

	t.Run("generates new UUID when no correlation ID exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if capturedHeaderID == "" {
			t.Error("Expected correlation ID to be generated in header")
		}

		if capturedContextID == "" {
			t.Error("Expected correlation ID to be added to context")
		}

		// Verify header and context have same ID
		if capturedHeaderID != capturedContextID {
			t.Errorf("Header ID (%s) should match context ID (%s)", capturedHeaderID, capturedContextID)
		}

		// Verify it's a valid UUID
		if _, err := uuid.Parse(capturedHeaderID); err != nil {
			t.Errorf("Generated correlation ID is not a valid UUID: %s", capturedHeaderID)
		}
	})

	t.Run("ignores valid existing UUID and generates new one", func(t *testing.T) {
		existingID := uuid.New().String()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", existingID)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if capturedHeaderID == existingID {
			t.Error("Expected existing correlation ID to be ignored and replaced in header")
		}

		if capturedContextID == existingID {
			t.Error("Expected existing correlation ID to be ignored and replaced in context")
		}

		// Verify header and context have same ID
		if capturedHeaderID != capturedContextID {
			t.Errorf("Header ID (%s) should match context ID (%s)", capturedHeaderID, capturedContextID)
		}

		// Verify the new ID is a valid UUID
		if _, err := uuid.Parse(capturedHeaderID); err != nil {
			t.Errorf("Generated correlation ID is not a valid UUID: %s", capturedHeaderID)
		}
	})

	t.Run("replaces invalid UUID with new valid one", func(t *testing.T) {
		invalidID := "not-a-uuid"
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", invalidID)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if capturedHeaderID == invalidID {
			t.Error("Expected invalid correlation ID to be replaced in header")
		}

		if capturedContextID == invalidID {
			t.Error("Expected invalid correlation ID to be replaced in context")
		}

		// Verify header and context have same ID
		if capturedHeaderID != capturedContextID {
			t.Errorf("Header ID (%s) should match context ID (%s)", capturedHeaderID, capturedContextID)
		}

		// Verify the new ID is a valid UUID
		if _, err := uuid.Parse(capturedHeaderID); err != nil {
			t.Errorf("Replacement correlation ID is not a valid UUID: %s", capturedHeaderID)
		}
	})

	t.Run("replaces malformed UUID with new valid one", func(t *testing.T) {
		malformedID := "123e4567-e89b-12d3-a456-42661417400" // Missing final character
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", malformedID)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if capturedHeaderID == malformedID {
			t.Error("Expected malformed correlation ID to be replaced in header")
		}

		if capturedContextID == malformedID {
			t.Error("Expected malformed correlation ID to be replaced in context")
		}

		// Verify header and context have same ID
		if capturedHeaderID != capturedContextID {
			t.Errorf("Header ID (%s) should match context ID (%s)", capturedHeaderID, capturedContextID)
		}

		// Verify the new ID is a valid UUID
		if _, err := uuid.Parse(capturedHeaderID); err != nil {
			t.Errorf("Replacement correlation ID is not a valid UUID: %s", capturedHeaderID)
		}
	})

	t.Run("replaces empty string with new valid UUID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", "")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if capturedHeaderID == "" {
			t.Error("Expected empty correlation ID to be replaced in header")
		}

		if capturedContextID == "" {
			t.Error("Expected empty correlation ID to be replaced in context")
		}

		// Verify header and context have same ID
		if capturedHeaderID != capturedContextID {
			t.Errorf("Header ID (%s) should match context ID (%s)", capturedHeaderID, capturedContextID)
		}

		// Verify the new ID is a valid UUID
		if _, err := uuid.Parse(capturedHeaderID); err != nil {
			t.Errorf("Replacement correlation ID is not a valid UUID: %s", capturedHeaderID)
		}
	})

	t.Run("replaces nil UUID with new valid one", func(t *testing.T) {
		nilUUID := "00000000-0000-0000-0000-000000000000"
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", nilUUID)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Nil UUID should be replaced with a new one
		if capturedHeaderID == nilUUID {
			t.Error("Expected nil UUID to be replaced in header")
		}

		if capturedContextID == nilUUID {
			t.Error("Expected nil UUID to be replaced in context")
		}

		// Verify header and context have same ID
		if capturedHeaderID != capturedContextID {
			t.Errorf("Header ID (%s) should match context ID (%s)", capturedHeaderID, capturedContextID)
		}

		// Verify the new ID is a valid UUID
		if _, err := uuid.Parse(capturedHeaderID); err != nil {
			t.Errorf("Replacement correlation ID is not a valid UUID: %s", capturedHeaderID)
		}
	})

	t.Run("generates unique IDs for different requests", func(t *testing.T) {
		var id1, id2 string

		// First request
		req1 := httptest.NewRequest("GET", "/test", nil)
		recorder1 := httptest.NewRecorder()
		handler.ServeHTTP(recorder1, req1)
		id1 = capturedHeaderID

		// Second request
		req2 := httptest.NewRequest("GET", "/test", nil)
		recorder2 := httptest.NewRecorder()
		handler.ServeHTTP(recorder2, req2)
		id2 = capturedHeaderID

		if id1 == id2 {
			t.Error("Expected different correlation IDs for different requests")
		}

		// Both should be valid UUIDs
		if _, err := uuid.Parse(id1); err != nil {
			t.Errorf("First correlation ID is not a valid UUID: %s", id1)
		}
		if _, err := uuid.Parse(id2); err != nil {
			t.Errorf("Second correlation ID is not a valid UUID: %s", id2)
		}
	})
}
