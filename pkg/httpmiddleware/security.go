package httpmiddleware

import (
	"net/http"

	"github.com/go-chi/cors"
	"github.com/unrolled/secure"
)

// CORSConfig represents CORS configuration options
type CORSConfig struct {
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowedOrigins   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns a default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Authorization", "X-CSRF-Token"},
		AllowedOrigins:   []string{"https://*", "http://*"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}
}

// CORS middleware configures Cross-Origin Resource Sharing
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	corsOptions := &cors.Options{
		AllowedMethods:   config.AllowedMethods,
		AllowedHeaders:   config.AllowedHeaders,
		AllowedOrigins:   config.AllowedOrigins,
		ExposedHeaders:   config.ExposedHeaders,
		AllowCredentials: config.AllowCredentials,
		MaxAge:           config.MaxAge,
	}

	return cors.Handler(*corsOptions)
}

// Security middleware adds security headers
func Security(opts *secure.Options) func(http.Handler) http.Handler {
	var s *secure.Secure
	if opts == nil {
		s = secure.New()
	} else {
		s = secure.New(*opts)
	}

	return s.Handler
}