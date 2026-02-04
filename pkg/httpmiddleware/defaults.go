package httpmiddleware

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/unrolled/secure"
)

// Config holds configuration for HTTP middleware application.
// Use DefaultConfig() for sensible defaults, then customize as needed.
type Config struct {
	// Core middleware settings
	Logger      logger.Logger   // Required for logging middleware
	StripPrefix string          // Path prefix to strip (e.g., "/api/v1")
	CORS        *CORSConfig     // CORS configuration
	Security    *secure.Options // Security headers configuration
	Timeout     time.Duration   // Request timeout duration

	// Feature flags for optional middleware
	EnableCorrelationID bool // Add correlation ID to requests
	EnableLogging       bool // Log HTTP requests (requires Logger)
	EnableRecovery      bool // Recover from panics
	EnableCORS          bool // Enable CORS headers
	EnableSecurity      bool // Add security headers
	EnableCompression   bool // Compress responses
	EnableHeartbeat     bool // Add /ping health endpoint
	EnableRealIP        bool // Extract real client IP
	EnableTimeout       bool // Add request timeouts
	EnableStripPrefix   bool // Strip path prefix (requires StripPrefix)
}

// DefaultConfig returns a production-ready middleware configuration.
// Logging is disabled by default - set Logger and EnableLogging=true to enable.
func DefaultConfig() Config {
	corsConfig := DefaultCORSConfig()
	return Config{
		// Core settings with sensible defaults
		Logger:      nil,
		StripPrefix: "",
		CORS:        &corsConfig,
		Security:    nil, // Uses secure package defaults
		Timeout:     60 * time.Second,

		// Enable production-ready middleware
		EnableCorrelationID: true,
		EnableLogging:       false, // Must set Logger and enable explicitly
		EnableRecovery:      true,
		EnableCORS:          true,
		EnableSecurity:      true,
		EnableCompression:   true,
		EnableHeartbeat:     true,
		EnableRealIP:        true,
		EnableTimeout:       true,
		EnableStripPrefix:   false, // Enable only if StripPrefix is set
	}
}

// ApplyToRouter applies the configured middleware to a Chi router in the recommended order.
// Middleware is applied in execution order (first applied = outermost layer).
//
// Execution order:
//  1. CorrelationID - Adds request correlation tracking
//  2. Security - Adds security headers
//  3. RealIP - Extracts real client IP
//  4. Logging - Logs HTTP requests
//  5. Recovery - Recovers from panics
//  6. StripPrefix - Removes path prefix
//  7. CORS - Handles cross-origin requests
//  8. Timeout - Adds request timeouts
//  9. Compression - Compresses responses
//
// 10. Heartbeat - Adds /ping health endpoint
func ApplyToRouter(router chi.Router, config Config) {
	applyMiddlewareInOrder(router, config)
}

// WithLogger is a convenience function that applies middleware with logging enabled.
// Uses DefaultConfig() with the provided logger and EnableLogging=true.
func WithLogger(router chi.Router, log logger.Logger) {
	config := DefaultConfig()
	config.Logger = log
	config.EnableLogging = true
	ApplyToRouter(router, config)
}

// applyMiddlewareInOrder applies middleware in the recommended execution order
func applyMiddlewareInOrder(router chi.Router, config Config) {
	if config.EnableCorrelationID {
		router.Use(CorrelationID())
	}

	if config.EnableSecurity {
		router.Use(Security(config.Security))
	}

	if config.EnableRealIP {
		router.Use(middleware.RealIP)
	}

	if config.EnableLogging && config.Logger != nil {
		httpLogger := NewHTTPLogger(config.Logger)
		router.Use(httpLogger.Middleware)
	}

	if config.EnableRecovery {
		router.Use(middleware.Recoverer)
	}

	if config.EnableStripPrefix && config.StripPrefix != "" {
		router.Use(StripPrefix(config.StripPrefix))
	}

	if config.EnableCORS && config.CORS != nil {
		router.Use(CORS(*config.CORS))
	}

	if config.EnableTimeout {
		router.Use(middleware.Timeout(config.Timeout))
	}

	if config.EnableCompression {
		router.Use(middleware.Compress(5))
	}

	if config.EnableHeartbeat {
		router.Use(middleware.Heartbeat("/ping"))
	}
}
