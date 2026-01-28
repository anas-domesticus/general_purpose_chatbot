package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
)

func main() {
	log.Println("Starting Slack Bot...")

	// Get required environment variables
	slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		log.Fatal("SLACK_BOT_TOKEN environment variable is required (format: xoxb-*)")
	}

	slackAppToken := os.Getenv("SLACK_APP_TOKEN")
	if slackAppToken == "" {
		log.Fatal("SLACK_APP_TOKEN environment variable is required (format: xapp-*)")
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

	log.Printf("Configuration:")
	log.Printf("  ADK Base URL: %s", adkBaseURL)
	log.Printf("  Agent Name: %s", agentName)
	log.Printf("  Bot Token: %s...%s", slackBotToken[:12], slackBotToken[len(slackBotToken)-4:])
	log.Printf("  App Token: %s...%s", slackAppToken[:12], slackAppToken[len(slackAppToken)-4:])

	// Create Slack connector configuration
	config := slack.Config{
		BotToken:   slackBotToken,
		AppToken:   slackAppToken,
		ADKBaseURL: adkBaseURL,
		AgentName:  agentName,
	}

	// Create the Slack connector
	connector, err := slack.NewConnector(config)
	if err != nil {
		log.Fatalf("Failed to create Slack connector: %v", err)
	}

	// Test connectivity
	log.Println("Testing Slack connectivity...")
	botInfo, err := connector.GetBotInfo()
	if err != nil {
		log.Fatalf("Failed to connect to Slack API: %v", err)
	}
	log.Printf("Connected to Slack as: %s (ID: %s)", botInfo.Name, botInfo.ID)

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the connector in a goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Println("Starting Socket Mode connection...")
		err := connector.Start(ctx)
		if err != nil {
			errChan <- err
		}
	}()

	log.Println("Slack bot is running! Press Ctrl+C to stop.")
	log.Println("")
	log.Println("Bot Setup Instructions:")
	log.Println("1. Make sure your ADK server is running on " + adkBaseURL)
	log.Println("2. The bot will respond to:")
	log.Println("   - Direct messages (DMs)")
	log.Println("   - @mentions in channels")
	log.Println("3. Make sure the bot has the following OAuth scopes:")
	log.Println("   - app_mentions:read")
	log.Println("   - channels:history")
	log.Println("   - chat:write")
	log.Println("   - im:history")
	log.Println("   - im:read")
	log.Println("4. Enable Socket Mode in your Slack app settings")
	log.Println("")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %s. Shutting down gracefully...", sig)
		cancel()
	case err := <-errChan:
		log.Printf("Connector error: %v", err)
		cancel()
	}

	// Graceful shutdown
	log.Println("Stopping Slack connector...")
	if err := connector.Stop(); err != nil {
		log.Printf("Error stopping connector: %v", err)
	}

	log.Println("Slack bot stopped.")
}