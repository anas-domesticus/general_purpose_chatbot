package session

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Test helper functions to match the old convenience functions
func NewLocalJSONSessionService(baseDir string) session.Service {
	log := testLogger()
	service, err := NewSessionService(StorageConfig{
		Backend: "local",
		Local:   LocalConfig{BaseDir: baseDir},
		Logger:  log,
	})
	if err != nil {
		panic(err)
	}
	return service
}

func NewS3JSONSessionService(bucket, prefix string, s3Client S3Client) session.Service {
	log := testLogger()
	service, err := NewSessionService(StorageConfig{
		Backend: "s3",
		S3: S3Config{
			Bucket:   bucket,
			Prefix:   prefix,
			S3Client: s3Client,
		},
		Logger: log,
	})
	if err != nil {
		panic(err)
	}
	return service
}

func testLogger() logger.Logger {
	return logger.NewLogger(logger.Config{
		Level:  logger.ErrorLevel,
		Format: "text",
	})
}

func emptyService(t *testing.T) session.Service {
	t.Helper()
	tmpDir := t.TempDir()
	service, err := NewSessionService(StorageConfig{
		Backend: "local",
		Local:   LocalConfig{BaseDir: tmpDir},
		Logger:  testLogger(),
	})
	require.NoError(t, err)
	return service
}

func serviceWithData(t *testing.T) session.Service {
	t.Helper()
	s := emptyService(t)
	ctx := context.Background()

	// Create test sessions
	sessions := []struct {
		appName   string
		userID    string
		sessionID string
		state     map[string]any
		events    []*session.Event
	}{
		{
			appName:   "app1",
			userID:    "user1",
			sessionID: "session1",
			state:     map[string]any{"k1": "v1"},
		},
		{
			appName:   "app1",
			userID:    "user2",
			sessionID: "session1",
			state:     map[string]any{"k1": "v2"},
		},
		{
			appName:   "app1",
			userID:    "user1",
			sessionID: "session2",
			state:     map[string]any{"k1": "v2"},
		},
		{
			appName:   "app2",
			userID:    "user2",
			sessionID: "session2",
			state:     map[string]any{"k2": "v2"},
			events: []*session.Event{
				{
					ID:     "existing_event1",
					Author: "test",
					LLMResponse: model.LLMResponse{
						Partial: false,
					},
				},
			},
		},
	}

	for _, sess := range sessions {
		created, err := s.Create(ctx, &session.CreateRequest{
			AppName:   sess.appName,
			UserID:    sess.userID,
			SessionID: sess.sessionID,
			State:     sess.state,
		})
		require.NoError(t, err)

		// Append events if any
		for _, event := range sess.events {
			err = s.AppendEvent(ctx, created.Session, event)
			require.NoError(t, err)
		}
	}

	return s
}

func TestJSONSessionService_Create(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create service
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Test creating a new session
	req := &session.CreateRequest{
		AppName: "test-app",
		UserID:  "user123",
	}

	resp, err := service.Create(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Session)

	assert.Equal(t, "test-app", resp.Session.AppName())
	assert.Equal(t, "user123", resp.Session.UserID())
	assert.NotEmpty(t, resp.Session.ID())
	assert.False(t, resp.Session.LastUpdateTime().IsZero())

	// Verify file was created
	sessionID := resp.Session.ID()
	expectedPath := filepath.Join(tmpDir, "test-app", "user123", sessionID+".json")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "Session file should exist")
}

func TestJSONSessionService_CreateWithSessionID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Test creating a session with specific ID
	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "custom-session-id",
	}

	resp, err := service.Create(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "custom-session-id", resp.Session.ID())
}

func TestJSONSessionService_CreateDuplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "duplicate-session",
	}

	// Create first session
	_, err = service.Create(ctx, req)
	require.NoError(t, err)

	// Create duplicate session - should fail (ADK-compatible behaviour)
	_, err = service.Create(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestJSONSessionService_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session first
	createReq := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "get-test-session",
	}

	createResp, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Now get the session
	getReq := &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "get-test-session",
	}

	getResp, err := service.Get(ctx, getReq)
	require.NoError(t, err)
	require.NotNil(t, getResp)
	require.NotNil(t, getResp.Session)

	assert.Equal(t, createResp.Session.ID(), getResp.Session.ID())
	assert.Equal(t, createResp.Session.AppName(), getResp.Session.AppName())
	assert.Equal(t, createResp.Session.UserID(), getResp.Session.UserID())
	assert.True(t, createResp.Session.LastUpdateTime().Equal(getResp.Session.LastUpdateTime()),
		"LastUpdateTime should be equal (ignoring timezone)")
}

func TestJSONSessionService_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create multiple sessions
	sessions := []struct {
		appName   string
		userID    string
		sessionID string
	}{
		{"app1", "user1", "session1"},
		{"app1", "user1", "session2"},
		{"app1", "user2", "session3"},
		{"app2", "user1", "session4"},
	}

	for _, s := range sessions {
		req := &session.CreateRequest{
			AppName:   s.appName,
			UserID:    s.userID,
			SessionID: s.sessionID,
		}
		_, err := service.Create(ctx, req)
		require.NoError(t, err)
	}

	// Test listing all sessions for app1
	listReq := &session.ListRequest{
		AppName: "app1",
	}

	listResp, err := service.List(ctx, listReq)
	require.NoError(t, err)
	assert.Len(t, listResp.Sessions, 3) // 3 sessions for app1

	// Test listing sessions for specific user
	listReq = &session.ListRequest{
		AppName: "app1",
		UserID:  "user1",
	}

	listResp, err = service.List(ctx, listReq)
	require.NoError(t, err)
	assert.Len(t, listResp.Sessions, 2) // 2 sessions for app1/user1
}

func TestJSONSessionService_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createReq := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "delete-test-session",
	}

	_, err = service.Create(ctx, createReq)
	require.NoError(t, err)

	// Verify session exists
	getReq := &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "delete-test-session",
	}

	_, err = service.Get(ctx, getReq)
	require.NoError(t, err)

	// Delete the session
	deleteReq := &session.DeleteRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "delete-test-session",
	}

	err = service.Delete(ctx, deleteReq)
	require.NoError(t, err)

	// Verify session no longer exists
	_, err = service.Get(ctx, getReq)
	assert.Error(t, err)

	// Verify file was deleted
	expectedPath := filepath.Join(tmpDir, "test-app", "user123", "delete-test-session.json")
	_, err = os.Stat(expectedPath)
	assert.True(t, os.IsNotExist(err), "Session file should be deleted")
}

func TestJSONSessionService_InvalidRequests(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Test nil requests
	_, err = service.Create(ctx, nil)
	assert.Error(t, err)

	_, err = service.Get(ctx, nil)
	assert.Error(t, err)

	_, err = service.List(ctx, nil)
	assert.Error(t, err)

	err = service.Delete(ctx, nil)
	assert.Error(t, err)

	// Test empty required fields
	_, err = service.Create(ctx, &session.CreateRequest{})
	assert.Error(t, err)

	_, err = service.Create(ctx, &session.CreateRequest{AppName: "test"})
	assert.Error(t, err)
}

func TestJSONSessionService_S3Backend(t *testing.T) {
	// Create mock S3 client
	s3Client := NewMockS3Client()

	// Create service with S3 backend
	service := NewS3JSONSessionService("test-bucket", "sessions", s3Client)
	ctx := context.Background()

	// Test creating a session
	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "s3-test-session",
	}

	resp, err := service.Create(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "s3-test-session", resp.Session.ID())

	// Test getting the session
	getReq := &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "s3-test-session",
	}

	getResp, err := service.Get(ctx, getReq)
	require.NoError(t, err)
	assert.Equal(t, "s3-test-session", getResp.Session.ID())

	// Verify data was stored in mock S3
	data, err := s3Client.GetObject(ctx, "test-bucket", "sessions/test-app/user123/s3-test-session.json")
	require.NoError(t, err)
	assert.Contains(t, string(data), "s3-test-session")
}

func TestJSONSessionService_AppendEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session first
	createReq := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "event-test-session",
	}

	resp, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Create a test event
	testEvent := &session.Event{
		Author: "test-user",
	}

	// Append the event
	err = service.AppendEvent(ctx, resp.Session, testEvent)
	require.NoError(t, err)

	// Verify the event was added by getting the session and checking events
	getReq := &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "event-test-session",
	}

	updatedResp, err := service.Get(ctx, getReq)
	require.NoError(t, err)

	events := updatedResp.Session.Events()
	assert.Equal(t, 1, events.Len())

	firstEvent := events.At(0)
	require.NotNil(t, firstEvent)
	assert.Equal(t, "test-user", firstEvent.Author)
	assert.NotEmpty(t, firstEvent.ID)              // Should have generated an ID
	assert.False(t, firstEvent.Timestamp.IsZero()) // Should have set timestamp
}

func TestJSONSessionService_StateOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createReq := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-test-session",
	}

	resp, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	state := resp.Session.State()

	// Test setting and getting state values
	err = state.Set("test-key", "test-value")
	require.NoError(t, err)

	err = state.Set("number-key", 42)
	require.NoError(t, err)

	// Get values
	value, err := state.Get("test-key")
	require.NoError(t, err)
	assert.Equal(t, "test-value", value)

	numberValue, err := state.Get("number-key")
	require.NoError(t, err)
	assert.Equal(t, 42, numberValue)

	// Test getting non-existent key
	_, err = state.Get("non-existent")
	assert.Error(t, err)

	// Test iterating over all values
	var keys []string
	var values []interface{}
	for key, value := range state.All() {
		keys = append(keys, key)
		values = append(values, value)
	}

	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "test-key")
	assert.Contains(t, keys, "number-key")
}

func TestNewSessionService(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	log := testLogger()

	// Test local storage configuration
	service, err := NewSessionService(StorageConfig{
		Backend: "local",
		Local:   LocalConfig{BaseDir: tmpDir},
		Logger:  log,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	// Test S3 storage configuration
	s3Client := NewMockS3Client()
	service, err = NewSessionService(StorageConfig{
		Backend: "s3",
		S3: S3Config{
			Bucket:   "bucket",
			Prefix:   "prefix",
			S3Client: s3Client,
		},
		Logger: log,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	// Test invalid backend
	_, err = NewSessionService(StorageConfig{
		Backend: "invalid",
		Logger:  log,
	})
	assert.Error(t, err)

	// Test local storage with missing BaseDir
	_, err = NewSessionService(StorageConfig{
		Backend: "local",
		Local:   LocalConfig{},
		Logger:  log,
	})
	assert.Error(t, err)

	// Test S3 storage with missing Bucket
	_, err = NewSessionService(StorageConfig{
		Backend: "s3",
		S3:      S3Config{S3Client: s3Client},
		Logger:  log,
	})
	assert.Error(t, err)

	// Test S3 storage with missing S3Client
	_, err = NewSessionService(StorageConfig{
		Backend: "s3",
		S3:      S3Config{Bucket: "bucket"},
		Logger:  log,
	})
	assert.Error(t, err)

	// Test missing logger
	_, err = NewSessionService(StorageConfig{
		Backend: "local",
		Local:   LocalConfig{BaseDir: tmpDir},
	})
	assert.Error(t, err)
}

// TestJSONSessionService_AppendEvent_StateDelta tests that event state deltas are applied to session state
func TestJSONSessionService_AppendEvent_StateDelta(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-delta-test",
	})
	require.NoError(t, err)

	// Create an event with state delta
	event := &session.Event{
		Author: "test-user",
		Actions: session.EventActions{
			StateDelta: map[string]any{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		},
	}

	// Append the event
	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Get the session and verify state was updated
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-delta-test",
	})
	require.NoError(t, err)

	state := getResp.Session.State()

	value1, err := state.Get("key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", value1)

	value2, err := state.Get("key2")
	require.NoError(t, err)
	assert.Equal(t, 42, value2)

	value3, err := state.Get("key3")
	require.NoError(t, err)
	assert.Equal(t, true, value3)
}

// TestJSONSessionService_AppendEvent_TemporaryKeysFiltered tests that temporary keys are not persisted
func TestJSONSessionService_AppendEvent_TemporaryKeysFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "temp-keys-test",
	})
	require.NoError(t, err)

	// Create an event with both regular and temporary state keys
	event := &session.Event{
		Author: "test-user",
		Actions: session.EventActions{
			StateDelta: map[string]any{
				"regular_key":   "should_persist",
				"temp:temp_key": "should_not_persist",
			},
		},
	}

	// Append the event
	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Get the session and verify only regular keys were persisted
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "temp-keys-test",
	})
	require.NoError(t, err)

	state := getResp.Session.State()

	// Regular key should exist
	value, err := state.Get("regular_key")
	require.NoError(t, err)
	assert.Equal(t, "should_persist", value)

	// Temporary key should NOT exist
	_, err = state.Get("temp:temp_key")
	assert.Error(t, err, "Temporary keys should not be persisted")
}

// TestJSONSessionService_AppendEvent_PartialEventsSkipped tests that partial events are skipped
func TestJSONSessionService_AppendEvent_PartialEventsSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "partial-event-test",
	})
	require.NoError(t, err)

	// Create a partial event - Partial is in the LLMResponse
	partialEvent := &session.Event{
		Author: "test-user",
		LLMResponse: model.LLMResponse{
			Partial: true,
		},
	}

	// Append the partial event - should not error but also not save
	err = service.AppendEvent(ctx, createResp.Session, partialEvent)
	require.NoError(t, err)

	// Get the session and verify no events were added
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "partial-event-test",
	})
	require.NoError(t, err)

	events := getResp.Session.Events()
	assert.Equal(t, 0, events.Len(), "Partial events should not be persisted")
}

// TestJSONSessionService_Get_EventFiltering tests event filtering by NumRecentEvents and After
func TestJSONSessionService_Get_EventFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "filter-test",
	})
	require.NoError(t, err)

	// Append multiple events with different timestamps
	baseTime := time.Now().Add(-10 * time.Minute)
	for i := 0; i < 5; i++ {
		event := &session.Event{
			Author:    "test-user",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
		err = service.AppendEvent(ctx, createResp.Session, event)
		require.NoError(t, err)
	}

	// Test 1: Get only recent events
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:         "test-app",
		UserID:          "user123",
		SessionID:       "filter-test",
		NumRecentEvents: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, getResp.Session.Events().Len(), "Should return only 2 most recent events")

	// Test 2: Get events after a timestamp
	cutoffTime := baseTime.Add(2 * time.Minute)
	getResp, err = service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "filter-test",
		After:     cutoffTime,
	})
	require.NoError(t, err)
	// Should get events at minutes 2, 3, 4 (3 events >= cutoffTime)
	assert.Equal(t, 3, getResp.Session.Events().Len(), "Should return events after cutoff time")

	// Test 3: Combine both filters
	getResp, err = service.Get(ctx, &session.GetRequest{
		AppName:         "test-app",
		UserID:          "user123",
		SessionID:       "filter-test",
		NumRecentEvents: 3,
		After:           baseTime.Add(1 * time.Minute),
	})
	require.NoError(t, err)
	// NumRecentEvents=3 would give last 3 events (indices 2,3,4)
	// After filter then applies to those 3, so we get events >= minute 1
	// Since we took the last 3 (minutes 2,3,4), all are >= minute 1
	assert.LessOrEqual(t, getResp.Session.Events().Len(), 3, "Should respect NumRecentEvents limit")

	// Test 4: Get all events (no filtering)
	getResp, err = service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "filter-test",
	})
	require.NoError(t, err)
	assert.Equal(t, 5, getResp.Session.Events().Len(), "Should return all events when no filters")
}

// TestJSONSessionService_AppendEvent_ConcurrentAppends tests that concurrent event appends don't result in lost events
func TestJSONSessionService_AppendEvent_ConcurrentAppends(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create service
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test",
		UserID:    "user1",
		SessionID: "session1",
	})
	require.NoError(t, err)

	// Number of concurrent goroutines
	numGoroutines := 10

	// Channel to signal start
	startChan := make(chan struct{})
	doneChan := make(chan error, numGoroutines)

	// Launch concurrent goroutines that append events
	for i := 0; i < numGoroutines; i++ {
		go func() {
			// Wait for start signal
			<-startChan

			// Append event - each event will get a unique timestamp-based ID
			event := &session.Event{
				Timestamp: time.Now(),
			}
			err := service.AppendEvent(ctx, createResp.Session, event)
			doneChan <- err
		}()
	}

	// Start all goroutines at once
	close(startChan)

	// Wait for all to complete
	for i := 0; i < numGoroutines; i++ {
		err := <-doneChan
		require.NoError(t, err)
	}

	// Verify all events were saved
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test",
		UserID:    "user1",
		SessionID: "session1",
	})
	require.NoError(t, err)

	// Check that we have all events
	eventCount := getResp.Session.Events().Len()
	assert.Equal(t, numGoroutines, eventCount, "Expected all %d events to be saved, but got %d", numGoroutines, eventCount)

	// Verify all event IDs are unique (no events were lost/overwritten)
	eventIDs := make(map[string]bool)
	for event := range getResp.Session.Events().All() {
		if event.ID != "" {
			eventIDs[event.ID] = true
		}
	}
	assert.Equal(t, numGoroutines, len(eventIDs), "Expected all event IDs to be unique")
}

// TestJSONSessionService_AppendEvent_InMemoryEventVisibility verifies that events are visible
// in the session object immediately after AppendEvent without re-fetching from storage.
// This is critical for ADK compatibility - the runner reuses the same session object.
func TestJSONSessionService_AppendEvent_InMemoryEventVisibility(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "inmemory-event-test",
	})
	require.NoError(t, err)

	// Verify initial state has no events
	assert.Equal(t, 0, createResp.Session.Events().Len(), "New session should have no events")

	// Append first event
	event1 := &session.Event{Author: "user"}
	err = service.AppendEvent(ctx, createResp.Session, event1)
	require.NoError(t, err)

	// Verify event is visible in-memory WITHOUT re-fetching
	assert.Equal(t, 1, createResp.Session.Events().Len(),
		"Event should be visible in-memory immediately after AppendEvent")

	// Append second event
	event2 := &session.Event{Author: "assistant"}
	err = service.AppendEvent(ctx, createResp.Session, event2)
	require.NoError(t, err)

	// Verify both events are visible in-memory
	assert.Equal(t, 2, createResp.Session.Events().Len(),
		"Both events should be visible in-memory")

	// Verify event order is preserved
	events := createResp.Session.Events()
	assert.Equal(t, "user", events.At(0).Author)
	assert.Equal(t, "assistant", events.At(1).Author)
}

// TestJSONSessionService_AppendEvent_InMemoryStateVisibility verifies that state changes
// from StateDelta are visible in the session object immediately after AppendEvent.
func TestJSONSessionService_AppendEvent_InMemoryStateVisibility(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session with initial state
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "inmemory-state-test",
		State:     map[string]any{"initial_key": "initial_value"},
	})
	require.NoError(t, err)

	// Verify initial state
	initialValue, err := createResp.Session.State().Get("initial_key")
	require.NoError(t, err)
	assert.Equal(t, "initial_value", initialValue)

	// Append event with state delta
	event := &session.Event{
		Author: "test",
		Actions: session.EventActions{
			StateDelta: map[string]any{
				"new_key":     "new_value",
				"initial_key": "updated_value",
			},
		},
	}
	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Verify state changes are visible in-memory WITHOUT re-fetching
	newValue, err := createResp.Session.State().Get("new_key")
	require.NoError(t, err)
	assert.Equal(t, "new_value", newValue, "New state key should be visible in-memory")

	updatedValue, err := createResp.Session.State().Get("initial_key")
	require.NoError(t, err)
	assert.Equal(t, "updated_value", updatedValue, "Updated state key should be visible in-memory")
}

// TestJSONSessionService_AppendEvent_InMemoryAndPersistenceConsistency verifies that
// in-memory state and persisted state remain consistent after AppendEvent.
func TestJSONSessionService_AppendEvent_InMemoryAndPersistenceConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "consistency-test",
	})
	require.NoError(t, err)

	// Append multiple events with state
	for i := 0; i < 3; i++ {
		event := &session.Event{
			Author: "test",
			Actions: session.EventActions{
				StateDelta: map[string]any{
					"counter": i + 1,
				},
			},
		}
		err = service.AppendEvent(ctx, createResp.Session, event)
		require.NoError(t, err)
	}

	// Check in-memory state
	inMemoryEvents := createResp.Session.Events().Len()
	inMemoryCounter, err := createResp.Session.State().Get("counter")
	require.NoError(t, err)

	// Re-fetch from storage
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "consistency-test",
	})
	require.NoError(t, err)

	persistedEvents := getResp.Session.Events().Len()
	persistedCounter, err := getResp.Session.State().Get("counter")
	require.NoError(t, err)

	// Verify consistency
	assert.Equal(t, inMemoryEvents, persistedEvents,
		"In-memory and persisted event counts should match")
	assert.Equal(t, inMemoryCounter, persistedCounter,
		"In-memory and persisted state should match")
	assert.Equal(t, 3, inMemoryEvents)
	assert.Equal(t, 3, inMemoryCounter)
}

// TestJSONSessionService_AppendEvent_PartialEventsNotVisibleInMemory verifies that
// partial events are not added to the in-memory session.
func TestJSONSessionService_AppendEvent_PartialEventsNotVisibleInMemory(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "partial-inmemory-test",
	})
	require.NoError(t, err)

	// Append a partial event (Partial is in LLMResponse)
	partialEvent := &session.Event{
		Author: "assistant",
		LLMResponse: model.LLMResponse{
			Partial: true,
		},
	}
	err = service.AppendEvent(ctx, createResp.Session, partialEvent)
	require.NoError(t, err)

	// Verify partial event is NOT visible in-memory
	assert.Equal(t, 0, createResp.Session.Events().Len(),
		"Partial events should not be visible in-memory")

	// Append a complete event
	completeEvent := &session.Event{
		Author: "assistant",
		LLMResponse: model.LLMResponse{
			Partial: false,
		},
	}
	err = service.AppendEvent(ctx, createResp.Session, completeEvent)
	require.NoError(t, err)

	// Verify only complete event is visible
	assert.Equal(t, 1, createResp.Session.Events().Len(),
		"Only complete events should be visible in-memory")
}

// TestJSONSessionService_AppendEvent_TempKeysNotVisibleInMemory verifies that
// temporary state keys are not visible in the in-memory session state.
func TestJSONSessionService_AppendEvent_TempKeysNotVisibleInMemory(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "temp-keys-inmemory-test",
	})
	require.NoError(t, err)

	// Append event with both regular and temp keys
	event := &session.Event{
		Author: "test",
		Actions: session.EventActions{
			StateDelta: map[string]any{
				"regular_key":   "regular_value",
				"temp:temp_key": "temp_value",
			},
		},
	}
	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Verify regular key is visible in-memory
	regularValue, err := createResp.Session.State().Get("regular_key")
	require.NoError(t, err)
	assert.Equal(t, "regular_value", regularValue)

	// Verify temp key is NOT visible in-memory
	_, err = createResp.Session.State().Get("temp:temp_key")
	assert.Error(t, err, "Temporary keys should not be visible in-memory state")
}

// TestJSONSessionService_Get_SessionNotFound verifies proper error handling for non-existent sessions.
func TestJSONSessionService_Get_SessionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	_, err := service.Get(ctx, &session.GetRequest{
		AppName:   "non-existent-app",
		UserID:    "non-existent-user",
		SessionID: "non-existent-session",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestJSONSessionService_AppendEvent_NilSession verifies proper error handling for nil session.
func TestJSONSessionService_AppendEvent_NilSession(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	err := service.AppendEvent(ctx, nil, &session.Event{Author: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// TestJSONSessionService_AppendEvent_NilEvent verifies proper error handling for nil event.
func TestJSONSessionService_AppendEvent_NilEvent(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "nil-event-test",
	})
	require.NoError(t, err)

	err = service.AppendEvent(ctx, createResp.Session, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// TestJSONSessionService_Create_WithInitialState verifies that initial state is properly set.
func TestJSONSessionService_Create_WithInitialState(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	initialState := map[string]any{
		"string_val": "hello",
		"int_val":    42,
		"bool_val":   true,
		"nested": map[string]any{
			"inner": "value",
		},
	}

	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "initial-state-test",
		State:     initialState,
	})
	require.NoError(t, err)

	// Verify all initial state values are accessible
	state := createResp.Session.State()

	strVal, err := state.Get("string_val")
	require.NoError(t, err)
	assert.Equal(t, "hello", strVal)

	intVal, err := state.Get("int_val")
	require.NoError(t, err)
	assert.Equal(t, 42, intVal)

	boolVal, err := state.Get("bool_val")
	require.NoError(t, err)
	assert.Equal(t, true, boolVal)

	nestedVal, err := state.Get("nested")
	require.NoError(t, err)
	nestedMap, ok := nestedVal.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", nestedMap["inner"])
}

// TestJSONSessionService_Events_Iteration verifies that event iteration works correctly.
func TestJSONSessionService_Events_Iteration(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "iteration-test",
	})
	require.NoError(t, err)

	// Append several events
	authors := []string{"user", "assistant", "user", "assistant", "user"}
	for _, author := range authors {
		err = service.AppendEvent(ctx, createResp.Session, &session.Event{Author: author})
		require.NoError(t, err)
	}

	// Test iteration with All()
	var iteratedAuthors []string
	for event := range createResp.Session.Events().All() {
		iteratedAuthors = append(iteratedAuthors, event.Author)
	}
	assert.Equal(t, authors, iteratedAuthors, "Iteration should return events in order")

	// Test Len()
	assert.Equal(t, len(authors), createResp.Session.Events().Len())

	// Test At()
	for i, author := range authors {
		assert.Equal(t, author, createResp.Session.Events().At(i).Author)
	}

	// Test At() with out of bounds
	assert.Nil(t, createResp.Session.Events().At(-1))
	assert.Nil(t, createResp.Session.Events().At(100))
}

// TestJSONSessionService_State_Iteration verifies that state iteration works correctly.
func TestJSONSessionService_State_Iteration(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalJSONSessionService(tmpDir)
	ctx := context.Background()

	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-iteration-test",
		State: map[string]any{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	})
	require.NoError(t, err)

	// Test iteration with All()
	iteratedState := make(map[string]any)
	for key, value := range createResp.Session.State().All() {
		iteratedState[key] = value
	}

	assert.Len(t, iteratedState, 3)
	assert.Equal(t, "value1", iteratedState["key1"])
	assert.Equal(t, "value2", iteratedState["key2"])
	assert.Equal(t, "value3", iteratedState["key3"])
}

// Table-driven tests using setup functions

func TestJSONSessionService_Create_Variants(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) session.Service
		req     *session.CreateRequest
		wantErr bool
	}{
		{
			name:  "full_key",
			setup: emptyService,
			req: &session.CreateRequest{
				AppName:   "testApp",
				UserID:    "testUserID",
				SessionID: "testSessionID",
				State: map[string]any{
					"k": 5,
				},
			},
		},
		{
			name:  "generated_session_id",
			setup: emptyService,
			req: &session.CreateRequest{
				AppName: "testApp",
				UserID:  "testUserID",
				State: map[string]any{
					"k": 5,
				},
			},
		},
		{
			name:  "already_exists_returns_error",
			setup: serviceWithData,
			req: &session.CreateRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
				State: map[string]any{
					"k": 10,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup(t)
			ctx := context.Background()

			got, err := s.Create(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Service.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if got.Session.AppName() != tt.req.AppName {
				t.Errorf("AppName got: %v, want: %v", got.Session.AppName(), tt.req.AppName)
			}

			if got.Session.UserID() != tt.req.UserID {
				t.Errorf("UserID got: %v, want: %v", got.Session.UserID(), tt.req.UserID)
			}

			if tt.req.SessionID != "" {
				if got.Session.ID() != tt.req.SessionID {
					t.Errorf("SessionID got: %v, want: %v", got.Session.ID(), tt.req.SessionID)
				}
			} else {
				if got.Session.ID() == "" {
					t.Errorf("SessionID was not generated on empty user input.")
				}
			}

			gotState := maps.Collect(got.Session.State().All())
			wantState := tt.req.State

			if diff := cmp.Diff(wantState, gotState); diff != "" {
				t.Errorf("Create State mismatch: (-want +got):\n%s", diff)
			}
		})
	}
}

func TestJSONSessionService_Delete_Variants(t *testing.T) {
	tests := []struct {
		name    string
		req     *session.DeleteRequest
		setup   func(t *testing.T) session.Service
		wantErr bool
	}{
		{
			name:  "delete_ok",
			setup: serviceWithData,
			req: &session.DeleteRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
			},
		},
		{
			name:  "no_error_when_not_found",
			setup: serviceWithData,
			req: &session.DeleteRequest{
				AppName:   "appTest",
				UserID:    "user1",
				SessionID: "session1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup(t)
			ctx := context.Background()
			if err := s.Delete(ctx, tt.req); (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJSONSessionService_Get_Variants(t *testing.T) {
	setupGetRespectsUserID := func(t *testing.T) session.Service {
		t.Helper()
		s := serviceWithData(t)
		ctx := context.Background()

		session1, err := s.Get(ctx, &session.GetRequest{
			AppName:   "app1",
			UserID:    "user1",
			SessionID: "session1",
		})
		require.NoError(t, err)

		err = s.AppendEvent(ctx, session1.Session, &session.Event{
			ID:     "event_for_user1",
			Author: "user",
			LLMResponse: model.LLMResponse{
				Partial: false,
			},
		})
		require.NoError(t, err)
		return s
	}

	setupGetWithConfig := func(t *testing.T) session.Service {
		t.Helper()
		s := emptyService(t)
		ctx := context.Background()
		numTestEvents := 5
		created, err := s.Create(ctx, &session.CreateRequest{
			AppName:   "my_app",
			UserID:    "user",
			SessionID: "s1",
		})
		require.NoError(t, err)

		for i := 1; i <= numTestEvents; i++ {
			event := &session.Event{
				ID:          string(rune('0' + i)),
				Author:      "user",
				Timestamp:   time.Time{}.Add(time.Duration(i)),
				LLMResponse: model.LLMResponse{},
			}
			err := s.AppendEvent(ctx, created.Session, event)
			require.NoError(t, err)
		}
		return s
	}

	tests := []struct {
		name       string
		req        *session.GetRequest
		setup      func(t *testing.T) session.Service
		wantEvents []*session.Event
		wantErr    bool
		checkState func(t *testing.T, sess session.Session)
	}{
		{
			name:  "ok",
			setup: serviceWithData,
			req: &session.GetRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
			},
			checkState: func(t *testing.T, sess session.Session) {
				gotState := maps.Collect(sess.State().All())
				wantState := map[string]any{"k1": "v1"}
				if diff := cmp.Diff(wantState, gotState); diff != "" {
					t.Errorf("State mismatch: (-want +got):\n%s", diff)
				}
			},
		},
		{
			name:  "error_when_not_found",
			setup: serviceWithData,
			req: &session.GetRequest{
				AppName:   "testApp",
				UserID:    "user1",
				SessionID: "session1",
			},
			wantErr: true,
		},
		{
			name:  "respects_user_id",
			setup: setupGetRespectsUserID,
			req: &session.GetRequest{
				AppName:   "app1",
				UserID:    "user2",
				SessionID: "session1",
			},
			checkState: func(t *testing.T, sess session.Session) {
				gotState := maps.Collect(sess.State().All())
				wantState := map[string]any{"k1": "v2"}
				if diff := cmp.Diff(wantState, gotState); diff != "" {
					t.Errorf("State mismatch: (-want +got):\n%s", diff)
				}
				// Should NOT have the event from user1's session
				if sess.Events().Len() != 0 {
					t.Errorf("user2's session should have 0 events, got %d", sess.Events().Len())
				}
			},
			wantErr: false,
		},
		{
			name:  "no_config_returns_all_events",
			setup: setupGetWithConfig,
			req: &session.GetRequest{
				AppName: "my_app", UserID: "user", SessionID: "s1",
			},
			wantEvents: []*session.Event{
				{ID: "1", Author: "user", Timestamp: time.Time{}.Add(1), LLMResponse: model.LLMResponse{}},
				{ID: "2", Author: "user", Timestamp: time.Time{}.Add(2), LLMResponse: model.LLMResponse{}},
				{ID: "3", Author: "user", Timestamp: time.Time{}.Add(3), LLMResponse: model.LLMResponse{}},
				{ID: "4", Author: "user", Timestamp: time.Time{}.Add(4), LLMResponse: model.LLMResponse{}},
				{ID: "5", Author: "user", Timestamp: time.Time{}.Add(5), LLMResponse: model.LLMResponse{}},
			},
		},
		{
			name:  "num_recent_events_filter",
			setup: setupGetWithConfig,
			req: &session.GetRequest{
				AppName: "my_app", UserID: "user", SessionID: "s1",
				NumRecentEvents: 3,
			},
			wantEvents: []*session.Event{
				{ID: "3", Author: "user", Timestamp: time.Time{}.Add(3), LLMResponse: model.LLMResponse{}},
				{ID: "4", Author: "user", Timestamp: time.Time{}.Add(4), LLMResponse: model.LLMResponse{}},
				{ID: "5", Author: "user", Timestamp: time.Time{}.Add(5), LLMResponse: model.LLMResponse{}},
			},
		},
		{
			name:  "after_timestamp_filter",
			setup: setupGetWithConfig,
			req: &session.GetRequest{
				AppName: "my_app", UserID: "user", SessionID: "s1",
				After: time.Time{}.Add(4),
			},
			wantEvents: []*session.Event{
				{ID: "4", Author: "user", Timestamp: time.Time{}.Add(4), LLMResponse: model.LLMResponse{}},
				{ID: "5", Author: "user", Timestamp: time.Time{}.Add(5), LLMResponse: model.LLMResponse{}},
			},
		},
		{
			name:  "combined_filters",
			setup: setupGetWithConfig,
			req: &session.GetRequest{
				AppName: "my_app", UserID: "user", SessionID: "s1",
				NumRecentEvents: 3,
				After:           time.Time{}.Add(4),
			},
			wantEvents: []*session.Event{
				{ID: "4", Author: "user", Timestamp: time.Time{}.Add(4), LLMResponse: model.LLMResponse{}},
				{ID: "5", Author: "user", Timestamp: time.Time{}.Add(5), LLMResponse: model.LLMResponse{}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup(t)
			ctx := context.Background()

			got, err := s.Get(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Service.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if tt.checkState != nil {
				tt.checkState(t, got.Session)
			}

			if tt.wantEvents != nil {
				opts := []cmp.Option{
					cmpopts.SortSlices(func(a, b *session.Event) bool { return a.Timestamp.Before(b.Timestamp) }),
					cmpopts.IgnoreFields(session.Event{}, "Timestamp"),
				}

				var gotEvents []*session.Event
				for event := range got.Session.Events().All() {
					gotEvents = append(gotEvents, event)
				}

				if diff := cmp.Diff(tt.wantEvents, gotEvents, opts...); diff != "" {
					t.Errorf("Get session events mismatch: (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestJSONSessionService_List_Variants(t *testing.T) {
	tests := []struct {
		name      string
		req       *session.ListRequest
		setup     func(t *testing.T) session.Service
		wantCount int
		wantErr   bool
	}{
		{
			name:  "list_for_user1",
			setup: serviceWithData,
			req: &session.ListRequest{
				AppName: "app1",
				UserID:  "user1",
			},
			wantCount: 2,
		},
		{
			name:  "empty_list_for_nonexistent_user",
			setup: serviceWithData,
			req: &session.ListRequest{
				AppName: "app1",
				UserID:  "custom_user",
			},
			wantCount: 0,
		},
		{
			name:  "list_for_user2",
			setup: serviceWithData,
			req: &session.ListRequest{
				AppName: "app1",
				UserID:  "user2",
			},
			wantCount: 1,
		},
		{
			name:      "list_all_users_for_app",
			setup:     serviceWithData,
			req:       &session.ListRequest{AppName: "app1", UserID: ""},
			wantCount: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup(t)
			ctx := context.Background()
			got, err := s.List(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if len(got.Sessions) != tt.wantCount {
					t.Errorf("Service.List() returned %d sessions, want %d", len(got.Sessions), tt.wantCount)
				}
			}
		})
	}
}

func TestJSONSessionService_AppendEvent_Variants(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) session.Service
		sessionReq     *session.CreateRequest
		event          *session.Event
		wantEventCount int
		wantState      map[string]any
		wantErr        bool
	}{
		{
			name:  "append_event_to_session",
			setup: emptyService,
			sessionReq: &session.CreateRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
				State:     map[string]any{"k1": "v1"},
			},
			event: &session.Event{
				ID:          "new_event1",
				LLMResponse: model.LLMResponse{Partial: false},
			},
			wantEventCount: 1,
			wantState:      map[string]any{"k1": "v1"},
		},
		{
			name:  "append_event_with_state_delta",
			setup: emptyService,
			sessionReq: &session.CreateRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
				State:     map[string]any{"k1": "v1"},
			},
			event: &session.Event{
				ID:      "event_complete",
				Author:  "user",
				Actions: session.EventActions{StateDelta: map[string]any{"k2": "v2"}},
				LLMResponse: model.LLMResponse{
					Content:      genai.NewContentFromText("test_text", "user"),
					TurnComplete: true,
					Partial:      false,
				},
			},
			wantEventCount: 1,
			wantState:      map[string]any{"k1": "v1", "k2": "v2"},
		},
		{
			name:  "partial_events_not_persisted",
			setup: emptyService,
			sessionReq: &session.CreateRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
				State:     map[string]any{"k1": "v1"},
			},
			event: &session.Event{
				ID:          "partial_event",
				Author:      "user",
				LLMResponse: model.LLMResponse{Partial: true},
			},
			wantEventCount: 0,
			wantState:      map[string]any{"k1": "v1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			s := tt.setup(t)

			created, err := s.Create(ctx, tt.sessionReq)
			require.NoError(t, err)

			err = s.AppendEvent(ctx, created.Session, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.AppendEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			resp, err := s.Get(ctx, &session.GetRequest{
				AppName:   tt.sessionReq.AppName,
				UserID:    tt.sessionReq.UserID,
				SessionID: tt.sessionReq.SessionID,
			})
			require.NoError(t, err)

			if resp.Session.Events().Len() != tt.wantEventCount {
				t.Errorf("AppendEvent returned %d events, want %d", resp.Session.Events().Len(), tt.wantEventCount)
			}

			gotState := maps.Collect(resp.Session.State().All())
			if diff := cmp.Diff(tt.wantState, gotState); diff != "" {
				t.Errorf("AppendEvent state mismatch: (-want +got):\n%s", diff)
			}
		})
	}
}

func TestJSONSessionService_StateManagement(t *testing.T) {
	ctx := context.Background()
	appName := "my_app"

	t.Run("session_state_is_not_shared", func(t *testing.T) {
		s := emptyService(t)
		s1, err := s.Create(ctx, &session.CreateRequest{
			AppName:   appName,
			UserID:    "u1",
			SessionID: "s1",
			State:     map[string]any{"sk1": "v1"},
		})
		require.NoError(t, err)

		err = s.AppendEvent(ctx, s1.Session, &session.Event{
			ID:          "event1",
			Actions:     session.EventActions{StateDelta: map[string]any{"sk2": "v2"}},
			LLMResponse: model.LLMResponse{},
		})
		require.NoError(t, err)

		s1Got, err := s.Get(ctx, &session.GetRequest{AppName: appName, UserID: "u1", SessionID: "s1"})
		require.NoError(t, err)

		wantState := map[string]any{"sk1": "v1", "sk2": "v2"}
		gotState := maps.Collect(s1Got.Session.State().All())
		if diff := cmp.Diff(wantState, gotState); diff != "" {
			t.Errorf("Refetched s1 state mismatch (-want +got):\n%s", diff)
		}

		s1b, err := s.Create(ctx, &session.CreateRequest{AppName: appName, UserID: "u1", SessionID: "s1b"})
		require.NoError(t, err)

		gotStateS1b := maps.Collect(s1b.Session.State().All())
		if len(gotStateS1b) != 0 {
			t.Errorf("Session s1b should have empty state, but got: %v", gotStateS1b)
		}
	})

	t.Run("temp_state_is_not_persisted", func(t *testing.T) {
		s := emptyService(t)
		s1, err := s.Create(ctx, &session.CreateRequest{AppName: appName, UserID: "u1", SessionID: "s1"})
		require.NoError(t, err)

		event := &session.Event{
			ID:          "event1",
			Actions:     session.EventActions{StateDelta: map[string]any{"temp:k1": "v1", "sk": "v2"}},
			LLMResponse: model.LLMResponse{},
		}
		err = s.AppendEvent(ctx, s1.Session, event)
		require.NoError(t, err)

		s1Got, err := s.Get(ctx, &session.GetRequest{AppName: appName, UserID: "u1", SessionID: "s1"})
		require.NoError(t, err)

		wantState := map[string]any{"sk": "v2"}
		gotState := maps.Collect(s1Got.Session.State().All())
		if diff := cmp.Diff(wantState, gotState); diff != "" {
			t.Errorf("Persisted state mismatch (-want +got):\n%s", diff)
		}

		storedEvents := s1Got.Session.Events()
		if storedEvents.Len() != 1 {
			t.Fatalf("Expected 1 stored event, got %d", storedEvents.Len())
		}
		storedDelta := storedEvents.At(0).Actions.StateDelta
		if _, exists := storedDelta["temp:k1"]; exists {
			t.Errorf("temp:k1 key should not be in stored event's state delta")
		}
		if storedDelta["sk"] != "v2" {
			t.Errorf("Expected 'sk' key in stored event, but was missing or wrong value")
		}
	})
}
