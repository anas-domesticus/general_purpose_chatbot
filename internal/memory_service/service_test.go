package memory_service

import (
	"context"
	"io"
	"iter"
	"testing"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func newTestLogger() logger.Logger {
	return logger.NewLogger(logger.Config{
		Level:  logger.DebugLevel,
		Output: io.Discard,
	})
}

func TestNew(t *testing.T) {
	provider := storage_manager.NewLocalFileProvider(t.TempDir())
	log := newTestLogger()

	svc := New(Config{
		FileProvider: provider,
		Logger:       log,
	})

	assert.NotNil(t, svc)
}

func TestAddSessionAndSearch(t *testing.T) {
	provider := storage_manager.NewLocalFileProvider(t.TempDir())
	log := newTestLogger()

	svc := New(Config{
		FileProvider: provider,
		Logger:       log,
	})

	ctx := context.Background()

	// Create a mock session with events
	mockSession := &mockSession{
		appName:   "testapp",
		userID:    "user123",
		sessionID: "session456",
		events: []*session.Event{
			{
				Author:    "assistant",
				Timestamp: time.Now(),
			},
		},
	}
	// Set content on the embedded LLMResponse
	mockSession.events[0].Content = genai.NewContentFromText("Hello world, this is a test response about weather", "model")

	// Add session to memory
	err := svc.AddSession(ctx, mockSession)
	require.NoError(t, err)

	// Search for matching content
	resp, err := svc.Search(ctx, &memory.SearchRequest{
		Query:   "weather",
		AppName: "testapp",
		UserID:  "user123",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Memories, 1)
	assert.Equal(t, "assistant", resp.Memories[0].Author)

	// Search for non-matching content
	resp, err = svc.Search(ctx, &memory.SearchRequest{
		Query:   "nonexistent",
		AppName: "testapp",
		UserID:  "user123",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Memories, 0)
}

func TestSearchEmptyQuery(t *testing.T) {
	provider := storage_manager.NewLocalFileProvider(t.TempDir())
	log := newTestLogger()

	svc := New(Config{
		FileProvider: provider,
		Logger:       log,
	})

	ctx := context.Background()

	// Search with empty query
	resp, err := svc.Search(ctx, &memory.SearchRequest{
		Query:   "",
		AppName: "testapp",
		UserID:  "user123",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Memories, 0)
}

func TestSearchNoIndex(t *testing.T) {
	provider := storage_manager.NewLocalFileProvider(t.TempDir())
	log := newTestLogger()

	svc := New(Config{
		FileProvider: provider,
		Logger:       log,
	})

	ctx := context.Background()

	// Search when no index exists
	resp, err := svc.Search(ctx, &memory.SearchRequest{
		Query:   "test",
		AppName: "testapp",
		UserID:  "user123",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Memories, 0)
}

func TestExtractWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]struct{}
	}{
		{
			name:  "simple words",
			input: "hello world",
			expected: map[string]struct{}{
				"hello": {},
				"world": {},
			},
		},
		{
			name:  "with punctuation",
			input: "Hello, World! How are you?",
			expected: map[string]struct{}{
				"hello": {},
				"world": {},
				"how":   {},
				"are":   {},
				"you":   {},
			},
		},
		{
			name:  "skip single chars",
			input: "a b c test",
			expected: map[string]struct{}{
				"test": {},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractWords(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckMapsIntersect(t *testing.T) {
	tests := []struct {
		name     string
		m1       map[string]struct{}
		m2       map[string]struct{}
		expected bool
	}{
		{
			name:     "intersecting",
			m1:       map[string]struct{}{"a": {}, "b": {}},
			m2:       map[string]struct{}{"b": {}, "c": {}},
			expected: true,
		},
		{
			name:     "non-intersecting",
			m1:       map[string]struct{}{"a": {}, "b": {}},
			m2:       map[string]struct{}{"c": {}, "d": {}},
			expected: false,
		},
		{
			name:     "empty first",
			m1:       map[string]struct{}{},
			m2:       map[string]struct{}{"a": {}},
			expected: false,
		},
		{
			name:     "empty second",
			m1:       map[string]struct{}{"a": {}},
			m2:       map[string]struct{}{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkMapsIntersect(tt.m1, tt.m2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockSession implements session.Session for testing
type mockSession struct {
	appName   string
	userID    string
	sessionID string
	events    []*session.Event
}

func (m *mockSession) AppName() string        { return m.appName }
func (m *mockSession) UserID() string         { return m.userID }
func (m *mockSession) ID() string             { return m.sessionID }
func (m *mockSession) State() session.State   { return nil }
func (m *mockSession) LastUpdateTime() time.Time { return time.Now() }

func (m *mockSession) Events() session.Events {
	return &mockEvents{events: m.events}
}

type mockEvents struct {
	events []*session.Event
}

func (e *mockEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, event := range e.events {
			if !yield(event) {
				return
			}
		}
	}
}

func (e *mockEvents) Len() int { return len(e.events) }

func (e *mockEvents) At(i int) *session.Event {
	if i < 0 || i >= len(e.events) {
		return nil
	}
	return e.events[i]
}
