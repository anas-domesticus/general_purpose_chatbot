# Health Check Package

A comprehensive health check package for Go applications with Kubernetes-ready liveness and readiness endpoints, HTTP handlers (Chi-compatible), gRPC integration, and built-in checkers for common services.

## Features

- **Kubernetes-ready** liveness (`/health`) and readiness (`/ready`) endpoints
- **HTTP handlers** compatible with Chi router and standard `http.HandlerFunc`
- **gRPC integration** using official gRPC health checking protocol v1
- **Built-in checkers** for Redis, HTTP endpoints, and custom checks
- **Failure thresholds** and configurable timeouts
- **Concurrent execution** of health checks
- **Structured logging** integration
- **Context cancellation** support

## Quick Start

```go
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/lewisedginton/go_project_boilerplate/pkg/health"
	"github.com/lewisedginton/go_project_boilerplate/pkg/health/checkers"
	"github.com/lewisedginton/go_project_boilerplate/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Create a health checker
	healthChecker := health.New(
		health.WithTimeout(5*time.Second),
		health.WithFailureThreshold(3),
		health.WithLogger(logger.New(logger.Config{})),
	)

	// Add liveness checks (things that if they fail, the pod should be restarted)
	healthChecker.AddLivenessCheck(health.NewCheckFunc("basic", func(ctx context.Context) error {
		// Basic application health check
		return nil
	}))

	// Add readiness checks (things that if they fail, the pod shouldn't receive traffic)
	redisClient := redis.NewClient(&redis.Options{Addr: "redis:6379"})
	healthChecker.AddReadinessCheck(checkers.NewRedisChecker(redisClient, "cache"))
	healthChecker.AddReadinessCheck(checkers.NewHTTPChecker("http://api.example.com/health", "external-api"))

	// Set up HTTP routes
	r := chi.NewRouter()
	r.Get("/health", healthChecker.LivenessHandler())
	r.Get("/ready", healthChecker.ReadinessHandler())

	// Start server
	http.ListenAndServe(":8080", r)
}
```

## Configuration Options

### HealthChecker Options

- **`WithTimeout(duration)`**: Set timeout for individual health checks (default: 5s)
- **`WithFailureThreshold(count)`**: Number of consecutive failures before reporting unhealthy (default: 3)
- **`WithLogger(logger)`**: Set logger for health check operations

### Liveness vs Readiness

- **Liveness checks** (`/health`): Determine if the process should be restarted
  - Use for: Memory leaks, deadlocks, corrupt state
  - Example: Database connection pool health, critical service dependencies

- **Readiness checks** (`/ready`): Determine if the service can handle requests  
  - Use for: External dependencies, startup procedures, graceful degradation
  - Example: Redis cache, external APIs, database migrations

## Built-in Checkers

### HTTP Checker

Check external HTTP endpoints:

```go
// Basic HTTP checker
checker := checkers.NewHTTPChecker("https://api.example.com/health", "external-api")

// With custom HTTP client
client := &http.Client{Timeout: 2 * time.Second}
checker := checkers.NewHTTPCheckerWithClient("https://api.example.com/health", "external-api", client)

healthChecker.AddReadinessCheck(checker)
```

**Behavior:**
- 2xx and 4xx status codes are considered healthy (service is responding)
- 5xx status codes are considered unhealthy
- Network errors are considered unhealthy

### Redis Checker

Check Redis connectivity:

```go
redisClient := redis.NewClient(&redis.Options{
	Addr: "redis:6379",
})

// Basic Redis checker
checker := checkers.NewRedisChecker(redisClient, "cache")

// Multiple Redis instances
cacheChecker := checkers.NewRedisChecker(cacheClient, "redis-cache")
sessionChecker := checkers.NewRedisChecker(sessionClient, "redis-sessions")

healthChecker.AddReadinessCheck(checker)
```

### Custom Checkers

Implement the `Check` interface:

```go
type Check interface {
	Name() string
	Check(ctx context.Context) error
}

type DatabaseChecker struct {
	db *sql.DB
}

func (d *DatabaseChecker) Name() string {
	return "database"
}

func (d *DatabaseChecker) Check(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Or use CheckFunc for simple checks
checker := health.NewCheckFunc("custom", func(ctx context.Context) error {
	// Your custom check logic here
	return nil
})
```

## HTTP Integration

### Standard HTTP

```go
mux := http.NewServeMux()
mux.HandleFunc("/health", healthChecker.LivenessHandler())
mux.HandleFunc("/ready", healthChecker.ReadinessHandler())

http.ListenAndServe(":8080", mux)
```

### Chi Router

```go
r := chi.NewRouter()
r.Get("/health", healthChecker.LivenessHandler())
r.Get("/ready", healthChecker.ReadinessHandler())

http.ListenAndServe(":8080", r)
```

### Response Format

Health endpoints return JSON responses:

```json
{
  "status": "healthy",
  "checks": {
    "database": {
      "status": "ok",
      "latency": "2.5ms"
    },
    "redis": {
      "status": "error",
      "error": "redis ping failed: dial tcp: connection refused",
      "latency": "100ms"
    }
  },
  "message": "health checks failed: [redis]"
}
```

**HTTP Status Codes:**
- `200 OK`: All checks passed
- `503 Service Unavailable`: One or more checks failed

## gRPC Integration

Register with gRPC server for official health checking protocol:

```go
import (
	"google.golang.org/grpc"
)

func main() {
	server := grpc.NewServer()
	
	healthChecker := health.New()
	healthChecker.AddReadinessCheck(checkers.NewRedisChecker(redisClient, "cache"))
	
	// Register with default 5-second update interval
	updater := healthChecker.RegisterWithGRPC(server)
	defer updater.Stop()
	
	// Or with custom interval
	updater := healthChecker.RegisterWithGRPCAndInterval(server, 10*time.Second)
	defer updater.Stop()
	
	server.Serve(listener)
}
```

The gRPC health service:
- Periodically checks readiness and updates status
- Uses empty service name ("") for overall server health
- Supports `SERVING` and `NOT_SERVING` statuses
- Gracefully handles shutdown

## Kubernetes Integration

### Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      containers:
      - name: myapp
        image: myapp:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2
```

### Service Example

```yaml
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: myapp
```

### Best Practices

**Liveness Probe Configuration:**
- `initialDelaySeconds`: Allow time for application startup
- `periodSeconds`: Check every 10-30 seconds
- `timeoutSeconds`: 5-10 seconds max
- `failureThreshold`: 3-5 failures before restart

**Readiness Probe Configuration:**  
- `initialDelaySeconds`: Shorter than liveness (5-10 seconds)
- `periodSeconds`: Check every 5-10 seconds  
- `timeoutSeconds`: 3-5 seconds max
- `failureThreshold`: 1-3 failures before removing from service

## Failure Threshold Logic

The health checker implements failure threshold logic to prevent flapping:

```go
healthChecker := health.New(health.WithFailureThreshold(3))
```

- Consecutive failures are tracked per check
- A check is only reported as unhealthy after reaching the threshold
- Successful checks reset the failure count to 0
- This prevents temporary network hiccups from causing restarts

## Logging Integration

The health checker integrates with the boilerplate's structured logging:

```go
logger := logger.New(logger.Config{
	Level: logger.InfoLevel,
})

healthChecker := health.New(health.WithLogger(logger))
```

Log messages include:
- Check names and results
- Error messages and latency
- Failure counts and thresholds
- gRPC status transitions

## Testing

The package includes comprehensive tests:

```bash
# Run all health package tests
go test ./pkg/health/...

# Run with verbose output
go test -v ./pkg/health/...

# Run specific test
go test -v ./pkg/health -run TestHealthChecker_Timeout
```

## Advanced Examples

### Graceful Degradation

```go
// Critical checks for liveness (will restart pod if they fail)
healthChecker.AddLivenessCheck(checkers.NewRedisChecker(primaryRedis, "primary-cache"))

// Non-critical checks for readiness (will remove from load balancer if they fail)
healthChecker.AddReadinessCheck(checkers.NewHTTPChecker("http://optional-service/health", "optional"))
```

### Multiple Redis Instances

```go
cacheClient := redis.NewClient(&redis.Options{Addr: "cache:6379"})
sessionClient := redis.NewClient(&redis.Options{Addr: "sessions:6379"})

healthChecker.AddReadinessCheck(checkers.NewRedisChecker(cacheClient, "cache"))
healthChecker.AddReadinessCheck(checkers.NewRedisChecker(sessionClient, "sessions"))
```

### Custom Database Checker

```go
type PostgresChecker struct {
	db *sql.DB
}

func (p *PostgresChecker) Name() string { return "postgres" }

func (p *PostgresChecker) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	var result int
	err := p.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("postgres health check failed: %w", err)
	}
	return nil
}
```

## Dependencies

- **Standard library**: `context`, `encoding/json`, `fmt`, `net/http`, `sync`, `time`
- **gRPC**: `google.golang.org/grpc` and `google.golang.org/grpc/health`
- **Redis** (optional): `github.com/redis/go-redis/v9` (only if using Redis checker)
- **Testing**: `github.com/stretchr/testify`
- **Logging**: Internal logger package

The Redis dependency is only required if you use the Redis checker. The core health checking functionality has no external dependencies beyond gRPC.

## Contributing

When adding new checkers:

1. Implement the `Check` interface
2. Follow the naming pattern: `NewXXXChecker(client, name)`
3. Include comprehensive tests
4. Document the checker behavior and examples
5. Consider failure modes and timeouts

The health package is designed to be extensible and robust for production Kubernetes environments.