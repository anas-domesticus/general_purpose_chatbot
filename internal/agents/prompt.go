package agents

import (
	"os"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// loadInstructionFile loads agent instructions from a file in the current working directory
func loadInstructionFile(filename string, log logger.Logger) string {
	// Try to load from the file in current working directory
	content, err := os.ReadFile(filename)
	if err != nil {
		// If file doesn't exist, return default instructions
		log.Warn("Could not load instruction file, using default instructions",
			logger.StringField("filename", filename),
			logger.ErrorField(err))
		return getDefaultInstructions()
	}

	log.Info("Loaded system instructions", logger.StringField("filename", filename))
	return string(content)
}

// getDefaultInstructions returns fallback instructions if system.md is not found
func getDefaultInstructions() string {
	return `You are a helpful AI assistant.

You can help with:
- General questions and conversation
- Technical discussions and explanations  
- Code analysis and programming help
- Creative writing and brainstorming
- Problem solving and reasoning

Be concise, helpful, and professional in your responses.
Ask clarifying questions when you need more context.

Note: No system.md file found. Create a system.md file in the current directory to customise these instructions.`
}
