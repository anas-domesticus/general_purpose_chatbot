package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// SlackCommand returns a command for Slack operations
func SlackCommand() *cli.Command {
	return &cli.Command{
		Name:    "slack",
		Aliases: []string{"sl"},
		Usage:   "Slack bot operations",
		Subcommands: []*cli.Command{
			{
				Name:   "start",
				Usage:  "Start the Slack bot",
				Action: slackStartAction,
			},
		},
	}
}

func slackStartAction(ctx *cli.Context) error {
	log := getLogger(ctx)

	// Load configuration using standardized pattern
	cfg := &appconfig.AppConfig{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		log.Error("Failed to load configuration", logger.ErrorField(err))
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Update service name for Slack bot
	slackLogger := logger.NewLogger(logger.Config{
		Level:   cfg.GetLogLevel(),
		Format:  cfg.Logging.Format,
		Service: cfg.ServiceName + "-slack-bot",
	})

	slackLogger.Info("Starting Slack Bot")

	// Get required environment variables for Slack
	slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		slackLogger.Error("SLACK_BOT_TOKEN environment variable is required (format: xoxb-*)")
		return fmt.Errorf("SLACK_BOT_TOKEN environment variable is required")
	}

	slackAppToken := os.Getenv("SLACK_APP_TOKEN")
	if slackAppToken == "" {
		slackLogger.Error("SLACK_APP_TOKEN environment variable is required (format: xapp-*)")
		return fmt.Errorf("SLACK_APP_TOKEN environment variable is required")
	}

	// Optional: ADK server URL (defaults to local)
	adkBaseURL := os.Getenv("ADK_BASE_URL")
	if adkBaseURL == "" {
		adkBaseURL = "http://localhost:8000"
	}

	// Optional: Agent name (defaults to slack_assistant)
	agentName := os.Getenv("ADK_AGENT_NAME")
	if agentName == "" {
		agentName = "slack_assistant"
	}

	slackLogger.Info("Slack bot configuration loaded",
		logger.StringField("adk_base_url", adkBaseURL),
		logger.StringField("agent_name", agentName),
		logger.StringField("bot_token_prefix", slackBotToken[:12]+"..."),
		logger.StringField("app_token_prefix", slackAppToken[:12]+"..."))

	// Create Slack connector configuration
	slackConfig := slack.Config{
		BotToken:   slackBotToken,
		AppToken:   slackAppToken,
		ADKBaseURL: adkBaseURL,
		AgentName:  agentName,
	}

	// Create the Slack connector
	connector, err := slack.NewConnector(slackConfig)
	if err != nil {
		slackLogger.Error("Failed to create Slack connector", logger.ErrorField(err))
		return fmt.Errorf("failed to create Slack connector: %w", err)
	}

	// Test connectivity
	slackLogger.Info("Testing Slack connectivity")
	botInfo, err := connector.GetBotInfo()
	if err != nil {
		slackLogger.Error("Failed to connect to Slack API", logger.ErrorField(err))
		return fmt.Errorf("failed to connect to Slack API: %w", err)
	}
	slackLogger.Info("Connected to Slack successfully",
		logger.StringField("bot_name", botInfo.Name),
		logger.StringField("bot_id", botInfo.ID))

	// Set up context for graceful shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the connector in a goroutine
	errChan := make(chan error, 1)
	go func() {
		slackLogger.Info("Starting Socket Mode connection")
		err := connector.Start(shutdownCtx)
		if err != nil {
			errChan <- err
		}
	}()

	slackLogger.Info("Slack bot is running! Press Ctrl+C to stop")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		slackLogger.Info("Received shutdown signal", logger.StringField("signal", sig.String()))
		cancel()
	case err := <-errChan:
		slackLogger.Error("Connector error", logger.ErrorField(err))
		cancel()
		return fmt.Errorf("connector error: %w", err)
	}

	// Graceful shutdown
	slackLogger.Info("Stopping Slack connector")
	if err := connector.Stop(); err != nil {
		slackLogger.Error("Error stopping connector", logger.ErrorField(err))
		return fmt.Errorf("error stopping connector: %w", err)
	}

	slackLogger.Info("Slack bot stopped")
	return nil
}