package session

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/session"
)

// JSONSessionService implements the session.Service interface using JSON file storage
type JSONSessionService struct {
	fileProvider FileProvider
	mutex        sync.RWMutex
	cache        map[string]*SessionData // In-memory cache for performance
	log          logger.Logger           // Logger for debugging
}

// SessionData represents the structure of session data stored in JSON
type SessionData struct {
	AppName   string                 `json:"app_name"`
	UserID    string                 `json:"user_id"`
	SessionID string                 `json:"session_id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	State     map[string]interface{} `json:"state,omitempty"`  // Session state as key-value pairs
	Events    []*session.Event       `json:"events,omitempty"` // Session events
}

// NewJSONSessionService creates a new JSON-based session service
func NewJSONSessionService(fileProvider FileProvider, log logger.Logger) *JSONSessionService {
	return &JSONSessionService{
		fileProvider: fileProvider,
		cache:        make(map[string]*SessionData),
		log:          log,
	}
}

// Create creates a new session
func (s *JSONSessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
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

	// Check if session already exists
	exists, err := s.fileProvider.Exists(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check if session exists: %w", err)
	}

	if exists {
		// Session already exists, load and return it
		sessionData, err := s.loadSession(ctx, sessionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing session: %w", err)
		}

		adkSession := s.sessionDataToADKSession(sessionData)
		return &session.CreateResponse{
			Session: adkSession,
		}, nil
	}

	// Create new session data
	now := time.Now()
	sessionData := &SessionData{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
		State:     make(map[string]interface{}),
		Events:    make([]*session.Event, 0),
	}

	// Save to file
	if err := s.saveSession(ctx, sessionKey, sessionData); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Add to cache
	s.cache[sessionKey] = sessionData

	// Convert to ADK session and return
	adkSession := s.sessionDataToADKSession(sessionData)
	return &session.CreateResponse{
		Session: adkSession,
	}, nil
}

// Get retrieves an existing session
func (s *JSONSessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("get request cannot be nil")
	}

	sessionKey := s.getSessionKey(req.AppName, req.UserID, req.SessionID)

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Try cache first
	if sessionData, exists := s.cache[sessionKey]; exists {
		s.log.Debug("Session cache hit", logger.StringField("session_key", sessionKey))
		adkSession := s.sessionDataToADKSession(sessionData)
		return &session.GetResponse{
			Session: adkSession,
		}, nil
	}

	s.log.Debug("Session cache miss, loading from storage", logger.StringField("session_key", sessionKey))

	// Load from file
	sessionData, err := s.loadSession(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Add to cache
	s.cache[sessionKey] = sessionData

	adkSession := s.sessionDataToADKSession(sessionData)
	return &session.GetResponse{
		Session: adkSession,
	}, nil
}

// List lists sessions matching the request criteria
func (s *JSONSessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("list request cannot be nil")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Build prefix for file listing
	prefix := req.AppName
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

	var sessions []session.Session
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

// Delete removes a session
func (s *JSONSessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	if req == nil {
		return fmt.Errorf("delete request cannot be nil")
	}

	sessionKey := s.getSessionKey(req.AppName, req.UserID, req.SessionID)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Remove from cache
	delete(s.cache, sessionKey)

	// Delete from file storage
	if err := s.fileProvider.Delete(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// AppendEvent appends an event to a session
func (s *JSONSessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	if sess == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	sessionKey := s.getSessionKey(sess.AppName(), sess.UserID(), sess.ID())
	s.log.Debug("Appending event to session", logger.StringField("session_key", sessionKey))

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Load current session data
	var sessionData *SessionData
	var err error

	// Try cache first
	if cachedData, exists := s.cache[sessionKey]; exists {
		sessionData = cachedData
	} else {
		// Load from file
		sessionData, err = s.loadSession(ctx, sessionKey)
		if err != nil {
			return fmt.Errorf("failed to load session for event append: %w", err)
		}
		s.cache[sessionKey] = sessionData
	}

	// Initialize events slice if nil
	if sessionData.Events == nil {
		sessionData.Events = make([]*session.Event, 0)
	}

	// Generate event ID if not set
	if event.ID == "" {
		event.ID = fmt.Sprintf("event_%d", time.Now().UnixNano())
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Append the event
	sessionData.Events = append(sessionData.Events, event)

	// Save the updated session
	if err := s.saveSession(ctx, sessionKey, sessionData); err != nil {
		return fmt.Errorf("failed to save session after event append: %w", err)
	}

	return nil
}

// Helper methods

// getSessionKey generates a consistent key for session storage
func (s *JSONSessionService) getSessionKey(appName, userID, sessionID string) string {
	if sessionID == "" {
		return fmt.Sprintf("%s/%s/", appName, userID)
	}
	return fmt.Sprintf("%s/%s/%s.json", appName, userID, sessionID)
}

// loadSession loads session data from file storage
func (s *JSONSessionService) loadSession(ctx context.Context, sessionKey string) (*SessionData, error) {
	start := time.Now()
	data, err := s.fileProvider.Read(ctx, sessionKey)
	if err != nil {
		s.log.Warn("Failed to read session from storage",
			logger.StringField("session_key", sessionKey),
			logger.ErrorField(err))
		return nil, err
	}

	var sessionData SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		s.log.Error("Failed to unmarshal session data",
			logger.StringField("session_key", sessionKey),
			logger.ErrorField(err))
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	s.log.Info("Loaded session from storage",
		logger.StringField("session_key", sessionKey),
		logger.IntField("events_count", len(sessionData.Events)),
		logger.DurationField("duration", time.Since(start)))

	return &sessionData, nil
}

// saveSession saves session data to file storage
func (s *JSONSessionService) saveSession(ctx context.Context, sessionKey string, sessionData *SessionData) error {
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

// sessionDataToADKSession converts internal session data to ADK session interface
func (s *JSONSessionService) sessionDataToADKSession(data *SessionData) session.Session {
	// Initialize state if nil
	if data.State == nil {
		data.State = make(map[string]interface{})
	}

	// Initialize events if nil
	if data.Events == nil {
		data.Events = make([]*session.Event, 0)
	}

	return &adkSession{
		appName:        data.AppName,
		userID:         data.UserID,
		sessionID:      data.SessionID,
		createdAt:      data.CreatedAt,
		lastUpdateTime: data.UpdatedAt,
		state:          &sessionState{data: data.State},
		events:         &sessionEvents{events: data.Events},
	}
}

// adkSession implements the session.Session interface
type adkSession struct {
	appName        string
	userID         string
	sessionID      string
	createdAt      time.Time
	lastUpdateTime time.Time
	state          session.State
	events         session.Events
}

// AppName returns the application name
func (s *adkSession) AppName() string {
	return s.appName
}

// UserID returns the user ID
func (s *adkSession) UserID() string {
	return s.userID
}

// ID returns the session ID
func (s *adkSession) ID() string {
	return s.sessionID
}

// State returns the session state
func (s *adkSession) State() session.State {
	return s.state
}

// Events returns the session events
func (s *adkSession) Events() session.Events {
	return s.events
}

// LastUpdateTime returns when the session was last updated
func (s *adkSession) LastUpdateTime() time.Time {
	return s.lastUpdateTime
}

// generateSessionID creates a new unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// sessionState implements the session.State interface
type sessionState struct {
	data  map[string]interface{}
	mutex sync.RWMutex
}

// Get retrieves the value associated with a given key
func (s *sessionState) Get(key string) (any, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key %s does not exist", key) // TODO: Use proper ErrStateKeyNotExist
	}

	return value, nil
}

// Set assigns the given value to the given key
func (s *sessionState) Set(key string, value any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.data == nil {
		s.data = make(map[string]interface{})
	}

	s.data[key] = value
	return nil
}

// All returns an iterator over all key-value pairs
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

// sessionEvents implements the session.Events interface
type sessionEvents struct {
	events []*session.Event
	mutex  sync.RWMutex
}

// All returns an iterator over all events
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

// Len returns the total number of events
func (e *sessionEvents) Len() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return len(e.events)
}

// At returns the event at the specified index
func (e *sessionEvents) At(i int) *session.Event {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if i < 0 || i >= len(e.events) {
		return nil
	}

	return e.events[i]
}
