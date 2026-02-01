package session

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Test helpers
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

func Test_Service_Create(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) session.Service
		req     *session.CreateRequest
		wantErr bool
	}{
		{
			name:  "full key",
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
			name:  "generated session id",
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
			name:  "when already exists, it returns error",
			setup: serviceWithData,
			req: &session.CreateRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
				State: map[string]any{
					"k": 10,
				},
			},
			wantErr: true, // ADK-compatible behavior: return error on duplicate
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

func Test_Service_Delete(t *testing.T) {
	tests := []struct {
		name    string
		req     *session.DeleteRequest
		setup   func(t *testing.T) session.Service
		wantErr bool
	}{
		{
			name:  "delete ok",
			setup: serviceWithData,
			req: &session.DeleteRequest{
				AppName:   "app1",
				UserID:    "user1",
				SessionID: "session1",
			},
		},
		{
			name:  "no error when not found",
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

func Test_Service_Get(t *testing.T) {
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
			name:  "error when not found",
			setup: serviceWithData,
			req: &session.GetRequest{
				AppName:   "testApp",
				UserID:    "user1",
				SessionID: "session1",
			},
			wantErr: true,
		},
		{
			name:  "get session respects user id",
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
			name:  "with config_no config returns all events",
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
			name:  "with config_num recent events",
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
			name:  "with config_after timestamp",
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
			name:  "with config_combined filters",
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
					cmpopts.IgnoreFields(session.Event{}, "Timestamp"), // Ignore timestamp differences
				}

				// Convert to slice for comparison
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

func Test_Service_List(t *testing.T) {
	tests := []struct {
		name      string
		req       *session.ListRequest
		setup     func(t *testing.T) session.Service
		wantCount int
		wantErr   bool
	}{
		{
			name:  "list for user1",
			setup: serviceWithData,
			req: &session.ListRequest{
				AppName: "app1",
				UserID:  "user1",
			},
			wantCount: 2,
		},
		{
			name:  "empty list for non-existent user",
			setup: serviceWithData,
			req: &session.ListRequest{
				AppName: "app1",
				UserID:  "custom_user",
			},
			wantCount: 0,
		},
		{
			name:  "list for user2",
			setup: serviceWithData,
			req: &session.ListRequest{
				AppName: "app1",
				UserID:  "user2",
			},
			wantCount: 1,
		},
		{
			name:      "list all users for app",
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

func Test_Service_AppendEvent(t *testing.T) {
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
			name:  "append event to the session",
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
			name:  "append event with state delta",
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
			name:  "partial events are not persisted",
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

			// Create session
			created, err := s.Create(ctx, tt.sessionReq)
			require.NoError(t, err)

			// Append event
			err = s.AppendEvent(ctx, created.Session, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.AppendEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			// Get session and verify
			resp, err := s.Get(ctx, &session.GetRequest{
				AppName:   tt.sessionReq.AppName,
				UserID:    tt.sessionReq.UserID,
				SessionID: tt.sessionReq.SessionID,
			})
			require.NoError(t, err)

			// Check event count
			if resp.Session.Events().Len() != tt.wantEventCount {
				t.Errorf("AppendEvent returned %d events, want %d", resp.Session.Events().Len(), tt.wantEventCount)
			}

			// Check state
			gotState := maps.Collect(resp.Session.State().All())
			if diff := cmp.Diff(tt.wantState, gotState); diff != "" {
				t.Errorf("AppendEvent state mismatch: (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_Service_StateManagement(t *testing.T) {
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

		s1_got, err := s.Get(ctx, &session.GetRequest{AppName: appName, UserID: "u1", SessionID: "s1"})
		require.NoError(t, err)

		wantState := map[string]any{"sk1": "v1", "sk2": "v2"}
		gotState := maps.Collect(s1_got.Session.State().All())
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

		s1_got, err := s.Get(ctx, &session.GetRequest{AppName: appName, UserID: "u1", SessionID: "s1"})
		require.NoError(t, err)

		wantState := map[string]any{"sk": "v2"}
		gotState := maps.Collect(s1_got.Session.State().All())
		if diff := cmp.Diff(wantState, gotState); diff != "" {
			t.Errorf("Persisted state mismatch (-want +got):\n%s", diff)
		}

		// Verify temp key is not in stored events
		storedEvents := s1_got.Session.Events()
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
