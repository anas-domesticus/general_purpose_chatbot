# Health Check Package

Liveness and readiness health checks with HTTP handlers, failure thresholds, and Kubernetes-ready endpoints.

## Usage

```go
checker := health.New(
    health.WithTimeout(5 * time.Second),
    health.WithFailureThreshold(3),
    health.WithLogger(log), // *zap.SugaredLogger
)

// Liveness: is the process alive?
checker.AddLivenessCheck(health.NewCheckFunc("process", func(ctx context.Context) error {
    return nil
}))

// Readiness: can we handle traffic?
checker.AddReadinessCheck(health.NewCheckFunc("slack", func(ctx context.Context) error {
    return slackConnector.Ready()
}))

// Serve HTTP endpoints
mux := http.NewServeMux()
mux.HandleFunc("/health/live", checker.LivenessHandler())
mux.HandleFunc("/health/ready", checker.ReadinessHandler())
```

## Response Format

```json
{
  "status": "healthy",
  "checks": {
    "slack": { "status": "ok", "latency": "1ms" }
  }
}
```

- `200 OK` — all checks passed
- `503 Service Unavailable` — one or more checks failed

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithTimeout(d)` | 5s | Timeout per health check |
| `WithFailureThreshold(n)` | 3 | Consecutive failures before unhealthy |
| `WithLogger(l)` | nil | `*zap.SugaredLogger` for logging |