package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	"github.com/lewisedginton/general_purpose_chatbot/internal/middleware"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	"github.com/lewisedginton/general_purpose_chatbot/internal/monitoring"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// Config holds application configuration
type Config struct {
	AnthropicAPIKey string
	ClaudeModel     string
	LogLevel        logger.Level
	LogFormat       string
	Port            int
	ServiceName     string
	HealthTimeout   time.Duration
	RequestTimeout  time.Duration
	DatabaseURL     string
	RedisURL        string
	AnthropicAPIURL string
}

func main() {
	// Load configuration
	config := loadConfig()

	// Initialize structured logger
	log := logger.NewLogger(logger.Config{
		Level:   config.LogLevel,
		Format:  config.LogFormat,
		Service: config.ServiceName,
	})

	log.Info("Starting General Purpose Chatbot",
		logger.StringField("version", getVersion()),
		logger.StringField("claude_model", config.ClaudeModel),
		logger.StringField("service_name", config.ServiceName))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle special health check command for Docker healthcheck
	if len(os.Args) > 1 && os.Args[1] == "health" {
		performHealthCheck(config.Port)
		return
	}

	// Set up graceful shutdown
	setupGracefulShutdown(cancel, log)

	// Initialize monitoring
	healthMonitor := monitoring.NewHealthMonitor(monitoring.Config{
		Logger:          log,
		AnthropicAPIURL: config.AnthropicAPIURL,
		DatabaseURL:     config.DatabaseURL,
		RedisURL:        config.RedisURL,
	})

	// Create Claude model instance
	claudeModel, err := anthropic.NewClaudeModel(config.AnthropicAPIKey, config.ClaudeModel)
	if err != nil {
		log.Error("Failed to create Claude model", logger.ErrorField(err))
		os.Exit(1)
	}

	log.Info("Claude model created successfully")

	// Create agent loader with Claude model
	agentLoader := agents.NewLoader(claudeModel)

	log.Info("Agent loader created successfully")

	// Configure the ADK launcher with enhanced middleware
	adkConfig := &launcher.Config{
		AgentLoader: agentLoader,
	}

	// Set up HTTP middleware for web mode
	if len(os.Args) > 1 && os.Args[1] == "web" {
		setupWebServerWithMonitoring(ctx, adkConfig, healthMonitor, log, config)
		return
	}

	// Create and execute the ADK launcher
	adkLauncher := full.NewLauncher()

	log.Info("Starting ADK launcher",
		logger.StringField("mode", strings.Join(os.Args[1:], " ")))

	// Execute the launcher with command line arguments
	if err := adkLauncher.Execute(ctx, adkConfig, os.Args[1:]); err != nil {
		log.Error("ADK launcher failed", 
			logger.ErrorField(err),
			logger.StringField("usage", adkLauncher.CommandLineSyntax()))
		os.Exit(1)
	}

	log.Info("ADK launcher completed successfully")
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	config := Config{
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:     getEnvWithDefault("CLAUDE_MODEL", "claude-3-5-sonnet-20241022"),
		LogFormat:       getEnvWithDefault("LOG_FORMAT", "json"),
		ServiceName:     getEnvWithDefault("SERVICE_NAME", "general-purpose-chatbot"),
		Port:            getEnvInt("PORT", 8080),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		AnthropicAPIURL: getEnvWithDefault("ANTHROPIC_API_URL", "https://api.anthropic.com/v1/messages"),
	}

	// Parse log level
	logLevelStr := getEnvWithDefault("LOG_LEVEL", "info")
	switch strings.ToLower(logLevelStr) {
	case "debug":
		config.LogLevel = logger.DebugLevel
	case "warn", "warning":
		config.LogLevel = logger.WarnLevel
	case "error":
		config.LogLevel = logger.ErrorLevel
	default:
		config.LogLevel = logger.InfoLevel
	}

	// Parse timeouts
	config.HealthTimeout = getEnvDuration("HEALTH_CHECK_TIMEOUT", 10*time.Second)
	config.RequestTimeout = getEnvDuration("REQUEST_TIMEOUT", 30*time.Second)

	// Validate required configuration
	if config.AnthropicAPIKey == "" {
		fmt.Fprintf(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	return config
}

// setupGracefulShutdown sets up signal handling for graceful shutdown
func setupGracefulShutdown(cancel context.CancelFunc, log logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Received shutdown signal", logger.StringField("signal", sig.String()))
		
		// Start graceful shutdown
		cancel()
		
		// Give processes time to shutdown gracefully, then force exit
		time.AfterFunc(30*time.Second, func() {
			log.Warn("Force exiting due to timeout")
			os.Exit(1)
		})
	}()
}

// setupWebServerWithMonitoring sets up the web server with monitoring endpoints
func setupWebServerWithMonitoring(ctx context.Context, adkConfig *launcher.Config, healthMonitor *monitoring.HealthMonitor, log logger.Logger, config Config) {
	// Create HTTP server with monitoring endpoints
	mux := http.NewServeMux()
	
	// Register health check endpoints
	healthMonitor.RegisterHandlers(mux)
	
	// Create recovery middleware configuration
	recoveryConfig := middleware.DefaultRecoveryConfig()
	recoveryConfig.Logger = log
	
	// Chain middleware: Recovery -> Request Logging -> Error Handling -> Timeout
	handler := middleware.ChainMiddleware(
		middleware.Recovery(recoveryConfig),
		middleware.RequestLogging(log),
		middleware.ErrorHandler(recoveryConfig),
		middleware.TimeoutHandler(config.RequestTimeout, recoveryConfig),
	)(mux)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      handler,
		ReadTimeout:  config.RequestTimeout,
		WriteTimeout: config.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("Starting HTTP server",
			logger.IntField("port", config.Port),
			logger.StringField("timeout", config.RequestTimeout.String()))
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", logger.ErrorField(err))
		}
	}()

	// Start ADK launcher in goroutine
	go func() {
		adkLauncher := full.NewLauncher()
		if err := adkLauncher.Execute(ctx, adkConfig, []string{"web"}); err != nil {
			log.Error("ADK launcher failed in web mode", logger.ErrorField(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	
	log.Info("Shutting down HTTP server")
	
	// Mark service as not ready
	healthMonitor.ShutdownCheck()
	
	// Shutdown server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", logger.ErrorField(err))
	} else {
		log.Info("HTTP server shutdown completed")
	}
}

// performHealthCheck performs a health check for Docker healthcheck
func performHealthCheck(port int) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health/live", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check failed with status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("Health check passed")
}

// Helper functions for environment variable parsing
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := time.ParseDuration(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

// getVersion returns the application version
func getVersion() string {
	// This would typically be injected at build time via -ldflags
	if version := os.Getenv("VERSION"); version != "" {
		return version
	}
	return "dev"
}