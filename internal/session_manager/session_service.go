package session_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/session"
)

// eventIDCounter ensures unique event IDs even when time.Now().UnixNano() returns the same value
var eventIDCounter atomic.Uint64

// SessionService implements the session.Service interface using JSON file storage.
type SessionService struct {
	fileProvider   storage_manager.FileProvider
	mutex          sync.RWMutex
	sessionLocks   map[string]*sync.Mutex // Per-session locks to prevent concurrent modifications
	sessionLockMux sync.Mutex             // Protects the sessionLocks map itself
	log            logger.Logger          // Logger for debugging
}

// SessionData represents the structure of session data stored in JSON.
type SessionData struct {
	AppName   string           `json:"app_name"`
	UserID    string           `json:"user_id"`
	SessionID string           `json:"session_id"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	State     map[string]any   `json:"state,omitempty"`  // Session state as key-value pairs
	Events    []*session.Event `json:"events,omitempty"` // Session events
}

// NewSessionService creates a new session service with the given file provider.
// The provider should be obtained from a StorageManager, typically with a
// "sessions" namespace prefix.
func NewSessionService(provider storage_manager.FileProvider, log logger.Logger) *SessionService {
	if provider == nil {
		panic("file provider cannot be nil")
	}
	if log == nil {
		panic("logger cannot be nil")
	}
	return &SessionService{
		fileProvider: provider,
		sessionLocks: make(map[string]*sync.Mutex),
		log:          log,
	}
}

// Create creates a new session.
func (s *SessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("create request cannot be nil")
	}

	if req.AppName == "" {
		return nil, fmt.Errorf("app name is required")
	}

	if req.UserID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Create session key for file storage
	sessionKey := s.getSessionKey(req.AppName, req.UserID, sessionID)

	// Check if session already exists - return error to match ADK behaviour
	exists, err := s.fileProvider.Exists(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check if session exists: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	// Create new session data
	now := time.Now()

	// Copy initial state from request
	initialState := make(map[string]any)
	if req.State != nil {
		for k, v := range req.State {
			initialState[k] = v
		}
	}

	sessionData := &SessionData{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
		State:     initialState,
		Events:    make([]*session.Event, 0),
	}

	// Save to file
	if err := s.saveSession(ctx, sessionKey, sessionData); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Convert to ADK session and return
	adkSession := s.sessionDataToADKSession(sessionData)
	return &session.CreateResponse{
		Session: adkSession,
	}, nil
}

// Get retrieves an existing session.
func (s *SessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("get request cannot be nil")
	}

	sessionKey := s.getSessionKey(req.AppName, req.UserID, req.SessionID)

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	s.log.Debug("Loading session from storage", logger.StringField("session_key", sessionKey))

	// Check if session exists before trying to load
	exists, err := s.fileProvider.Exists(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check session existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("session not found: %s (app: %s, user: %s)", req.SessionID, req.AppName, req.UserID)
	}

	// Load from file
	sessionData, err := s.loadSession(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Apply event filtering based on request parameters
	filteredEvents := s.filterEvents(sessionData.Events, req)

	// Create a copy of session data with filtered events
	filteredSessionData := &SessionData{
		AppName:   sessionData.AppName,
		UserID:    sessionData.UserID,
		SessionID: sessionData.SessionID,
		CreatedAt: sessionData.CreatedAt,
		UpdatedAt: sessionData.UpdatedAt,
		State:     sessionData.State,
		Events:    filteredEvents,
	}

	adkSession := s.sessionDataToADKSession(filteredSessionData)
	return &session.GetResponse{
		Session: adkSession,
	}, nil
}

// filterEvents applies filtering based on GetRequest parameters.
func (s *SessionService) filterEvents(events []*session.Event, req *session.GetRequest) []*session.Event {
	if events == nil {
		return nil
	}

	filteredEvents := events

	// Filter by NumRecentEvents - return only the N most recent events
	if req.NumRecentEvents > 0 && len(filteredEvents) > req.NumRecentEvents {
		start := len(filteredEvents) - req.NumRecentEvents
		filteredEvents = filteredEvents[start:]
	}

	// Filter by timestamp - return events with timestamp >= After
	// Assumes events are sorted by timestamp (which they should be since we append chronologically)
	if !req.After.IsZero() && len(filteredEvents) > 0 {
		// Find the first event that is not before the After timestamp
		firstIndexToKeep := 0
		for firstIndexToKeep < len(filteredEvents) {
			if !filteredEvents[firstIndexToKeep].Timestamp.Before(req.After) {
				break
			}
			firstIndexToKeep++
		}
		filteredEvents = filteredEvents[firstIndexToKeep:]
	}

	return filteredEvents
}

// List lists sessions matching the request criteria.
func (s *SessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("list request cannot be nil")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Build prefix for file listing
	var prefix string
	if req.UserID != "" {
		// For specific user, we want to match files in that user directory
		prefix = fmt.Sprintf("%s/%s/", req.AppName, req.UserID)
	} else {
		// For app-wide listing, include trailing slash to match subdirectories
		prefix = fmt.Sprintf("%s/", req.AppName)
	}

	// List files matching the prefix
	files, err := s.fileProvider.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list session files: %w", err)
	}

	// Pre-allocate with estimated capacity
	sessions := make([]session.Session, 0, len(files))
	for _, file := range files {
		// Load session data
		sessionData, err := s.loadSession(ctx, file)
		if err != nil {
			// Log error but continue with other sessions
			continue
		}

		// Apply additional filters if specified
		if req.UserID != "" && sessionData.UserID != req.UserID {
			continue
		}

		adkSession := s.sessionDataToADKSession(sessionData)
		sessions = append(sessions, adkSession)
	}

	return &session.ListResponse{
		Sessions: sessions,
	}, nil
}

// Delete removes a session.
func (s *SessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	if req == nil {
		return fmt.Errorf("delete request cannot be nil")
	}

	sessionKey := s.getSessionKey(req.AppName, req.UserID, req.SessionID)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Delete from file storage
	if err := s.fileProvider.Delete(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session %s (app: %s, user: %s): %w", req.SessionID, req.AppName, req.UserID, err)
	}

	return nil
}

// AppendEvent appends an event to a session.
//
//nolint:gocyclo,revive // Event handling requires multiple type assertions and state management
func (s *SessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	if sess == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Skip partial events - they should not be persisted
	if event.Partial {
		return nil
	}

	// Generate event ID if not set
	// Use atomic counter combined with timestamp to ensure uniqueness even under high concurrency
	if event.ID == "" {
		counter := eventIDCounter.Add(1)
		event.ID = fmt.Sprintf("event_%d_%d", time.Now().UnixNano(), counter)
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Update the in-memory session object - this is critical!
	// The runner reuses the same session object and expects events to be available
	// via sess.Events() on subsequent turns. Without this, the API call fails with
	// "messages: Field required" because the events list appears empty.
	if adkSess, ok := sess.(*adkSession); ok {
		// Update events in-memory
		if evts, ok := adkSess.events.(*sessionEvents); ok {
			evts.mutex.Lock()
			evts.events = append(evts.events, event)
			evts.mutex.Unlock()
		}
		// Update state in-memory (excluding temporary keys)
		if state, ok := adkSess.state.(*sessionState); ok {
			for key, value := range event.Actions.StateDelta {
				if !isTemporaryKey(key) {
					_ = state.Set(key, value)
				}
			}
		}
	}

	sessionKey := s.getSessionKey(sess.AppName(), sess.UserID(), sess.ID())
	s.log.Debug("Appending event to session", logger.StringField("session_key", sessionKey))

	// Acquire session-specific lock to prevent concurrent modifications to the same session
	sessionLock := s.getSessionLock(sessionKey)
	sessionLock.Lock()
	defer sessionLock.Unlock()

	// Load current session data from storage
	sessionData, err := s.loadSession(ctx, sessionKey)
	if err != nil {
		return fmt.Errorf("failed to load session for event append: %w", err)
	}

	// Initialise events slice if nil
	if sessionData.Events == nil {
		sessionData.Events = make([]*session.Event, 0)
	}

	// Apply state delta from the event to the session state
	// Exclude temporary keys (temp: prefix) from persistence
	if len(event.Actions.StateDelta) > 0 {
		if sessionData.State == nil {
			sessionData.State = make(map[string]any)
		}

		// Filter out temporary keys from the event's StateDelta before persisting
		filteredStateDelta := make(map[string]any)
		for key, value := range event.Actions.StateDelta {
			if !isTemporaryKey(key) {
				sessionData.State[key] = value
				filteredStateDelta[key] = value
			}
		}
		// Update the event's StateDelta to only contain non-temporary keys
		event.Actions.StateDelta = filteredStateDelta
	}

	// Append the event
	sessionData.Events = append(sessionData.Events, event)

	// Save the updated session
	if err := s.saveSession(ctx, sessionKey, sessionData); err != nil {
		return fmt.Errorf("failed to save session after event append: %w", err)
	}

	return nil
}

// isTemporaryKey checks if a state key is temporary (should not be persisted).
func isTemporaryKey(key string) bool {
	return len(key) >= len(session.KeyPrefixTemp) && key[:len(session.KeyPrefixTemp)] == session.KeyPrefixTemp
}

// Helper methods

// getSessionLock returns a session-specific lock, creating it if necessary.
func (s *SessionService) getSessionLock(sessionKey string) *sync.Mutex {
	s.sessionLockMux.Lock()
	defer s.sessionLockMux.Unlock()

	if lock, exists := s.sessionLocks[sessionKey]; exists {
		return lock
	}

	// Create new lock for this session
	lock := &sync.Mutex{}
	s.sessionLocks[sessionKey] = lock
	return lock
}

// getSessionKey generates a consistent key for session storage.
func (s *SessionService) getSessionKey(appName, userID, sessionID string) string {
	if sessionID == "" {
		return fmt.Sprintf("%s/%s/", appName, userID)
	}
	return fmt.Sprintf("%s/%s/%s.json", appName, userID, sessionID)
}

// loadSession loads session data from file storage.
func (s *SessionService) loadSession(ctx context.Context, sessionKey string) (*SessionData, error) {
	start := time.Now()
	data, err := s.fileProvider.Read(ctx, sessionKey)
	if err != nil {
		s.log.Warn("Failed to read session from storage",
			logger.StringField("session_key", sessionKey),
			logger.ErrorField(err))
		return nil, err
	}

	var sessionData SessionData
	// Use a decoder with UseNumber to preserve number precision
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	if err := decoder.Decode(&sessionData); err != nil {
		s.log.Error("Failed to unmarshal session data",
			logger.StringField("session_key", sessionKey),
			logger.ErrorField(err))
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	// Convert json.Number values back to appropriate types in State
	if sessionData.State != nil {
		convertJSONNumbers(sessionData.State)
	}

	s.log.Info("Loaded session from storage",
		logger.StringField("session_key", sessionKey),
		logger.IntField("events_count", len(sessionData.Events)),
		logger.DurationField("duration", time.Since(start)))

	return &sessionData, nil
}

// saveSession saves session data to file storage.
func (s *SessionService) saveSession(ctx context.Context, sessionKey string, sessionData *SessionData) error {
	start := time.Now()

	// Update timestamp
	sessionData.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		s.log.Error("Failed to marshal session data",
			logger.StringField("session_key", sessionKey),
			logger.ErrorField(err))
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	if err := s.fileProvider.Write(ctx, sessionKey, data); err != nil {
		s.log.Error("Failed to write session to storage",
			logger.StringField("session_key", sessionKey),
			logger.ErrorField(err))
		return fmt.Errorf("failed to write session file: %w", err)
	}

	s.log.Info("Saved session to storage",
		logger.StringField("session_key", sessionKey),
		logger.IntField("events_count", len(sessionData.Events)),
		logger.IntField("size_bytes", len(data)),
		logger.DurationField("duration", time.Since(start)))

	return nil
}

// sessionDataToADKSession converts internal session data to ADK session interface.
// Creates defensive copies of state and events to prevent external modifications.
func (s *SessionService) sessionDataToADKSession(data *SessionData) session.Session {
	// Initialise state if nil
	if data.State == nil {
		data.State = make(map[string]any)
	}

	// Initialise events if nil
	if data.Events == nil {
		data.Events = make([]*session.Event, 0)
	}

	// Create defensive copies to prevent external modifications
	stateCopy := make(map[string]any, len(data.State))
	for k, v := range data.State {
		stateCopy[k] = v
	}

	eventsCopy := make([]*session.Event, len(data.Events))
	copy(eventsCopy, data.Events)

	return &adkSession{
		appName:        data.AppName,
		userID:         data.UserID,
		sessionID:      data.SessionID,
		createdAt:      data.CreatedAt,
		lastUpdateTime: data.UpdatedAt,
		state:          &sessionState{data: stateCopy},
		events:         &sessionEvents{events: eventsCopy},
	}
}

// adkSession implements the session.Session interface.
type adkSession struct {
	appName        string
	userID         string
	sessionID      string
	createdAt      time.Time
	lastUpdateTime time.Time
	state          session.State
	events         session.Events
}

// AppName returns the application name.
func (s *adkSession) AppName() string {
	return s.appName
}

// UserID returns the user ID.
func (s *adkSession) UserID() string {
	return s.userID
}

// ID returns the session ID.
func (s *adkSession) ID() string {
	return s.sessionID
}

// State returns the session state.
func (s *adkSession) State() session.State {
	return s.state
}

// Events returns the session events.
func (s *adkSession) Events() session.Events {
	return s.events
}

// LastUpdateTime returns when the session was last updated.
func (s *adkSession) LastUpdateTime() time.Time {
	return s.lastUpdateTime
}

// generateSessionID creates a new unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// sessionState implements the session.State interface.
// IMPORTANT: Changes made via Set() are NOT persisted to storage.
// State changes must be made through event.Actions.StateDelta for persistence.
// This is a read-only view with in-memory modification capability for the current session lifecycle.
type sessionState struct {
	data  map[string]any
	mutex sync.RWMutex
}

// Get retrieves the value associated with a given key.
func (s *sessionState) Get(key string) (any, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key %s does not exist", key)
	}

	return value, nil
}

// Set assigns the given value to the given key.
// WARNING: This change is NOT persisted to storage. It only modifies the in-memory state.
// To persist state changes, use event.Actions.StateDelta when appending events.
func (s *sessionState) Set(key string, value any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.data == nil {
		s.data = make(map[string]any)
	}

	s.data[key] = value
	return nil
}

// All returns an iterator over all key-value pairs.
func (s *sessionState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		s.mutex.RLock()
		defer s.mutex.RUnlock()

		for key, value := range s.data {
			if !yield(key, value) {
				return
			}
		}
	}
}

// sessionEvents implements the session.Events interface.
type sessionEvents struct {
	events []*session.Event
	mutex  sync.RWMutex
}

// All returns an iterator over all events.
func (e *sessionEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		e.mutex.RLock()
		defer e.mutex.RUnlock()

		for _, event := range e.events {
			if !yield(event) {
				return
			}
		}
	}
}

// Len returns the total number of events.
func (e *sessionEvents) Len() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return len(e.events)
}

// At returns the event at the specified index.
func (e *sessionEvents) At(i int) *session.Event {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if i < 0 || i >= len(e.events) {
		return nil
	}

	return e.events[i]
}

// convertJSONNumbers converts json.Number values in a map back to their appropriate Go types.
func convertJSONNumbers(m map[string]any) {
	for key, value := range m {
		switch v := value.(type) {
		case json.Number:
			// Try to convert to int first, then float64
			if intVal, err := v.Int64(); err == nil {
				// Check if it fits in a regular int
				if intVal >= int64(int(^uint(0)>>1)*-1) && intVal <= int64(int(^uint(0)>>1)) {
					m[key] = int(intVal)
				} else {
					m[key] = intVal
				}
			} else if floatVal, err := v.Float64(); err == nil {
				m[key] = floatVal
			}
			// If both conversions fail, keep as json.Number
		case map[string]any:
			// Recursively handle nested maps
			convertJSONNumbers(v)
		}
	}
}
