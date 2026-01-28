# Config

Type-safe configuration loading from YAML files and environment variables with validation.

## Purpose
Generic configuration loader supporting struct tags for environment variables, defaults, required fields, and custom validation with proper precedence handling.

## Available Config Types

The config package provides reusable configuration types for common service components:

- **`CommonConfig`** - Common settings (log level)
- **`HttpServerConfig`** - HTTP server settings (port, timeouts, max header size)
- **`DatabaseConfig`** - Database connection settings (URL)
- **`MetricsConfig`** - Prometheus metrics configuration (enabled metrics, port, exposure)

Each config type includes a `Validate()` method for input validation.

## Usage

### Basic Example
```go
package main

import (
    "fmt"
    "net/http"
    
    "github.com/lewisedginton/go_project_boilerplate/pkg/config"
)

type ServiceConfig struct {
    config.CommonConfig `yaml:",inline"`
    Http               config.HttpServerConfig `yaml:"http,inline"`
    Database           config.DatabaseConfig   `yaml:"database,inline"`
    Metrics            config.MetricsConfig    `yaml:"metrics,inline"`
    
    // Custom fields
    APIKey    string `env:"API_KEY" yaml:"api_key" required:"true"`
    Debug     bool   `env:"DEBUG" yaml:"debug" default:"false"`
    Features  []string `env:"FEATURES" yaml:"features"` // comma-separated in env
}

func main() {
    var cfg ServiceConfig
    err := config.GetConfig(&cfg, "config.yaml", true) // true = fallback to env if file missing
    if err != nil {
        panic(err)
    }
    
    // Use the config
    server := &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.Http.Port),
        ReadTimeout:  cfg.Http.ReadTimeout(),
        WriteTimeout: cfg.Http.WriteTimeout(),
    }
    
    fmt.Printf("Starting server on port %d with log level %s\n", cfg.Http.Port, cfg.LogLevel)
    server.ListenAndServe()
}
```

### YAML File Example (config.yaml)
```yaml
log_level: info
http_port: 8080
database:
  url: postgres://user:pass@localhost:5432/myapp
metrics:
  expose_metrics: true
  enable_http_metrics: true
  metrics_port: 9090
api_key: your-secret-key
debug: false
features:
  - feature1
  - feature2
```

### Environment Variables
All config fields can be overridden with environment variables:
```bash
export LOG_LEVEL=debug
export HTTP_PORT=3000
export DATABASE_URL=postgres://prod-user:pass@prod-host:5432/prod-db
export API_KEY=prod-secret
export DEBUG=true
export FEATURES=prod-feature1,prod-feature2
```

## Struct Tags

- **`env`**: Environment variable name
- **`yaml`**: YAML field name  
- **`default`**: Fallback value if not set
- **`required`**: Must be provided (unless default exists)

## Precedence

Configuration values are loaded in this order (later overrides earlier):
1. **Defaults** (from `default` tags)
2. **YAML file** values
3. **Environment variables** (highest priority)

## Custom Validation

Implement the `Validator` interface for custom validation:
```go
type MyConfig struct {
    Port int `env:"PORT" yaml:"port" default:"8080"`
}

func (c MyConfig) Validate() error {
    if c.Port < 1024 {
        return fmt.Errorf("port must be >= 1024, got %d", c.Port)
    }
    return nil
}
```

## Helper Methods

Config types include helper methods for common conversions:
```go
cfg := config.HttpServerConfig{ReadTimeoutSeconds: 30}
timeout := cfg.ReadTimeout() // Returns time.Duration(30 * time.Second)
```