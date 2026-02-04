package session_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// loadMetadata loads session metadata from the JSON file
func (sm *sessionManager) loadMetadata(ctx context.Context) error {
	sm.fileMutex.Lock()
	defer sm.fileMutex.Unlock()

	// Check if file exists
	exists, err := sm.config.FileProvider.Exists(ctx, sm.config.MetadataFile)
	if err != nil {
		return fmt.Errorf("failed to check metadata file existence: %w", err)
	}

	if !exists {
		// Initialize with empty structure
		sm.config.Logger.Info("Metadata file does not exist, starting with empty index")
		sm.index = make(map[string]map[string][]SessionInfo)
		return nil
	}

	// Read file
	data, err := sm.config.FileProvider.Read(ctx, sm.config.MetadataFile)
	if err != nil {
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Parse JSON
	var store metadataStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	// Load into index
	sm.index = store.Sessions
	if sm.index == nil {
		sm.index = make(map[string]map[string][]SessionInfo)
	}

	sm.config.Logger.Info("Loaded session metadata", logger.StringField("file", sm.config.MetadataFile))
	return nil
}

// saveMetadata persists session metadata to the JSON file
func (sm *sessionManager) saveMetadata(ctx context.Context) error {
	sm.fileMutex.Lock()
	defer sm.fileMutex.Unlock()

	// Create metadata store structure
	store := metadataStore{
		Sessions: sm.index,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(sm.config.MetadataFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Write file
	if err := sm.config.FileProvider.Write(ctx, sm.config.MetadataFile, data); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}
