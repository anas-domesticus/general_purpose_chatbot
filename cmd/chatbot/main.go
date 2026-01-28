package main

import (
	"context"
	"log"
	"os"

	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"

	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
)

func main() {
	ctx := context.Background()

	// Get required environment variables
	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Optional: Allow model override via environment variable
	modelName := os.Getenv("CLAUDE_MODEL")
	if modelName == "" {
		modelName = "claude-3-5-sonnet-20241022" // Default to Claude 3.5 Sonnet
	}

	log.Printf("Initializing Claude model: %s", modelName)

	// Create Claude model instance
	claudeModel, err := anthropic.NewClaudeModel(anthropicAPIKey, modelName)
	if err != nil {
		log.Fatalf("Failed to create Claude model: %v", err)
	}

	log.Println("Claude model created successfully")

	// Create agent loader with Claude model
	agentLoader := agents.NewLoader(claudeModel)

	log.Println("Agent loader created successfully")

	// Configure the ADK launcher
	config := &launcher.Config{
		AgentLoader: agentLoader,
	}

	// Create and execute the full launcher (includes web UI, CLI, etc.)
	adkLauncher := full.NewLauncher()

	log.Println("Starting ADK launcher...")
	log.Println("Available commands:")
	log.Println("  go run cmd/chatbot/main.go         - CLI mode")
	log.Println("  go run cmd/chatbot/main.go web     - Web UI mode")

	// Execute the launcher with command line arguments
	if err := adkLauncher.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("ADK launcher failed: %v\n\nUsage:\n%s", err, adkLauncher.CommandLineSyntax())
	}

	log.Println("ADK launcher completed successfully")
}