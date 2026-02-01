package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func main() {
	// Load configuration
	cfg := &appconfig.AppConfig{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	log := logger.NewLogger(logger.Config{
		Level:   cfg.GetLogLevel(),
		Format:  cfg.Logging.Format,
		Service: cfg.ServiceName,
	})

	cfg.LogConfig(log)

	log.Info("Starting Slack Chatbot",
		logger.StringField("version", cfg.Version),
		logger.StringField("claude_model", cfg.Anthropic.Model))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupGracefulShutdown(cancel, log)

	// Create Claude model instance
	claudeModel, err := anthropic.NewClaudeModel(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
	if err != nil {
		log.Error("Failed to create Claude model", logger.ErrorField(err))
		os.Exit(1)
	}

	// Verify Slack is configured
	if !cfg.Slack.Enabled() {
		log.Error("Slack is not configured. Set SLACK_BOT_TOKEN and SLACK_APP_TOKEN environment variables.")
		os.Exit(1)
	}

	// Create Slack agent
	slackAgent, err := agents.NewSlackAgent(claudeModel, cfg.MCP)
	if err != nil {
		log.Error("Failed to create Slack agent", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create executor
	exec, err := executor.NewExecutor(slackAgent)
	if err != nil {
		log.Error("Failed to create executor", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create Slack connector
	slackConnector, err := slack.NewConnector(slack.Config{
		BotToken: cfg.Slack.BotToken,
		AppToken: cfg.Slack.AppToken,
		Debug:    cfg.Slack.Debug,
	}, exec)
	if err != nil {
		log.Error("Failed to create Slack connector", logger.ErrorField(err))
		os.Exit(1)
	}

	// Start Slack connector
	log.Info("Starting Slack connector")
	if err := slackConnector.Start(ctx); err != nil {
		log.Error("Slack connector error", logger.ErrorField(err))
		os.Exit(1)
	}
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
