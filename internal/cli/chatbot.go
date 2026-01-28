package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/middleware"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	"github.com/lewisedginton/general_purpose_chatbot/internal/monitoring"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// ChatbotCommand returns a command for chatbot operations
func ChatbotCommand() *cli.Command {
	return &cli.Command{
		Name:    "chatbot",
		Aliases: []string{"bot"},
		Usage:   "Chatbot operations",
		Subcommands: []*cli.Command{
			{
				Name:   "start",
				Usage:  "Start the chatbot service",
				Action: chatbotStartAction,
			},
			{
				Name:   "web",
				Usage:  "Start the chatbot in web mode with HTTP endpoints",
				Action: chatbotWebAction,
			},
			{
				Name:   "health",
				Usage:  "Perform health check",
				Action: chatbotHealthAction,
			},
		},
	}
}

func chatbotStartAction(ctx *cli.Context) error {
	log := getLogger(ctx)

	// Load configuration using standardized pattern
	cfg := &appconfig.AppConfig{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		log.Error("Failed to load configuration", logger.ErrorField(err))
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Log configuration (without sensitive data)
	cfg.LogConfig(log)

	log.Info("Starting General Purpose Chatbot",
		logger.StringField("version", cfg.Version),
		logger.StringField("claude_model", cfg.Anthropic.Model))

	// Create context for graceful shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up graceful shutdown
	setupGracefulShutdown(cancel, log)

	// Create Claude model instance
	claudeModel, err := anthropic.NewClaudeModel(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
	if err != nil {
		log.Error("Failed to create Claude model", logger.ErrorField(err))
		return fmt.Errorf("failed to create Claude model: %w", err)
	}

	log.Info("Claude model created successfully")

	// Create agent loader with Claude model and MCP configuration
	agentLoader := agents.NewLoader(claudeModel, cfg.MCP)
	log.Info("Agent loader created successfully")

	// Configure the ADK launcher
	adkConfig := &launcher.Config{
		AgentLoader: agentLoader,
	}

	// Create and execute the ADK launcher
	adkLauncher := full.NewLauncher()

	log.Info("Starting ADK launcher")

	// Execute the launcher
	if err := adkLauncher.Execute(shutdownCtx, adkConfig, []string{}); err != nil {
		log.Error("ADK launcher failed", logger.ErrorField(err))
		return fmt.Errorf("ADK launcher failed: %w", err)
	}

	log.Info("Chatbot completed successfully")
	return nil
}

func chatbotWebAction(ctx *cli.Context) error {
	log := getLogger(ctx)

	// Load configuration
	cfg := &appconfig.AppConfig{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		log.Error("Failed to load configuration", logger.ErrorField(err))
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg.LogConfig(log)

	// Create context for graceful shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up graceful shutdown
	setupGracefulShutdown(cancel, log)

	// Initialize monitoring
	healthMonitor := monitoring.NewHealthMonitor(monitoring.Config{
		Logger:          log,
		AnthropicAPIURL: cfg.Anthropic.APIBaseURL,
		DatabaseURL:     cfg.Database.URL,
		RedisURL:        cfg.Redis.URL,
	})

	// Create Claude model instance
	claudeModel, err := anthropic.NewClaudeModel(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
	if err != nil {
		log.Error("Failed to create Claude model", logger.ErrorField(err))
		return fmt.Errorf("failed to create Claude model: %w", err)
	}

	// Create agent loader with MCP configuration
	agentLoader := agents.NewLoader(claudeModel, cfg.MCP)

	// Configure the ADK launcher
	adkConfig := &launcher.Config{
		AgentLoader: agentLoader,
	}

	// Set up HTTP server with monitoring endpoints
	setupWebServerWithMonitoring(shutdownCtx, adkConfig, healthMonitor, log, cfg)

	return nil
}

func chatbotHealthAction(ctx *cli.Context) error {
	log := getLogger(ctx)

	// Load configuration
	cfg := &appconfig.AppConfig{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		log.Error("Failed to load configuration", logger.ErrorField(err))
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Perform health check
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health/live", cfg.Port))
	if err != nil {
		log.Error("Health check failed", logger.ErrorField(err))
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("Health check failed with status", logger.IntField("status_code", resp.StatusCode))
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	log.Info("Health check passed")
	fmt.Println("✅ Health check passed")
	return nil
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
func setupWebServerWithMonitoring(ctx context.Context, adkConfig *launcher.Config, healthMonitor *monitoring.HealthMonitor, log logger.Logger, cfg *appconfig.AppConfig) {
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
		middleware.TimeoutHandler(cfg.RequestTimeout, recoveryConfig),
	)(mux)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info("Starting HTTP server",
			logger.IntField("port", cfg.Port),
			logger.StringField("timeout", cfg.RequestTimeout.String()))
		
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