package memory_service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
)

// Service implements the ADK memory.Service interface with persistent storage.
type Service struct {
	fileProvider storage_manager.FileProvider
	userLocks    map[string]*sync.Mutex // Per-user locks
	userLockMux  sync.Mutex
	log          logger.Logger
}

// Config holds configuration for the memory service.
type Config struct {
	FileProvider storage_manager.FileProvider
	Logger       logger.Logger
}

// New creates a new memory service with the given configuration.
func New(cfg Config) *Service {
	if cfg.FileProvider == nil {
		panic("file provider cannot be nil")
	}
	if cfg.Logger == nil {
		panic("logger cannot be nil")
	}

	return &Service{
		fileProvider: cfg.FileProvider,
		userLocks:    make(map[string]*sync.Mutex),
		log:          cfg.Logger,
	}
}

// AddSession adds a session to the memory service.
// A session can be added multiple times during its lifetime.
func (s *Service) AddSession(ctx context.Context, sess session.Session) error {
	if sess == nil {
		return fmt.Errorf("session cannot be nil")
	}

	// Extract entries from session events
	entries := s.extractEntries(sess)
	if len(entries) == 0 {
		s.log.Debug("No memory entries to add from session",
			logger.StringField("session_id", sess.ID()))
		return nil
	}

	appName := sess.AppName()
	userID := sess.UserID()
	sessionID := sess.ID()

	// Acquire user-specific lock
	userLock := s.getUserLock(appName, userID)
	userLock.Lock()
	defer userLock.Unlock()

	// Build memory data
	memData := MemoryData{
		SessionID: sessionID,
		AppName:   appName,
		UserID:    userID,
		UpdatedAt: time.Now(),
		Entries:   entries,
	}

	// Persist memory data
	memPath := s.memoryPath(appName, userID, sessionID)
	if err := s.writeJSON(ctx, memPath, memData); err != nil {
		return fmt.Errorf("failed to write memory data: %w", err)
	}

	// Update word index
	if err := s.updateWordIndex(ctx, appName, userID, sessionID, entries); err != nil {
		s.log.Warn("Failed to update word index",
			logger.StringField("session_id", sessionID),
			logger.ErrorField(err))
		// Don't fail the whole operation if index update fails
	}

	s.log.Info("Added session to memory",
		logger.StringField("session_id", sessionID),
		logger.IntField("entries_count", len(entries)))

	return nil
}

// Search returns memory entries relevant to the given query.
// Empty slice is returned if there are no matches.
func (s *Service) Search(ctx context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	if req == nil {
		return &memory.SearchResponse{}, nil
	}

	queryWords := extractWords(req.Query)
	if len(queryWords) == 0 {
		return &memory.SearchResponse{}, nil
	}

	// Load word index
	index, err := s.loadWordIndex(ctx, req.AppName, req.UserID)
	if err != nil {
		s.log.Debug("No word index found",
			logger.StringField("app_name", req.AppName),
			logger.StringField("user_id", req.UserID))
		return &memory.SearchResponse{}, nil
	}

	// Find matching session IDs
	sessionIDs := s.findMatchingSessions(index, queryWords)
	if len(sessionIDs) == 0 {
		return &memory.SearchResponse{}, nil
	}

	// Load and filter entries from matching sessions
	var memories []memory.Entry
	for _, sessionID := range sessionIDs {
		entries, err := s.loadSessionMemories(ctx, req.AppName, req.UserID, sessionID, queryWords)
		if err != nil {
			s.log.Debug("Failed to load session memories",
				logger.StringField("session_id", sessionID),
				logger.ErrorField(err))
			continue
		}
		memories = append(memories, entries...)
	}

	s.log.Debug("Memory search completed",
		logger.StringField("query", req.Query),
		logger.IntField("results_count", len(memories)))

	return &memory.SearchResponse{Memories: memories}, nil
}

// extractEntries extracts memory entries from session events.
func (s *Service) extractEntries(sess session.Session) []MemoryEntry {
	entries := make([]MemoryEntry, 0, sess.Events().Len())

	for event := range sess.Events().All() {
		// Only process LLM responses with content
		// LLMResponse is embedded directly in Event, so check Content
		if event.Content == nil {
			continue
		}

		content := event.Content

		// Extract words from content
		text := extractTextFromContent(content)
		words := extractWords(text)
		if len(words) == 0 {
			continue
		}

		entries = append(entries, MemoryEntry{
			Content:   contentToData(content),
			Author:    event.Author,
			Timestamp: event.Timestamp,
			Words:     wordsToSlice(words),
		})
	}

	return entries
}

// updateWordIndex updates the word index with entries from a session.
func (s *Service) updateWordIndex(ctx context.Context, appName, userID, sessionID string, entries []MemoryEntry) error {
	indexPath := s.indexPath(appName, userID)

	// Load existing index or create new one
	index, err := s.loadWordIndex(ctx, appName, userID)
	if err != nil {
		index = &WordIndex{
			AppName:   appName,
			UserID:    userID,
			UpdatedAt: time.Now(),
			Words:     make(map[string][]string),
		}
	}

	// Collect all unique words from entries
	allWords := make(map[string]struct{})
	for _, entry := range entries {
		for _, word := range entry.Words {
			allWords[word] = struct{}{}
		}
	}

	// Update index with new words
	for word := range allWords {
		// Check if session already indexed for this word
		sessions := index.Words[word]
		found := false
		for _, sid := range sessions {
			if sid == sessionID {
				found = true
				break
			}
		}
		if !found {
			index.Words[word] = append(sessions, sessionID)
		}
	}

	index.UpdatedAt = time.Now()

	// Save index
	return s.writeJSON(ctx, indexPath, index)
}

// loadWordIndex loads the word index for a user.
func (s *Service) loadWordIndex(ctx context.Context, appName, userID string) (*WordIndex, error) {
	indexPath := s.indexPath(appName, userID)

	data, err := s.fileProvider.Read(ctx, indexPath)
	if err != nil {
		return nil, err
	}

	var index WordIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal word index: %w", err)
	}

	return &index, nil
}

// findMatchingSessions finds session IDs that contain any of the query words.
func (s *Service) findMatchingSessions(index *WordIndex, queryWords map[string]struct{}) []string {
	sessionSet := make(map[string]struct{})

	for word := range queryWords {
		if sessions, ok := index.Words[word]; ok {
			for _, sid := range sessions {
				sessionSet[sid] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(sessionSet))
	for sid := range sessionSet {
		result = append(result, sid)
	}

	return result
}

// loadSessionMemories loads and filters memory entries from a session.
func (s *Service) loadSessionMemories(
	ctx context.Context,
	appName, userID, sessionID string,
	queryWords map[string]struct{},
) ([]memory.Entry, error) {
	memPath := s.memoryPath(appName, userID, sessionID)

	data, err := s.fileProvider.Read(ctx, memPath)
	if err != nil {
		return nil, err
	}

	var memData MemoryData
	if err := json.Unmarshal(data, &memData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal memory data: %w", err)
	}

	var entries []memory.Entry
	for _, entry := range memData.Entries {
		// Check if entry matches query
		entryWords := sliceToWords(entry.Words)
		if checkMapsIntersect(entryWords, queryWords) {
			entries = append(entries, memory.Entry{
				Content:   dataToContent(entry.Content),
				Author:    entry.Author,
				Timestamp: entry.Timestamp,
			})
		}
	}

	return entries, nil
}

// getUserLock returns a user-specific lock, creating it if necessary.
func (s *Service) getUserLock(appName, userID string) *sync.Mutex {
	key := fmt.Sprintf("%s/%s", appName, userID)

	s.userLockMux.Lock()
	defer s.userLockMux.Unlock()

	if lock, exists := s.userLocks[key]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	s.userLocks[key] = lock
	return lock
}

// memoryPath returns the storage path for session memory data.
func (s *Service) memoryPath(appName, userID, sessionID string) string {
	return fmt.Sprintf("memories/%s/%s/%s.json", appName, userID, sessionID)
}

// indexPath returns the storage path for the word index.
func (s *Service) indexPath(appName, userID string) string {
	return fmt.Sprintf("index/%s/%s/words.json", appName, userID)
}

// writeJSON writes data as JSON to the file provider.
func (s *Service) writeJSON(ctx context.Context, path string, data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return s.fileProvider.Write(ctx, path, jsonData)
}
