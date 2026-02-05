// Package memory_service provides persistent memory storage for the ADK memory interface.
package memory_service

import (
	"time"
)

// MemoryData represents persisted memory for a session.
type MemoryData struct {
	SessionID string        `json:"session_id"`
	AppName   string        `json:"app_name"`
	UserID    string        `json:"user_id"`
	UpdatedAt time.Time     `json:"updated_at"`
	Entries   []MemoryEntry `json:"entries"`
}

// MemoryEntry represents a single memory entry with pre-computed words for search.
type MemoryEntry struct {
	Content   *ContentData `json:"content"`
	Author    string       `json:"author"`
	Timestamp time.Time    `json:"timestamp"`
	Words     []string     `json:"words"` // Pre-computed for search
}

// ContentData is a JSON-serializable version of genai.Content.
type ContentData struct {
	Role  string     `json:"role"`
	Parts []PartData `json:"parts"`
}

// PartData is a JSON-serializable version of genai.Part.
type PartData struct {
	Text string `json:"text,omitempty"`
}

// WordIndex maps words to session IDs for fast lookup.
type WordIndex struct {
	AppName   string              `json:"app_name"`
	UserID    string              `json:"user_id"`
	UpdatedAt time.Time           `json:"updated_at"`
	Words     map[string][]string `json:"words"` // word -> []sessionID
}
