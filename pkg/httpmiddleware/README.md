# HTTP Middleware Package

A comprehensive HTTP middleware package for Go applications using the Chi router. Provides a modular, feature-flag-driven approach to applying common HTTP middleware with sensible defaults.

## Features

- **Modular Design**: Individual middleware functions that can be used independently
- **Feature Flags**: Enable/disable middleware components via configuration
- **Chi Router Integration**: Seamlessly integrates with go-chi/chi router
- **Production Ready**: Sensible defaults for production environments
- **Logger Integration**: Works with the project's logger package for structured logging

## Included Middleware

1. **Correlation ID**: Generates unique request tracking IDs
2. **Security Headers**: Adds security headers via the unrolled/secure package
3. **Real IP**: Extracts real client IP from proxy headers
4. **HTTP Logging**: Structured request/response logging
5. **Recovery**: Recovers from panics and logs them
6. **Path Stripping**: Removes path prefixes from URLs
7. **CORS**: Cross-Origin Resource Sharing support
8. **Timeout**: Request timeout handling
9. **Compression**: Response compression
10. **Heartbeat**: Health check endpoint at /ping

## Quick Start

### Basic Usage with Defaults

```go
package main

import (
    "github.com/go-chi/chi/v5"
    "github.com/lewisedginton/go_project_boilerplate/pkg/httpmiddleware"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
)

func main() {
    // Create logger
    log := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "my-service",
    })

    // Create router and apply middleware with logging
    r := chi.NewRouter()
    httpmiddleware.WithLogger(r, log)

    // Add your routes
    r.Get("/api/users", getUsersHandler)
    
    // Start server...
}
```

### Custom Configuration

```go
package main

import (
    "time"
    "github.com/go-chi/chi/v5"
    "github.com/lewisedginton/go_project_boilerplate/pkg/httpmiddleware"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
)

func main() {
    log := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "my-service",
    })

    // Customize configuration
    config := httpmiddleware.DefaultConfig()
    config.Logger = log
    config.EnableLogging = true
    config.StripPrefix = "/api/v1"
    config.EnableStripPrefix = true
    config.Timeout = 30 * time.Second
    
    // Custom CORS settings
    config.CORS.AllowedOrigins = []string{"https://myapp.com"}
    config.CORS.AllowCredentials = true

    // Apply to router
    r := chi.NewRouter()
    httpmiddleware.ApplyToRouter(r, config)

    // Add routes (will have "/api/v1" stripped)
    r.Get("/users", getUsersHandler) // Responds to /api/v1/users
    
    // Start server...
}
```

## Configuration Options

### Core Settings

```go
type Config struct {
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
```

### Default Configuration

The `DefaultConfig()` function provides production-ready defaults:

```go
config := httpmiddleware.DefaultConfig()
// Returns:
// - EnableCorrelationID: true
// - EnableRecovery: true
// - EnableCORS: true (with permissive defaults)
// - EnableSecurity: true
// - EnableCompression: true
// - EnableHeartbeat: true (adds /ping endpoint)
// - EnableRealIP: true
// - EnableTimeout: true (60s timeout)
// - EnableLogging: false (must set Logger and enable explicitly)
// - EnableStripPrefix: false (enable only if StripPrefix is set)
```

## CORS Configuration

The CORS middleware supports comprehensive configuration:

```go
type CORSConfig struct {
    AllowedMethods   []string
    AllowedHeaders   []string
    AllowedOrigins   []string
    ExposedHeaders   []string
    AllowCredentials bool
    MaxAge           int
}
```

Default CORS settings:
- **Methods**: GET, POST, PUT, DELETE, OPTIONS
- **Headers**: Origin, Content-Type, Authorization, X-CSRF-Token
- **Origins**: https://\*, http://\* (permissive for development)
- **Credentials**: false
- **MaxAge**: 300 seconds

## Individual Middleware Usage

Each middleware can be used independently:

```go
// Just correlation ID
r.Use(httpmiddleware.CorrelationID())

// Just security headers
r.Use(httpmiddleware.Security(nil)) // Uses defaults

// Just path stripping
r.Use(httpmiddleware.StripPrefix("/api/v1"))

// Custom CORS
corsConfig := httpmiddleware.DefaultCORSConfig()
corsConfig.AllowedOrigins = []string{"https://myapp.com"}
r.Use(httpmiddleware.CORS(corsConfig))
```

## Middleware Order

When using `ApplyToRouter()`, middleware is applied in this order (outermost to innermost):

1. **CorrelationID** - Tracks requests
2. **Security** - Security headers
3. **RealIP** - Client IP extraction
4. **Logging** - Request/response logging
5. **Recovery** - Panic recovery
6. **StripPrefix** - Path manipulation
7. **CORS** - Cross-origin handling
8. **Timeout** - Request timeouts
9. **Compression** - Response compression
10. **Heartbeat** - Health endpoint

This order ensures that correlation IDs are available for logging, security headers are applied early, and compression happens at the end.

## Logging Integration

The middleware integrates with the project's logger package to provide structured logging:

```go
// In your handlers, get a request-scoped logger
func getUsersHandler(w http.ResponseWriter, r *http.Request) {
    // Get logger with request context (correlation ID, method, path, etc.)
    reqLogger := httpLogger.RequestLogger(r)
    
    reqLogger.Info("Processing users request")
    
    // Your handler logic...
}
```

Log entries automatically include:
- `correlation_id`: Unique request identifier
- `http_method`: HTTP method
- `http_path`: Request path
- `client_ip`: Client IP address
- `duration`: Request processing time
- `http_status`: Response status code
- `response_bytes`: Response size

## Health Checks

When `EnableHeartbeat` is true, a `/ping` endpoint is automatically added:

```bash
curl http://localhost:8080/ping
# Returns: HTTP 200 OK
```

## Security Headers

The security middleware (via unrolled/secure) adds headers like:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security` (HTTPS only)

Customize via the `Security` field in config:

```go
config.Security = &secure.Options{
    AllowedHosts: []string{"myapp.com"},
    SSLRedirect:  true,
    // ... other options
}
```

## Examples

### API Server with Authentication

```go
func setupRouter() *chi.Mux {
    log := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "api-server",
    })

    config := httpmiddleware.DefaultConfig()
    config.Logger = log
    config.EnableLogging = true
    config.StripPrefix = "/api/v1"
    config.EnableStripPrefix = true
    
    // Restrict CORS for production
    config.CORS.AllowedOrigins = []string{"https://webapp.com"}
    config.CORS.AllowCredentials = true

    r := chi.NewRouter()
    httpmiddleware.ApplyToRouter(r, config)

    // Routes (prefix will be stripped)
    r.Route("/auth", func(r chi.Router) {
        r.Post("/login", loginHandler)
        r.Post("/logout", logoutHandler)
    })
    
    r.Route("/users", func(r chi.Router) {
        r.Use(authMiddleware) // Your auth middleware
        r.Get("/", getUsersHandler)
        r.Post("/", createUserHandler)
    })

    return r
}
```

### Development vs Production

```go
func createConfig(env string) httpmiddleware.Config {
    config := httpmiddleware.DefaultConfig()
    
    if env == "development" {
        // More permissive CORS for dev
        config.CORS.AllowedOrigins = []string{"http://localhost:*"}
        config.EnableLogging = true // Enable request logging in dev
    } else {
        // Stricter settings for production
        config.CORS.AllowedOrigins = []string{"https://myapp.com"}
        config.CORS.AllowCredentials = true
        
        // Custom security headers for production
        config.Security = &secure.Options{
            AllowedHosts:         []string{"api.myapp.com"},
            SSLRedirect:          true,
            STSSeconds:           31536000,
            STSIncludeSubdomains: true,
        }
    }
    
    return config
}
```

## Testing

The package includes comprehensive tests. Run them with:

```bash
go test ./pkg/httpmiddleware/...
```

Key test coverage:
- Default configuration validation
- Individual middleware functionality
- Integration tests with full middleware stack
- CORS behavior
- Path stripping logic
- Logging output verification

## Dependencies

- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/go-chi/cors` - CORS middleware
- `github.com/unrolled/secure` - Security headers
- `github.com/google/uuid` - UUID generation
- Logger package from this project

## Best Practices

1. **Always use correlation IDs** for request tracking
2. **Enable logging in development** to debug issues
3. **Customize CORS** for production (don't use wildcard origins)
4. **Set appropriate timeouts** based on your use case
5. **Use security headers** in production
6. **Test middleware order** if using custom combinations
7. **Monitor health endpoints** for service availability

## Contributing

When adding new middleware:
1. Follow the existing pattern of feature flags in `Config`
2. Add comprehensive tests
3. Update the README with usage examples
4. Consider middleware order implications