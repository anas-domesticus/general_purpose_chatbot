# Metrics Package

A comprehensive Prometheus metrics collection package for Go applications, providing HTTP/gRPC middleware, built-in metrics server, and job metrics support.

## Features

- 🚀 **Prometheus Integration**: Full Prometheus client support with customizable metrics
- 🌐 **HTTP Middleware**: Chi-compatible middleware for automatic request tracking
- 🔗 **gRPC Interceptor**: Unary interceptor for gRPC request metrics
- 📊 **Built-in Metrics Server**: Standalone `/metrics` endpoint server
- ⚙️ **Job Metrics**: Track job processing statistics
- 🎛️ **Custom Metrics**: Support for custom Prometheus collectors
- 📝 **Comprehensive Testing**: Full test coverage with examples

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
)

func main() {
    // Create logger
    log := logger.NewLogger(logger.Config{
        Service: "my-service",
        Level:   logger.InfoLevel,
        Format:  "json",
    })

    // Create metrics with HTTP and gRPC support
    m := metrics.NewMetrics(
        true,  // enable HTTP counters
        true,  // enable gRPC counters  
        false, // disable job metrics
        log,
    )

    // Start metrics server on port 8080
    m.Listen(8080)

    // Your application logic here...
}
```

### HTTP Middleware with Chi

```go
package main

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
)

func main() {
    log := logger.NewLogger(logger.Config{Service: "web-app"})
    m := metrics.NewMetrics(true, false, false, log)

    // Start metrics server
    m.Listen(8080)

    // Setup Chi router with metrics middleware
    r := chi.NewRouter()
    r.Use(m.HTTPMiddleware())

    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello World"))
    })

    r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"users": []}`))
    })

    http.ListenAndServe(":3000", r)
}
```

### gRPC Integration

```go
package main

import (
    "google.golang.org/grpc"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
)

func main() {
    log := logger.NewLogger(logger.Config{Service: "grpc-service"})
    m := metrics.NewMetrics(false, true, false, log)

    // Start metrics server
    m.Listen(8080)

    // Setup gRPC server with metrics interceptor
    s := grpc.NewServer(
        grpc.UnaryInterceptor(m.GrpcRequestsInterceptor),
    )

    // Register your services...
    
    // Start server...
}
```

### Job Metrics

```go
package main

import (
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
)

func processJob() {
    log := logger.NewLogger(logger.Config{Service: "job-processor"})
    m := metrics.NewMetrics(false, false, true, log)

    // Start metrics server
    m.Listen(8080)

    // Track job processing
    m.JobMetricCounters[metrics.JobMetricTotal].Inc()
    
    err := doWork()
    if err != nil {
        m.JobMetricCounters[metrics.JobMetricTotalFailed].Inc()
    } else {
        m.JobMetricCounters[metrics.JobMetricTotalSuccess].Inc()
    }
}

func doWork() error {
    // Your job processing logic
    return nil
}
```

### Custom Metrics

```go
package main

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
)

func main() {
    log := logger.NewLogger(logger.Config{Service: "custom-app"})
    m := metrics.NewMetrics(false, false, false, log)

    // Create custom metrics
    customCounter := prometheus.NewCounter(prometheus.CounterOpts{
        Subsystem: "myapp",
        Name:      "custom_events_total",
        Help:      "Total custom events processed",
    })

    customGauge := prometheus.NewGauge(prometheus.GaugeOpts{
        Subsystem: "myapp", 
        Name:      "active_connections",
        Help:      "Number of active connections",
    })

    // Register custom metrics
    m.AddCustomMetric(customCounter)
    m.AddCustomMetric(customGauge)

    // Start metrics server
    m.Listen(8080)

    // Use your custom metrics
    customCounter.Inc()
    customGauge.Set(42)
}
```

## Available Metrics

When enabled, the package automatically creates the following metrics:

### HTTP Metrics
- `app_total_http_requests` - Total HTTP requests counter
- `app_http_request_duration_seconds` - HTTP request duration histogram
- `app_total_{status_code}_http_responses` - Response counters by status code (created dynamically)

### gRPC Metrics  
- `app_total_grpc_requests` - Total gRPC requests counter
- `app_grpc_request_duration_seconds` - gRPC request duration histogram
- `app_total_{code}_grpc_responses` - Response counters by gRPC code (created dynamically)

### Job Metrics
- `app_total_jobs_handled` - Total jobs processed
- `app_total_jobs_successful` - Successfully completed jobs
- `app_total_jobs_failed` - Failed jobs
- `app_total_jobs_killed` - Killed jobs

## Prometheus Configuration

### Scraping Configuration

Add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'my-service'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "3000:3000"  # Application port
      - "8080:8080"  # Metrics port
    environment:
      - METRICS_PORT=8080

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
```

### Kubernetes Example

```yaml
apiVersion: v1
kind: Service
metadata:
  name: app-metrics
  labels:
    app: my-app
spec:
  ports:
  - port: 8080
    name: metrics
  selector:
    app: my-app

---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: app-metrics
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
```

## Grafana Dashboard

Example queries for creating Grafana dashboards:

### HTTP Request Rate
```promql
rate(app_total_http_requests[5m])
```

### HTTP Response Time (95th percentile)
```promql
histogram_quantile(0.95, rate(app_http_request_duration_seconds_bucket[5m]))
```

### HTTP Error Rate
```promql
rate(app_total_4xx_http_responses[5m]) + rate(app_total_5xx_http_responses[5m])
```

### gRPC Success Rate
```promql
rate(app_total_0_grpc_responses[5m]) / rate(app_total_grpc_requests[5m])
```

## Testing

Run the test suite:

```bash
go test ./pkg/metrics/... -v
```

Run with coverage:

```bash
go test ./pkg/metrics/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Dependencies

- `github.com/prometheus/client_golang` - Prometheus client library
- `google.golang.org/grpc` - gRPC support
- `github.com/stretchr/testify` - Testing framework (dev dependency)

## Configuration

The package uses the logger from `github.com/lewisedginton/go_project_boilerplate/pkg/logger` and is compatible with the Chi router framework.

### Environment Variables

While not required, you may want to configure:

```bash
METRICS_PORT=8080          # Port for metrics server
LOG_LEVEL=info            # Logger level
SERVICE_NAME=my-service   # Service name for logs
```

## Best Practices

1. **Separate Metrics Port**: Run metrics on a separate port from your main application
2. **Resource Limits**: Set appropriate scrape intervals (15-60s typical)
3. **Label Cardinality**: Avoid high-cardinality labels (like user IDs)
4. **Custom Metrics**: Use descriptive names and help text
5. **Error Handling**: Don't let metrics collection crash your application

## Contributing

When adding new features:

1. Add comprehensive tests
2. Update this README with examples
3. Follow existing code patterns
4. Test with real Prometheus setup

## License

This package is part of the go_project_boilerplate and follows the same license terms.