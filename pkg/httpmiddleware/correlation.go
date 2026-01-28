package httpmiddleware

import (
	"github.com/google/uuid"
	"net/http"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// CorrelationID middleware ensures every request has a unique correlation ID.
// Always generates a new correlation ID and ignores any client-provided correlation headers.
// This ensures we control our own correlation IDs for security and consistency.
// Also enriches the request context with the correlation ID.
func CorrelationID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always generate a new correlation ID, ignoring any client-provided headers
			correlationID := uuid.New().String()

			// Set the correlation ID in the request header
			r.Header.Set("X-Correlation-ID", correlationID)

			// Enrich context with correlation ID
			ctx := logger.WithCorrelationIDContext(r.Context(), correlationID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}