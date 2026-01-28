# Utils Package

The utils package provides common utility functions for Go projects, including gRPC server lifecycle management with graceful shutdown, error channel aggregation, server configuration patterns, and memory-safe channel cleanup.

## Features

- **gRPC Server Lifecycle Management**: Start gRPC servers with graceful shutdown patterns
- **Error Channel Aggregation**: Merge multiple error channels from concurrent operations
- **Server Configuration**: Structured server configuration with sensible defaults
- **Memory-Safe Channel Cleanup**: Prevent goroutine leaks with proper channel management
- **Generic Utilities**: Type-safe pointer utilities using Go generics

## Usage Examples

### gRPC Server Lifecycle

The `grpc_helpers` module provides utilities for managing gRPC server lifecycle with graceful shutdown.

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "google.golang.org/grpc"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/utils"
)

func main() {
    // Create logger
    log := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "my-service",
    })

    // Create gRPC server with your services
    server := grpc.NewServer()
    // Register your gRPC services here
    // pb.RegisterMyServiceServer(server, &myServiceImpl{})

    // Start the server
    errChan, forceCloser, gracefulCloser, err := utils.Listen(server, 8000, log)
    if err != nil {
        log.Error("Failed to start server", logger.ErrorField(err))
        return
    }

    // Set up graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    select {
    case err := <-errChan:
        if err != nil {
            log.Error("Server error", logger.ErrorField(err))
        }
    case sig := <-sigChan:
        log.Info("Received shutdown signal", logger.StringField("signal", sig.String()))
        gracefulCloser() // Use graceful shutdown
        // Use forceCloser() for immediate shutdown if needed
    }
}
```

### Error Channel Merging

Aggregate errors from multiple concurrent operations:

```go
package main

import (
    "errors"
    "fmt"
    "time"
    
    "github.com/lewisedginton/go_project_boilerplate/pkg/utils"
)

func main() {
    // Create multiple error channels for different operations
    dbErrors := make(chan error, 1)
    apiErrors := make(chan error, 1)
    cacheErrors := make(chan error, 1)

    // Start concurrent operations
    go databaseOperation(dbErrors)
    go apiOperation(apiErrors)
    go cacheOperation(cacheErrors)

    // Merge all error channels
    merged := utils.MergeErrorChans(dbErrors, apiErrors, cacheErrors)

    // Handle all errors from a single channel
    for err := range merged {
        fmt.Printf("Operation failed: %v\n", err)
        // Handle error appropriately (log, retry, etc.)
    }
}

func databaseOperation(errChan chan error) {
    defer close(errChan)
    // Simulate work
    time.Sleep(100 * time.Millisecond)
    errChan <- errors.New("database connection timeout")
}

func apiOperation(errChan chan error) {
    defer close(errChan)
    // Simulate work
    time.Sleep(200 * time.Millisecond)
    // No error in this example
}

func cacheOperation(errChan chan error) {
    defer close(errChan)
    // Simulate work
    time.Sleep(150 * time.Millisecond)
    errChan <- errors.New("cache miss")
}
```

### Server Configuration with Defaults

Use the server configuration pattern for consistent service setup:

```go
package main

import (
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
    "github.com/lewisedginton/go_project_boilerplate/pkg/utils"
)

func main() {
    // Create logger
    log := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "my-service",
    })

    // Create metrics (optional)
    m := metrics.NewMetrics()

    // Server with defaults (gRPC: 8000, HTTP: 8080)
    server1 := utils.NewServer(log, nil)

    // Server with custom configuration
    config := &utils.ServerConfig{
        GrpcListenPort: 9000,
        HttpListenPort: 9001,
        Metrics:        m,
    }
    server2 := utils.NewServer(log, config)

    log.Info("Server1 ports", 
        logger.IntField("grpc", server1.GrpcListenPort),
        logger.IntField("http", server1.HttpListenPort))
    
    log.Info("Server2 ports",
        logger.IntField("grpc", server2.GrpcListenPort), 
        logger.IntField("http", server2.HttpListenPort))
}
```

### Complete Server Example

Here's a complete example showing all utilities working together:

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "google.golang.org/grpc"
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/metrics"
    "github.com/lewisedginton/go_project_boilerplate/pkg/utils"
)

func main() {
    // Create logger
    log := logger.NewLogger(logger.Config{
        Level:   logger.InfoLevel,
        Format:  "json",
        Service: "complete-example",
    })

    // Create server configuration
    m := metrics.NewMetrics()
    config := &utils.ServerConfig{
        GrpcListenPort: 8000,
        HttpListenPort: 8080,
        Metrics:        m,
    }
    serverConfig := utils.NewServer(log, config)

    // Start gRPC server
    grpcServer := grpc.NewServer()
    grpcErrChan, grpcForceCloser, grpcGracefulCloser, err := utils.Listen(
        grpcServer, serverConfig.GrpcListenPort, log)
    if err != nil {
        log.Error("Failed to start gRPC server", logger.ErrorField(err))
        return
    }

    // Start HTTP server
    httpErrChan := make(chan error, 1)
    httpServer := &http.Server{
        Addr:    fmt.Sprintf(":%d", serverConfig.HttpListenPort),
        Handler: http.DefaultServeMux,
    }
    go func() {
        log.Info("Starting HTTP server", 
            logger.IntField("port", serverConfig.HttpListenPort))
        httpErrChan <- httpServer.ListenAndServe()
    }()

    // Additional service error channels
    serviceErrChan := make(chan error, 1)
    go someBackgroundService(serviceErrChan, log)

    // Merge all error channels
    allErrors := utils.MergeErrorChans(grpcErrChan, httpErrChan, serviceErrChan)

    // Set up graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    select {
    case err := <-allErrors:
        if err != nil {
            log.Error("Service error", logger.ErrorField(err))
        }
    case sig := <-sigChan:
        log.Info("Received shutdown signal", 
            logger.StringField("signal", sig.String()))
        
        // Graceful shutdown
        grpcGracefulCloser()
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        if err := httpServer.Shutdown(ctx); err != nil {
            log.Error("HTTP server shutdown error", logger.ErrorField(err))
        }
    }
}

func someBackgroundService(errChan chan error, log logger.Logger) {
    defer close(errChan)
    
    // Simulate background work
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            log.Debug("Background service tick")
            // Do work, send error if something fails
            // errChan <- errors.New("background service error")
        }
    }
}
```

### Generic Pointer Utility

Convert values to pointers safely:

```go
package main

import (
    "fmt"
    "github.com/lewisedginton/go_project_boilerplate/pkg/utils"
)

func main() {
    // String to pointer
    str := "hello"
    strPtr := utils.ToPtr(str)
    fmt.Printf("String: %s, Pointer: %s\n", str, *strPtr)

    // Int to pointer
    num := 42
    numPtr := utils.ToPtr(num)
    fmt.Printf("Int: %d, Pointer: %d\n", num, *numPtr)

    // Struct to pointer
    type Person struct {
        Name string
        Age  int
    }
    person := Person{Name: "Alice", Age: 30}
    personPtr := utils.ToPtr(person)
    fmt.Printf("Person: %+v, Pointer: %+v\n", person, *personPtr)
}
```

## Best Practices

### Graceful Shutdown

Always use graceful shutdown for production services:

1. **Use `gracefulCloser()`** instead of `forceCloser()` when possible
2. **Set timeouts** for shutdown operations to prevent hanging
3. **Handle multiple services** by merging their error channels
4. **Clean up resources** in shutdown handlers

### Error Handling

1. **Merge error channels** from multiple concurrent operations
2. **Always close channels** to prevent goroutine leaks
3. **Use buffered channels** for error channels to prevent blocking
4. **Log errors appropriately** with context

### Configuration

1. **Use sensible defaults** for server configuration
2. **Make metrics optional** but available
3. **Validate configuration** before using it
4. **Document default values** clearly

## Testing

The package includes comprehensive tests for all utilities. Run tests with:

```bash
go test ./pkg/utils/...
```

All utilities are designed to be memory-safe and prevent goroutine leaks through proper channel management and lifecycle handling.