# Logger

Type-safe structured logging with concrete types and configurable output.

## Purpose
Provides structured logging with concrete types, no `interface{}` usage, and support for testing via custom output writers. Includes middleware for both HTTP and gRPC request logging with automatic correlation ID tracking.

## Features

- **Type-safe**: All fields use concrete types, no `interface{}`
- **Immutable**: `WithFields()` returns new logger instances
- **Testable**: Custom output writers for testing
- **HTTP Middleware**: Chi-compatible middleware for HTTP request/response logging
- **gRPC Integration**: Built-in unary interceptor for request logging
- **Structured**: JSON or text output formats
- **Configurable**: Multiple log levels and custom outputs
- **Correlation ID**: Automatic correlation ID tracking across HTTP and gRPC

## Basic Usage

```go
import "github.com/lewisedginton/go_project_boilerplate/pkg/logger"

// Create logger
logger := logger.NewLogger(logger.Config{
    Level:   logger.InfoLevel,
    Format:  "json",
    Service: "my-service",
})

// Basic logging with fields
logger.Info("User logged in",
    logger.StringField("user_id", "123"),
    logger.HTTPStatusField(200),
)

// Add persistent fields (immutable)
requestLogger := logger.WithFields(
    logger.CorrelationIDField("req-456"),
    logger.StringField("endpoint", "/api/users"),
)

requestLogger.Info("Processing request")
requestLogger.Error("Request failed", logger.ErrorField(err))
```

## HTTP Middleware (Chi-compatible)

```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
)

// Create logger
logger := logger.NewLogger(logger.Config{
    Level:   logger.InfoLevel,
    Format:  "json",
    Service: "http-service",
})

// Use as HTTP middleware
r := chi.NewRouter()
r.Use(logger.HTTPMiddleware)

r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
    // Logger automatically adds:
    // - Correlation ID (X-Correlation-ID header)
    // - HTTP method, path, status, response size
    // - Request/response timing
    // - Client IP
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("user data"))
})
```

## gRPC Interceptor

```go
import (
    "google.golang.org/grpc"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
)

// Create logger
logger := logger.NewLogger(logger.Config{
    Level:   logger.InfoLevel,
    Format:  "json",
    Service: "grpc-service",
})

// Use as gRPC interceptor
server := grpc.NewServer(
    grpc.UnaryInterceptor(logger.GrpcRequestsInterceptor),
)

// Logger automatically adds:
// - Correlation ID from metadata
// - gRPC method, status code, timing
// - Request start/completion logging
```

## Configuration

```go
config := logger.Config{
    Level:   logger.InfoLevel,  // DebugLevel, InfoLevel, WarnLevel, ErrorLevel
    Format:  "json",           // "json" or "text"
    Service: "my-service",     // Added to all log entries
    Output:  os.Stdout,        // Optional: custom writer (default: os.Stdout)
}

logger := logger.NewLogger(config)
```

## Field Helpers

### HTTP Fields
```go
logger.HTTPMethodField("GET")
logger.HTTPPathField("/api/users")
logger.HTTPStatusField(200)
logger.ClientIPField("192.168.1.1")
```

### gRPC Fields
```go
logger.GrpcMethodField("/users.UserService/GetUser")
logger.GrpcServiceField("users.UserService")
logger.GrpcCodeField(codes.OK)
```

### Common Fields
```go
logger.StringField("key", "value")
logger.IntField("count", 42)
logger.Int64Field("id", 123456789)
logger.BoolField("enabled", true)
logger.DurationField("duration", time.Second)
logger.TimeField("timestamp", time.Now())
logger.ErrorField(err)
logger.CorrelationIDField("req-123")

// Generic field for any type
logger.Field("data", someStruct)
```

## Correlation ID Tracking

### Automatic HTTP Correlation ID
```go
// Middleware automatically:
// 1. Checks for X-Correlation-ID header
// 2. Generates UUID if missing or invalid
// 3. Adds to context and response header
// 4. Includes in all log entries

r.Use(logger.HTTPMiddleware)
```

### Manual Correlation ID Management
```go
// Extract from context
correlationID := logger.GetCorrelationIDFromContext(ctx)

// Add to logger
requestLogger := logger.WithCorrelationID(correlationID)

// Get logger with correlation ID from context
contextLogger := logger.GetLoggerFromContext(ctx, baseLogger)
```

### gRPC Correlation ID
```go
// Automatically extracts/generates correlation ID from gRPC metadata
// Uses "x-correlation-id" metadata key
server := grpc.NewServer(
    grpc.UnaryInterceptor(logger.GrpcRequestsInterceptor),
)
```

## Testing

```go
func TestWithLogger(t *testing.T) {
    var buf bytes.Buffer
    
    // Create logger with custom output for testing
    logger := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "test-service",
        Output:  &buf, // Capture output
    })
    
    logger.Info("test message", logger.StringField("key", "value"))
    
    // Parse and verify JSON output
    var logEntry map[string]interface{}
    json.Unmarshal(buf.Bytes(), &logEntry)
    
    assert.Equal(t, "test message", logEntry["msg"])
    assert.Equal(t, "value", logEntry["key"])
    assert.Equal(t, "test-service", logEntry["service"])
}
```

## Integration with Config

Works seamlessly with the config package:

```go
type ServiceConfig struct {
    config.CommonConfig `yaml:",inline"`  // Includes log_level
    Http                config.HttpServerConfig `yaml:"http,inline"`
    
    Service string `env:"SERVICE_NAME" yaml:"service" default:"my-service"`
}

cfg := ServiceConfig{}
config.GetConfig(&cfg, "config.yaml", true)

// Create logger from config
logger := logger.NewLogger(logger.Config{
    Level:   logger.ParseLevel(cfg.LogLevel), // From CommonConfig
    Format:  "json",
    Service: cfg.Service,
})
```

## Example Output

**JSON Format:**
```json
{
  "level": "info",
  "msg": "HTTP request received",
  "time": "2024-01-15T10:30:45Z",
  "service": "api-service",
  "correlation_id": "123e4567-e89b-12d3-a456-426614174000",
  "http_method": "GET",
  "http_path": "/api/users/123",
  "client_ip": "192.168.1.100"
}
```

**Text Format:**
```
time="2024-01-15T10:30:45Z" level=info msg="HTTP request received" service="api-service" correlation_id="123e4567-e89b-12d3-a456-426614174000" http_method="GET" http_path="/api/users/123" client_ip="192.168.1.100"
```