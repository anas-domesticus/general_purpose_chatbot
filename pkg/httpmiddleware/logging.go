package httpmiddleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// HTTPLogger provides HTTP request/response logging middleware
type HTTPLogger struct {
	logger logger.Logger
}

// NewHTTPLogger creates a new HTTP logger middleware
func NewHTTPLogger(log logger.Logger) *HTTPLogger {
	return &HTTPLogger{
		logger: log,
	}
}

// Middleware returns the HTTP logging middleware
func (h *HTTPLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get correlation ID from header (guaranteed to be valid UUID by correlation middleware)
		correlationID := r.Header.Get("X-Correlation-ID")

		// Create logger with request fields
		requestLogger := h.logger.WithFields(
			logger.ClientIPField(r.RemoteAddr),
			logger.HTTPMethodField(r.Method),
			logger.HTTPPathField(r.URL.Path),
			logger.CorrelationIDField(correlationID),
		)

		// Log incoming request
		requestLogger.Info("HTTP request received")

		// Create wrapped response writer to capture response details
		wrappedWriter := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Process request
		next.ServeHTTP(wrappedWriter, r)

		// Calculate duration
		duration := time.Since(start)

		// Log response
		responseLogger := requestLogger.WithFields(
			logger.StringField("http_status", strconv.Itoa(wrappedWriter.Status())),
			logger.StringField("response_bytes", strconv.Itoa(wrappedWriter.BytesWritten())),
			logger.DurationField("duration", duration),
		)

		responseLogger.Info("HTTP response sent")
	})
}

// RequestLogger creates a logger with request context for use in handlers
func (h *HTTPLogger) RequestLogger(r *http.Request) logger.Logger {
	// Get correlation ID from header (guaranteed to be valid UUID by correlation middleware)
	correlationID := r.Header.Get("X-Correlation-ID")

	return h.logger.WithFields(
		logger.ClientIPField(r.RemoteAddr),
		logger.HTTPMethodField(r.Method),
		logger.HTTPPathField(r.URL.Path),
		logger.CorrelationIDField(correlationID),
	)
}