package session_manager

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/prefixed_uuid"
)

// Manager provides session tracking and lifecycle management
type Manager interface {
	// GetLatestSession returns the most recent session ID for a user+connector
	// Returns empty string if no sessions exist
	GetLatestSession(ctx context.Context, connector, userID string) (string, error)

	// GetOrCreateSession returns existing latest session or creates new one
	GetOrCreateSession(ctx context.Context, connector, userID, channelID string) (string, error)

	// CreateNewSession always creates a new session (for /new command)
	CreateNewSession(ctx context.Context, connector, userID, channelID string) (string, error)

	// UpdateLastActive updates the last active timestamp for a session
	UpdateLastActive(ctx context.Context, sessionID string) error

	// ListUserSessions returns all sessions for a user+connector
	ListUserSessions(ctx context.Context, connector, userID string) ([]SessionInfo, error)
}

// sessionManager implements the Manager interface
type sessionManager struct {
	config    Config
	mutex     sync.RWMutex
	index     map[string]map[string][]SessionInfo // connector -> userID -> []sessions
	fileMutex sync.Mutex
}

// New creates a new session manager instance
func New(config Config) (Manager, error) {
	if config.MetadataFile == "" {
		return nil, fmt.Errorf("metadata file path is required")
	}
	if config.FileProvider == nil {
		return nil, fmt.Errorf("file provider is required")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	sm := &sessionManager{
		config: config,
		index:  make(map[string]map[string][]SessionInfo),
	}

	// Load existing metadata
	if err := sm.loadMetadata(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	return sm, nil
}

// GetLatestSession returns the most recent session ID for a user+connector
func (sm *sessionManager) GetLatestSession(ctx context.Context, connector, userID string) (string, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	users, ok := sm.index[connector]
	if !ok {
		return "", nil
	}

	sessions, ok := users[userID]
	if !ok || len(sessions) == 0 {
		return "", nil
	}

	// Find session with most recent LastActive
	latest := sessions[0]
	for _, s := range sessions[1:] {
		if s.LastActive.After(latest.LastActive) {
			latest = s
		}
	}

	return latest.SessionID, nil
}

// GetOrCreateSession returns existing latest session or creates new one
func (sm *sessionManager) GetOrCreateSession(ctx context.Context, connector, userID, channelID string) (string, error) {
	// Try to get existing session first
	sessionID, err := sm.GetLatestSession(ctx, connector, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get latest session: %w", err)
	}

	if sessionID != "" {
		// Update last active time
		if err := sm.UpdateLastActive(ctx, sessionID); err != nil {
			sm.config.Logger.Warn("Failed to update last active time",
				logger.StringField("session_id", sessionID),
				logger.ErrorField(err))
		}
		return sessionID, nil
	}

	// No existing session, create new one
	return sm.CreateNewSession(ctx, connector, userID, channelID)
}

// CreateNewSession always creates a new session
func (sm *sessionManager) CreateNewSession(ctx context.Context, connector, userID, channelID string) (string, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Generate new session ID
	sessionID := prefixed_uuid.New("session").String()

	// Create session info
	now := time.Now()
	info := SessionInfo{
		SessionID:  sessionID,
		Connector:  connector,
		UserID:     userID,
		ChannelID:  channelID,
		CreatedAt:  now,
		LastActive: now,
	}

	// Ensure connector map exists
	if sm.index[connector] == nil {
		sm.index[connector] = make(map[string][]SessionInfo)
	}

	// Add to index
	sm.index[connector][userID] = append(sm.index[connector][userID], info)

	// Persist to file
	if err := sm.saveMetadata(ctx); err != nil {
		sm.config.Logger.Error("Failed to save metadata after creating session",
			logger.StringField("session_id", sessionID),
			logger.ErrorField(err))
		// Don't return error - session is created in memory
	}

	sm.config.Logger.Info("Created new session",
		logger.StringField("session_id", sessionID),
		logger.StringField("connector", connector),
		logger.StringField("user_id", userID))

	return sessionID, nil
}

// UpdateLastActive updates the last active timestamp for a session
func (sm *sessionManager) UpdateLastActive(ctx context.Context, sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Find session in index
	found := false
	for connector, users := range sm.index {
		for userID, sessions := range users {
			for i, s := range sessions {
				if s.SessionID == sessionID {
					// Update timestamp
					sm.index[connector][userID][i].LastActive = time.Now()
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Persist to file
	if err := sm.saveMetadata(ctx); err != nil {
		sm.config.Logger.Warn("Failed to save metadata after updating last active",
			logger.StringField("session_id", sessionID),
			logger.ErrorField(err))
		// Don't return error - update is in memory
	}

	return nil
}

// ListUserSessions returns all sessions for a user+connector, sorted by LastActive descending
func (sm *sessionManager) ListUserSessions(ctx context.Context, connector, userID string) ([]SessionInfo, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	users, ok := sm.index[connector]
	if !ok {
		return []SessionInfo{}, nil
	}

	sessions, ok := users[userID]
	if !ok {
		return []SessionInfo{}, nil
	}

	// Create a copy to avoid returning internal state
	result := make([]SessionInfo, len(sessions))
	copy(result, sessions)

	// Sort by LastActive descending (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastActive.After(result[j].LastActive)
	})

	return result, nil
}
