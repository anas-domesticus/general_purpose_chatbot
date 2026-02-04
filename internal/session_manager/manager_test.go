package session_manager

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (Manager, string) {
	t.Helper()

	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "sessions_metadata.json")

	fileProvider := storage_manager.NewLocalFileProvider(tmpDir)

	mgr, err := New(Config{
		MetadataFile: metadataFile,
		FileProvider: fileProvider,
		Logger:       logger.NewLogger(logger.Config{Level: logger.InfoLevel, Format: "text"}),
	})
	require.NoError(t, err)

	return mgr, metadataFile
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				MetadataFile: "/tmp/test.json",
				FileProvider: storage_manager.NewLocalFileProvider("/tmp"),
				Logger:       logger.NewLogger(logger.Config{Level: logger.InfoLevel, Format: "text"}),
			},
			expectError: false,
		},
		{
			name: "missing metadata file",
			config: Config{
				FileProvider: storage_manager.NewLocalFileProvider(""),
				Logger:       logger.NewLogger(logger.Config{Level: logger.InfoLevel, Format: "text"}),
			},
			expectError: true,
		},
		{
			name: "missing file provider",
			config: Config{
				MetadataFile: "/tmp/test.json",
				Logger:       logger.NewLogger(logger.Config{Level: logger.InfoLevel, Format: "text"}),
			},
			expectError: true,
		},
		{
			name: "missing logger",
			config: Config{
				MetadataFile: "/tmp/test.json",
				FileProvider: storage_manager.NewLocalFileProvider(""),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateNewSession(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	sessionID, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.Contains(t, sessionID, "session-")

	// Verify session is tracked
	sessions, err := mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, sessionID, sessions[0].SessionID)
	assert.Equal(t, "slack", sessions[0].Connector)
	assert.Equal(t, "U12345", sessions[0].UserID)
	assert.Equal(t, "C67890", sessions[0].ChannelID)
}

func TestGetLatestSession(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// No sessions initially
	sessionID, err := mgr.GetLatestSession(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Empty(t, sessionID)

	// Create first session
	session1, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	// Should return first session
	latest, err := mgr.GetLatestSession(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Equal(t, session1, latest)

	// Wait a moment to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create second session
	session2, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	// After creation, both sessions exist - one will be latest
	// We don't care which one initially since they have similar timestamps
	latest, err = mgr.GetLatestSession(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.NotEmpty(t, latest)

	// Update session2's last active time to make it unambiguously latest
	time.Sleep(10 * time.Millisecond)
	err = mgr.UpdateLastActive(ctx, session2)
	require.NoError(t, err)

	// Now should definitely return session2 (most recently active)
	latest, err = mgr.GetLatestSession(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Equal(t, session2, latest)

	// Update session1's last active time to make it the latest
	time.Sleep(10 * time.Millisecond)
	err = mgr.UpdateLastActive(ctx, session1)
	require.NoError(t, err)

	// Now should return session1 (most recently active)
	latest, err = mgr.GetLatestSession(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Equal(t, session1, latest)
}

func TestGetOrCreateSession(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// First call should create new session
	session1, err := mgr.GetOrCreateSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)
	assert.NotEmpty(t, session1)

	// Second call should return same session
	session2, err := mgr.GetOrCreateSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)
	assert.Equal(t, session1, session2)

	// Verify only one session exists
	sessions, err := mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestUpdateLastActive(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// Create session
	sessionID, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	// Get initial last active time
	sessions, err := mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	initialTime := sessions[0].LastActive

	// Wait to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Update last active
	err = mgr.UpdateLastActive(ctx, sessionID)
	require.NoError(t, err)

	// Verify timestamp updated
	sessions, err = mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.True(t, sessions[0].LastActive.After(initialTime))

	// Test with non-existent session
	err = mgr.UpdateLastActive(ctx, "session-nonexistent")
	assert.Error(t, err)
}

func TestListUserSessions(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// Empty list initially
	sessions, err := mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Empty(t, sessions)

	// Create multiple sessions
	session1, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	session2, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	session3, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	// List all sessions
	sessions, err = mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Len(t, sessions, 3)

	// Verify sorted by LastActive descending (most recent first)
	assert.Equal(t, session3, sessions[0].SessionID)
	assert.Equal(t, session2, sessions[1].SessionID)
	assert.Equal(t, session1, sessions[2].SessionID)
}

func TestMultipleConnectors(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// Create sessions for different connectors
	slackSession, err := mgr.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	telegramSession, err := mgr.CreateNewSession(ctx, "telegram", "1985720465", "1985720465")
	require.NoError(t, err)

	// Verify isolated by connector
	slackSessions, err := mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Len(t, slackSessions, 1)
	assert.Equal(t, slackSession, slackSessions[0].SessionID)

	telegramSessions, err := mgr.ListUserSessions(ctx, "telegram", "1985720465")
	require.NoError(t, err)
	assert.Len(t, telegramSessions, 1)
	assert.Equal(t, telegramSession, telegramSessions[0].SessionID)

	// Wrong connector returns empty
	emptySlack, err := mgr.ListUserSessions(ctx, "slack", "1985720465")
	require.NoError(t, err)
	assert.Empty(t, emptySlack)
}

func TestPersistenceAcrossRestarts(t *testing.T) {
	tmpDir := t.TempDir()
	metadataFile := filepath.Join(tmpDir, "sessions_metadata.json")

	fileProvider := storage_manager.NewLocalFileProvider(tmpDir)

	ctx := context.Background()

	// Create first manager and add sessions
	mgr1, err := New(Config{
		MetadataFile: metadataFile,
		FileProvider: fileProvider,
		Logger:       logger.NewLogger(logger.Config{Level: logger.InfoLevel, Format: "text"}),
	})
	require.NoError(t, err)

	session1, err := mgr1.CreateNewSession(ctx, "slack", "U12345", "C67890")
	require.NoError(t, err)

	session2, err := mgr1.CreateNewSession(ctx, "telegram", "1985720465", "1985720465")
	require.NoError(t, err)

	// Create second manager (simulating restart)
	mgr2, err := New(Config{
		MetadataFile: metadataFile,
		FileProvider: fileProvider,
		Logger:       logger.NewLogger(logger.Config{Level: logger.InfoLevel, Format: "text"}),
	})
	require.NoError(t, err)

	// Verify sessions loaded
	slackSessions, err := mgr2.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	assert.Len(t, slackSessions, 1)
	assert.Equal(t, session1, slackSessions[0].SessionID)

	telegramSessions, err := mgr2.ListUserSessions(ctx, "telegram", "1985720465")
	require.NoError(t, err)
	assert.Len(t, telegramSessions, 1)
	assert.Equal(t, session2, telegramSessions[0].SessionID)
}

func TestConcurrentAccess(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Spawn multiple goroutines performing concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			userID := "U12345"
			channelID := "C67890"

			for j := 0; j < numOperations; j++ {
				switch j % 4 {
				case 0:
					// Create new session
					_, err := mgr.CreateNewSession(ctx, "slack", userID, channelID)
					if err != nil {
						errors <- err
					}
				case 1:
					// Get or create session
					_, err := mgr.GetOrCreateSession(ctx, "slack", userID, channelID)
					if err != nil {
						errors <- err
					}
				case 2:
					// List sessions
					_, err := mgr.ListUserSessions(ctx, "slack", userID)
					if err != nil {
						errors <- err
					}
				case 3:
					// Get latest session and update if found
					sessionID, err := mgr.GetLatestSession(ctx, "slack", userID)
					if err != nil {
						errors <- err
					}
					if sessionID != "" {
						_ = mgr.UpdateLastActive(ctx, sessionID)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify data consistency
	sessions, err := mgr.ListUserSessions(ctx, "slack", "U12345")
	require.NoError(t, err)
	t.Logf("Total sessions created: %d", len(sessions))

	// All sessions should have valid IDs
	for _, s := range sessions {
		assert.Contains(t, s.SessionID, "session-")
		assert.Equal(t, "slack", s.Connector)
		assert.Equal(t, "U12345", s.UserID)
		assert.Equal(t, "C67890", s.ChannelID)
		assert.False(t, s.CreatedAt.IsZero())
		assert.False(t, s.LastActive.IsZero())
	}
}

func TestSessionIsolation(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	// Create sessions for different users
	user1Session, err := mgr.CreateNewSession(ctx, "slack", "U11111", "C67890")
	require.NoError(t, err)

	user2Session, err := mgr.CreateNewSession(ctx, "slack", "U22222", "C67890")
	require.NoError(t, err)

	// Verify isolation
	user1Sessions, err := mgr.ListUserSessions(ctx, "slack", "U11111")
	require.NoError(t, err)
	assert.Len(t, user1Sessions, 1)
	assert.Equal(t, user1Session, user1Sessions[0].SessionID)

	user2Sessions, err := mgr.ListUserSessions(ctx, "slack", "U22222")
	require.NoError(t, err)
	assert.Len(t, user2Sessions, 1)
	assert.Equal(t, user2Session, user2Sessions[0].SessionID)
}
