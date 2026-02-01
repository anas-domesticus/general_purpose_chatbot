package agents

import (
	"log"
	"os"

	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
)

// NewLoader creates an agent loader with the provided model and MCP configuration
func NewLoader(llmModel model.LLM, mcpConfig config.MCPConfig, platform string) agent.Loader {
	// Create agent configuration based on platform
	var agentConfig AgentConfig
	switch platform {
	case "slack":
		agentConfig = AgentConfig{
			Name:        "chat_assistant",
			Platform:    "Slack",
			Description: "Claude-powered assistant for Slack workspace interactions with MCP capabilities",
		}
	case "telegram":
		agentConfig = AgentConfig{
			Name:        "chat_assistant",
			Platform:    "Telegram",
			Description: "Claude-powered assistant for Telegram chat interactions with MCP capabilities",
		}
	default:
		log.Fatalf("Unsupported platform: %s", platform)
	}

	// Create the chat agent with MCP configuration
	chatAgent, err := NewChatAgent(llmModel, mcpConfig, agentConfig)
	if err != nil {
		log.Fatalf("Failed to create %s agent: %v", platform, err)
	}

	return agent.NewSingleLoader(chatAgent)
}

// loadInstructionFile loads agent instructions from a file in the current working directory
func loadInstructionFile(filename string) string {
	// Try to load from the file in current working directory
	content, err := os.ReadFile(filename)
	if err != nil {
		// If file doesn't exist, return default instructions
		log.Printf("Warning: Could not load instruction file %s: %v. Using default instructions.", filename, err)
		return getDefaultInstructions()
	}

	log.Printf("Loaded system instructions from %s", filename)
	return string(content)
}

// getDefaultInstructions returns fallback instructions if system.md is not found
func getDefaultInstructions() string {
	return `You are a helpful AI assistant powered by Claude.

You can help with:
- General questions and conversation
- Technical discussions and explanations  
- Code analysis and programming help
- Creative writing and brainstorming
- Problem solving and reasoning

Be concise, helpful, and professional in your responses.
Ask clarifying questions when you need more context.

Note: No system.md file found. Create a system.md file in the current directory to customize these instructions.`
}
