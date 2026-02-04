package skills_manager

import (
	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// Skill represents a skill with its content
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Text        string `json:"text"`
}

// Config holds configuration for the skills manager
type Config struct {
	FileProvider storage_manager.FileProvider // File provider for persistence
	Logger       logger.Logger
}
