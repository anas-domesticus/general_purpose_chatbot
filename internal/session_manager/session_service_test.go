package session_manager

import (
	"context"
	"fmt"
	"testing"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager/mocks"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
)

// Test helper functions
func testLogger() logger.Logger {
	return logger.NewLogger(logger.Config{
		Level:  logger.ErrorLevel,
		Format: "text",
	})
}

func emptySessionService(t *testing.T) session.Service {
	t.Helper()
	tmpDir := t.TempDir()
	provider := storage_manager.NewLocalFileProvider(tmpDir)
	return NewSessionService(provider, testLogger())
}

func sessionServiceWithData(t *testing.T) session.Service {
	t.Helper()
	s := emptySessionService(t)
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

		for _, e := range sess.events {
			err := s.AppendEvent(ctx, created.Session, e)
			require.NoError(t, err)
		}
	}

	return s
}

func TestSessionService_Create(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	// Test basic creation
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
}

func TestSessionService_CreateWithSessionID(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "custom-session-id",
	}

	resp, err := service.Create(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "custom-session-id", resp.Session.ID())
}

func TestSessionService_CreateWithState(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-test",
		State: map[string]any{
			"key1": "value1",
			"key2": 42,
		},
	}

	resp, err := service.Create(ctx, req)
	require.NoError(t, err)

	// Verify state was saved
	val1, err := resp.Session.State().Get("key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val1)

	val2, err := resp.Session.State().Get("key2")
	require.NoError(t, err)
	assert.Equal(t, 42, val2)
}

func TestSessionService_CreateDuplicate(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "duplicate-test",
	}

	_, err := service.Create(ctx, req)
	require.NoError(t, err)

	// Try to create again
	_, err = service.Create(ctx, req)
	assert.Error(t, err)
}

func TestSessionService_Get(t *testing.T) {
	service := sessionServiceWithData(t)
	ctx := context.Background()

	resp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "app1",
		UserID:    "user1",
		SessionID: "session1",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Session)

	assert.Equal(t, "app1", resp.Session.AppName())
	assert.Equal(t, "user1", resp.Session.UserID())
	assert.Equal(t, "session1", resp.Session.ID())
}

func TestSessionService_GetNotFound(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	_, err := service.Get(ctx, &session.GetRequest{
		AppName:   "nonexistent",
		UserID:    "user",
		SessionID: "session",
	})
	assert.Error(t, err)
}

func TestSessionService_List(t *testing.T) {
	service := sessionServiceWithData(t)
	ctx := context.Background()

	// List all sessions for app1
	resp, err := service.List(ctx, &session.ListRequest{
		AppName: "app1",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Sessions, 3) // user1/session1, user1/session2, user2/session1

	// List sessions for specific user
	resp, err = service.List(ctx, &session.ListRequest{
		AppName: "app1",
		UserID:  "user1",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Sessions, 2)
}

func TestSessionService_Delete(t *testing.T) {
	service := sessionServiceWithData(t)
	ctx := context.Background()

	// Delete existing session
	err := service.Delete(ctx, &session.DeleteRequest{
		AppName:   "app1",
		UserID:    "user1",
		SessionID: "session1",
	})
	require.NoError(t, err)

	// Verify it's gone
	_, err = service.Get(ctx, &session.GetRequest{
		AppName:   "app1",
		UserID:    "user1",
		SessionID: "session1",
	})
	assert.Error(t, err)
}

func TestSessionService_AppendEvent(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	// Create a session
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "event-test",
	})
	require.NoError(t, err)

	// Append an event
	event := &session.Event{
		Author: "test-user",
	}

	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Verify event was persisted
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "event-test",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, getResp.Session.Events().Len())
}

func TestSessionService_AppendEvent_SkipsPartial(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "partial-test",
	})
	require.NoError(t, err)

	// Append a partial event (should be skipped)
	event := &session.Event{
		Author: "test-user",
		LLMResponse: model.LLMResponse{
			Partial: true,
		},
	}

	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Verify no events were persisted
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "partial-test",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, getResp.Session.Events().Len())
}

func TestSessionService_AppendEvent_StateDelta(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-delta-test",
	})
	require.NoError(t, err)

	// Append event with state delta
	event := &session.Event{
		Author: "test-user",
		Actions: session.EventActions{
			StateDelta: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
		},
	}

	err = service.AppendEvent(ctx, createResp.Session, event)
	require.NoError(t, err)

	// Verify state was updated
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "state-delta-test",
	})
	require.NoError(t, err)

	val1, err := getResp.Session.State().Get("key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val1)
}

func TestSessionService_Get_NumRecentEvents(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	// Create session with multiple events
	createResp, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "recent-events-test",
	})
	require.NoError(t, err)

	// Add 5 events
	for i := 0; i < 5; i++ {
		event := &session.Event{
			Author: fmt.Sprintf("user-%d", i),
		}
		err = service.AppendEvent(ctx, createResp.Session, event)
		require.NoError(t, err)
	}

	// Get only 2 most recent events
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:         "test-app",
		UserID:          "user123",
		SessionID:       "recent-events-test",
		NumRecentEvents: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, getResp.Session.Events().Len())
}

func TestNewSessionService(t *testing.T) {
	tmpDir := t.TempDir()
	log := testLogger()

	// Test with local file provider
	provider := storage_manager.NewLocalFileProvider(tmpDir)
	service := NewSessionService(provider, log)
	require.NotNil(t, service)

	// Test with mock file provider
	mockProvider := mocks.NewFileProvider(t)
	service = NewSessionService(mockProvider, log)
	require.NotNil(t, service)

	// Test panic on nil provider
	assert.Panics(t, func() {
		NewSessionService(nil, log)
	})

	// Test panic on nil logger
	assert.Panics(t, func() {
		NewSessionService(provider, nil)
	})
}

func TestSessionService_StatePreservesNumbers(t *testing.T) {
	service := emptySessionService(t)
	ctx := context.Background()

	// Create session with numeric state
	_, err := service.Create(ctx, &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "number-test",
		State: map[string]any{
			"int_val":   42,
			"float_val": 3.14,
		},
	})
	require.NoError(t, err)

	// Get session and verify numbers are preserved
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "number-test",
	})
	require.NoError(t, err)

	intVal, err := getResp.Session.State().Get("int_val")
	require.NoError(t, err)
	assert.Equal(t, 42, intVal)

	floatVal, err := getResp.Session.State().Get("float_val")
	require.NoError(t, err)
	assert.Equal(t, 3.14, floatVal)
}

func TestSessionService_MockProvider(t *testing.T) {
	// Create mock file provider with in-memory storage
	mockProvider := mocks.NewFileProvider(t)
	storage := make(map[string][]byte)

	sessionPath := "test-app/user123/mock-test-session.json"

	// Set up mock to use in-memory storage
	mockProvider.EXPECT().Exists(mock.Anything, sessionPath).
		Return(false, nil).Once()
	mockProvider.EXPECT().Write(mock.Anything, sessionPath, mock.Anything).
		RunAndReturn(func(_ context.Context, path string, data []byte) error {
			storage[path] = data
			return nil
		}).Once()
	mockProvider.EXPECT().Exists(mock.Anything, sessionPath).
		Return(true, nil).Once()
	mockProvider.EXPECT().Read(mock.Anything, sessionPath).
		RunAndReturn(func(_ context.Context, path string) ([]byte, error) {
			data, ok := storage[path]
			if !ok {
				return nil, fmt.Errorf("file not found")
			}
			return data, nil
		}).Once()

	service := NewSessionService(mockProvider, testLogger())
	ctx := context.Background()

	// Test creating a session
	req := &session.CreateRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "mock-test-session",
	}

	resp, err := service.Create(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "mock-test-session", resp.Session.ID())

	// Test getting the session
	getResp, err := service.Get(ctx, &session.GetRequest{
		AppName:   "test-app",
		UserID:    "user123",
		SessionID: "mock-test-session",
	})
	require.NoError(t, err)
	assert.Equal(t, "mock-test-session", getResp.Session.ID())
}
