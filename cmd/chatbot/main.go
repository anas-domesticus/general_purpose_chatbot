package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/telegram"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/session"
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

	log.Info("Starting Multi-Platform Chatbot",
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

	// Create generic chat agent factory (shared across all platforms)
	// Note: nil formatting provider for now - will be platform-specific in the future
	chatAgentFactory, err := agents.NewChatAgent(claudeModel, cfg.MCP, agents.AgentConfig{
		Name:        "chat_assistant",
		Platform:    "Multi-Platform",
		Description: "Claude-powered assistant with MCP capabilities",
	})
	if err != nil {
		log.Error("Failed to create chat agent factory", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create in-memory session service
	// TODO: Make this configurable
	sessionService := session.InMemoryService()

	// Create executor with agent factory (shared across all platforms)
	// Note: nil formatting provider for now - will be platform-specific in the future
	exec, err := executor.NewExecutor(chatAgentFactory, "chatbot", sessionService)
	if err != nil {
		log.Error("Failed to create executor", logger.ErrorField(err))
		os.Exit(1)
	}

	// Detect and start enabled connectors
	var wg sync.WaitGroup
	enabledCount := 0

	// Start Slack connector if configured
	if cfg.Slack.Enabled() {
		enabledCount++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startSlackConnector(ctx, cfg, exec, log); err != nil {
				log.Error("Slack connector failed", logger.ErrorField(err))
				cancel() // Trigger shutdown on error
			}
		}()
	} else {
		log.Info("Slack connector disabled (missing SLACK_BOT_TOKEN or SLACK_APP_TOKEN)")
	}

	// Start Telegram connector if configured
	if cfg.Telegram.Enabled() {
		enabledCount++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startTelegramConnector(ctx, cfg, exec, log); err != nil {
				log.Error("Telegram connector failed", logger.ErrorField(err))
				cancel() // Trigger shutdown on error
			}
		}()
	} else {
		log.Info("Telegram connector disabled (missing TELEGRAM_BOT_TOKEN)")
	}

	// Verify at least one connector is enabled
	if enabledCount == 0 {
		log.Error("No connectors configured. Please set environment variables for at least one platform (Slack or Telegram)")
		os.Exit(1)
	}

	log.Info("All enabled connectors started", logger.IntField("count", enabledCount))

	// Wait for all connectors to finish
	wg.Wait()
	log.Info("All connectors stopped")
}

// startSlackConnector initializes and starts the Slack connector
func startSlackConnector(ctx context.Context, cfg *appconfig.AppConfig, exec *executor.Executor, log logger.Logger) error {
	log.Info("Starting Slack connector")

	// Create Slack connector
	slackConnector, err := slack.NewConnector(slack.Config{
		BotToken: cfg.Slack.BotToken,
		AppToken: cfg.Slack.AppToken,
		Debug:    cfg.Slack.Debug,
	}, exec)
	if err != nil {
		return fmt.Errorf("failed to create Slack connector: %w", err)
	}

	// Start connector (blocks until context is cancelled)
	if err := slackConnector.Start(ctx); err != nil {
		return fmt.Errorf("Slack connector error: %w", err)
	}

	return nil
}

// startTelegramConnector initializes and starts the Telegram connector
func startTelegramConnector(ctx context.Context, cfg *appconfig.AppConfig, exec *executor.Executor, log logger.Logger) error {
	log.Info("Starting Telegram connector")

	// Create Telegram connector
	telegramConnector, err := telegram.NewConnector(telegram.Config{
		BotToken: cfg.Telegram.BotToken,
		Debug:    cfg.Telegram.Debug,
	}, exec)
	if err != nil {
		return fmt.Errorf("failed to create Telegram connector: %w", err)
	}

	// Get and log bot info
	botInfo, err := telegramConnector.GetBotInfo(ctx)
	if err != nil {
		log.Warn("Failed to get Telegram bot info", logger.ErrorField(err))
	} else {
		log.Info("Telegram bot connected",
			logger.StringField("bot_username", botInfo.Username),
			logger.StringField("bot_first_name", botInfo.FirstName))
	}

	// Start connector (blocks until context is cancelled)
	if err := telegramConnector.Start(ctx); err != nil {
		return fmt.Errorf("Telegram connector error: %w", err)
	}

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
