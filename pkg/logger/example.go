package logger

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
)

/*
Example usage with config integration:

package main

import (
	"fmt"
	"net/http"
	
	"github.com/go-chi/chi/v5"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// ServiceConfig combines config types with logger integration
type ServiceConfig struct {
	config.CommonConfig `yaml:",inline"`          // log_level
	Http               config.HttpServerConfig `yaml:"http,inline"`     // http_port, timeouts, etc.
	Database           config.DatabaseConfig   `yaml:"database,inline"` // url
	
	Service string `env:"SERVICE_NAME" yaml:"service" default:"my-service"`
}

func main() {
	// Load configuration
	var cfg ServiceConfig
	if err := config.GetConfig(&cfg, "config.yaml", true); err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}
	
	// Create logger from config
	logger := logger.NewLogger(logger.Config{
		Level:   logger.ParseLevel(cfg.LogLevel), // From CommonConfig
		Format:  "json",
		Service: cfg.Service,
	})
	
	logger.Info("Service starting",
		logger.StringField("service", cfg.Service),
		logger.IntField("http_port", cfg.Http.Port),
	)
	
	// Create HTTP server with logger middleware
	r := chi.NewRouter()
	r.Use(logger.HTTPMiddleware) // Automatic request/response logging + correlation ID
	
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// Get logger with correlation ID from request context
		requestLogger := logger.GetLoggerFromContext(r.Context(), logger)
		
		requestLogger.Info("Health check requested")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})
	
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Logger automatically has correlation ID from middleware
		requestLogger := logger.GetLoggerFromContext(r.Context(), logger)
		
		userID := chi.URLParam(r, "id")
		requestLogger.Info("Getting user",
			logger.StringField("user_id", userID),
		)
		
		// Simulate processing time
		time.Sleep(50 * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"id": "%s", "name": "User %s"}`, userID, userID)))
	})
	
	// Create HTTP server with config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Http.Port),
		Handler:      r,
		ReadTimeout:  cfg.Http.ReadTimeout(),
		WriteTimeout: cfg.Http.WriteTimeout(),
		IdleTimeout:  cfg.Http.IdleTimeout(),
	}
	
	logger.Info("HTTP server starting",
		logger.IntField("port", cfg.Http.Port),
		logger.DurationField("read_timeout", cfg.Http.ReadTimeout()),
		logger.DurationField("write_timeout", cfg.Http.WriteTimeout()),
	)
	
	if err := server.ListenAndServe(); err != nil {
		logger.Error("Server failed", logger.ErrorField(err))
	}
}

Example config.yaml:
---
log_level: info
service: user-api
http_port: 8080
read_timeout_seconds: 30
write_timeout_seconds: 30
database:
  url: postgres://user:pass@localhost:5432/users
*/

// ExampleHTTPServer shows a complete HTTP server setup with logger
func ExampleHTTPServer() {
	// This is a documentation example - not executable
	
	// Create logger
	logger := NewLogger(Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "example-api",
	})
	
	// Create router with logger middleware
	r := chi.NewRouter()
	r.Use(logger.HTTPMiddleware)
	
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Get logger with correlation ID from context
		requestLogger := GetLoggerFromContext(r.Context(), logger)
		
		userID := chi.URLParam(r, "id")
		requestLogger.Info("Processing user request",
			StringField("user_id", userID),
		)
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})
}

// ExampleGRPCServer shows a complete gRPC server setup with logger
func ExampleGRPCServer() {
	// This is a documentation example - not executable
	
	// Create logger
	logger := NewLogger(Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "example-grpc",
	})
	
	// Create gRPC server with logger interceptor
	server := grpc.NewServer(
		grpc.UnaryInterceptor(logger.GrpcRequestsInterceptor),
	)
	
	// Register your services here...
	// pb.RegisterUserServiceServer(server, userService)
	
	_ = server // Example only
}