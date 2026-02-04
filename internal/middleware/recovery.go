// Package middleware provides HTTP middleware components.
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// RecoveryConfig holds configuration for the recovery middleware
type RecoveryConfig struct {
	Logger              logger.Logger
	EnableStackTrace    bool          // Whether to log full stack traces
	ResponseMessage     string        // Custom message to return to clients
	ResponseContentType string        // Content type for error responses
	Timeout             time.Duration // Timeout for handling panics
}

// DefaultRecoveryConfig returns a sensible default configuration
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		EnableStackTrace:    true,
		ResponseMessage:     `{"error":"Internal server error","code":"INTERNAL_ERROR"}`,
		ResponseContentType: "application/json",
		Timeout:             30 * time.Second,
	}
}

// Recovery returns a middleware that recovers from panics and logs them
func Recovery(config RecoveryConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					handlePanic(w, r, err, config)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// handlePanic handles a recovered panic
func handlePanic(w http.ResponseWriter, r *http.Request, err interface{}, config RecoveryConfig) {
	// Get stack trace if enabled
	var stackTrace string
	if config.EnableStackTrace {
		stackTrace = string(debug.Stack())
	}

	// Log the panic with full details
	logPanic(r, err, stackTrace, config.Logger)

	// Set headers for error response
	w.Header().Set("Content-Type", config.ResponseContentType)
	w.Header().Set("Connection", "close") // Signal to close connection after response

	// Write error response
	w.WriteHeader(http.StatusInternalServerError)

	// Write response body
	if config.ResponseMessage != "" {
		_, _ = w.Write([]byte(config.ResponseMessage))
	}
}

// logPanic logs panic information
func logPanic(r *http.Request, panicErr interface{}, stackTrace string, log logger.Logger) {
	if log == nil {
		// Fallback to basic logging if no logger provided
		fmt.Printf("PANIC: %v\nRequest: %s %s\nStack:\n%s\n",
			panicErr, r.Method, r.URL.Path, stackTrace)
		return
	}

	fields := []logger.LogField{
		logger.StringField("panic_error", fmt.Sprintf("%v", panicErr)),
		logger.HTTPMethodField(r.Method),
		logger.HTTPPathField(r.URL.Path),
		logger.ClientIPField(getClientIP(r)),
		logger.StringField("user_agent", r.UserAgent()),
		logger.StringField("request_id", r.Header.Get("X-Request-ID")),
	}

	if stackTrace != "" {
		fields = append(fields, logger.StringField("stack_trace", stackTrace))
	}

	// Add query parameters if present
	if r.URL.RawQuery != "" {
		fields = append(fields, logger.StringField("query_params", r.URL.RawQuery))
	}

	// Add content length if available
	if r.ContentLength > 0 {
		fields = append(fields, logger.Int64Field("content_length", r.ContentLength))
	}

	log.Error("HTTP request panic recovered", fields...)
}

// getClientIP extracts the real client IP from various headers
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		for idx := 0; idx < len(xff); idx++ {
			if xff[idx] == ',' {
				return xff[:idx]
			}
		}
		return xff
	}

	// Check X-Real-IP header (common in nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Check CF-Connecting-IP (Cloudflare)
	if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
		return cfip
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// ErrorHandler creates a middleware that handles application errors gracefully
func ErrorHandler(config RecoveryConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a custom response writer to capture status codes and errors
			wrapped := &errorResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				logger:         config.Logger,
				request:        r,
			}

			next.ServeHTTP(wrapped, r)

			// Log errors for non-2xx responses
			if wrapped.statusCode >= 400 && config.Logger != nil {
				logHTTPError(r, wrapped.statusCode, config.Logger)
			}
		})
	}
}

// errorResponseWriter wraps http.ResponseWriter to capture error details
type errorResponseWriter struct {
	http.ResponseWriter
	statusCode int
	logger     logger.Logger
	request    *http.Request
}

// WriteHeader captures the status code
func (w *errorResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures write operations (status defaults to 200 if WriteHeader not called)
func (w *errorResponseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

// logHTTPError logs HTTP error responses
func logHTTPError(r *http.Request, statusCode int, log logger.Logger) {
	fields := []logger.LogField{
		logger.HTTPStatusField(statusCode),
		logger.HTTPMethodField(r.Method),
		logger.HTTPPathField(r.URL.Path),
		logger.ClientIPField(getClientIP(r)),
		logger.StringField("user_agent", r.UserAgent()),
	}

	// Add query parameters if present
	if r.URL.RawQuery != "" {
		fields = append(fields, logger.StringField("query_params", r.URL.RawQuery))
	}

	message := fmt.Sprintf("HTTP %d response", statusCode)

	switch {
	case statusCode >= 500:
		log.Error(message, fields...)
	case statusCode >= 400:
		log.Warn(message, fields...)
	default:
		log.Info(message, fields...)
	}
}

// TimeoutHandler creates a middleware that enforces request timeouts
func TimeoutHandler(timeout time.Duration, config RecoveryConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "Request timeout")
	}
}

// RequestLogging creates a middleware that logs all HTTP requests
func RequestLogging(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create wrapped writer to capture response details
			wrapped := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Log request completion
			duration := time.Since(start)

			fields := []logger.LogField{
				logger.HTTPMethodField(r.Method),
				logger.HTTPPathField(r.URL.Path),
				logger.HTTPStatusField(wrapped.statusCode),
				logger.DurationField("duration", duration),
				logger.ClientIPField(getClientIP(r)),
				logger.IntField("response_size", wrapped.bytesWritten),
			}

			if r.URL.RawQuery != "" {
				fields = append(fields, logger.StringField("query_params", r.URL.RawQuery))
			}

			log.Info("HTTP request completed", fields...)
		})
	}
}

// loggingResponseWriter wraps http.ResponseWriter for logging
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

// ChainMiddleware chains multiple middleware functions together
func ChainMiddleware(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		// Apply middleware in reverse order so they execute in the order they were passed
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
		return handler
	}
}
