package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/session"
)

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
	resp1, err := service.Create(ctx, req)
	require.NoError(t, err)

	// Create duplicate session - should return existing
	resp2, err := service.Create(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, resp1.Session.ID(), resp2.Session.ID())
	// Use approximate time comparison to handle timezone differences
	assert.WithinDuration(t, resp1.Session.LastUpdateTime(), resp2.Session.LastUpdateTime(), time.Second)
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
	assert.Equal(t, createResp.Session.LastUpdateTime(), getResp.Session.LastUpdateTime())
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

func TestSessionServiceBuilder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Test builder with local storage
	builder := NewSessionServiceBuilder()
	service, err := builder.WithLocalFileStorage(tmpDir).Build()
	require.NoError(t, err)
	require.NotNil(t, service)

	// Test builder with S3 storage
	s3Client := NewMockS3Client()
	builder = NewSessionServiceBuilder()
	service, err = builder.WithS3Storage("bucket", "prefix", s3Client).Build()
	require.NoError(t, err)
	require.NotNil(t, service)

	// Test builder with no provider
	builder = NewSessionServiceBuilder()
	_, err = builder.Build()
	assert.Error(t, err)
}
