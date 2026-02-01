package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
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

func TestJSONSessionService_Create(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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

	// Create duplicate session - should fail (ADK-compatible behavior)
	_, err = service.Create(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestJSONSessionService_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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

// TestAppendEvent_StateDelta tests that event state deltas are applied to session state
func TestAppendEvent_StateDelta(t *testing.T) {
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

// TestAppendEvent_TemporaryKeysFiltered tests that temporary keys are not persisted
func TestAppendEvent_TemporaryKeysFiltered(t *testing.T) {
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

// TestAppendEvent_PartialEvents tests that partial events are skipped
func TestAppendEvent_PartialEvents(t *testing.T) {
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

// TestGet_EventFiltering tests event filtering by NumRecentEvents and After
func TestGet_EventFiltering(t *testing.T) {
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

// TestConcurrentAppendEvent tests that concurrent event appends don't result in lost events
func TestConcurrentAppendEvent(t *testing.T) {
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
