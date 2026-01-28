package agents

import (
	"log"
	"os"
	"path/filepath"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
)

// NewLoader creates an agent loader with the provided model
func NewLoader(llmModel model.LLM) agent.Loader {
	// Create the Slack agent
	slackAgent, err := NewSlackAgent(llmModel)
	if err != nil {
		log.Fatalf("Failed to create slack agent: %v", err)
	}

	// For now, we return a single agent loader
	// This can be extended to support multiple agents
	return agent.NewSingleLoader(slackAgent)
}

// loadInstructionFile loads agent instructions from a file
func loadInstructionFile(filePath string) string {
	// Try to load from the config file
	content, err := os.ReadFile(filePath)
	if err != nil {
		// If file doesn't exist, return default instructions
		log.Printf("Warning: Could not load instruction file %s: %v. Using default instructions.", filePath, err)
		return getDefaultInstructions()
	}

	return string(content)
}

// getDefaultInstructions returns fallback instructions if config file is not found
func getDefaultInstructions() string {
	return `You are a helpful AI assistant powered by Claude.

You can help with:
- General questions and conversation
- Technical discussions and explanations  
- Code analysis and programming help
- Creative writing and brainstorming
- Problem solving and reasoning

Be concise, helpful, and professional in your responses.
Ask clarifying questions when you need more context.`
}

// getConfigPath returns the absolute path to a config file
func getConfigPath(relativePath string) string {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not get working directory: %v", err)
		return relativePath
	}

	// Join with the relative path
	return filepath.Join(wd, relativePath)
}