package agents

import (
	"log"
	"os"
)

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
	return `You are a helpful AI assistant.

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
