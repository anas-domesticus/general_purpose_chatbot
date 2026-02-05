package session_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// SessionInfo represents metadata about a chat session
type SessionInfo struct {
	SessionID  string    `json:"session_id"` // e.g., "session-abc123..."
	Connector  string    `json:"connector"`  // "slack" or "telegram"
	UserID     string    `json:"user_id"`    // Platform-specific user ID
	ChannelID  string    `json:"channel_id"` // Channel/Chat ID
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

// Config holds configuration for the session manager
type Config struct {
	MetadataFile string                       // Path to metadata JSON file (relative to FileProvider root)
	FileProvider storage_manager.FileProvider // File provider for persistence (used for both metadata and session data)
	Logger       logger.Logger
}

// metadataStore represents the structure of the metadata JSON file
type metadataStore struct {
	// connector -> userID -> []SessionInfo
	Sessions map[string]map[string][]SessionInfo `json:"sessions"`
}
