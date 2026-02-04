// Package skills_manager provides skill management for the chatbot.
package skills_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/tool"
)

// Manager provides skill tracking and lifecycle management
type Manager interface {
	// SearchSkills searches for skills by query string (substring match on name/description)
	// Use "*" to return all skills
	SearchSkills(ctx context.Context, query string) ([]Skill, error)

	// RetrieveSkill retrieves a skill by exact name
	RetrieveSkill(ctx context.Context, name string) (*Skill, error)

	// UpsertSkill creates or updates a skill
	UpsertSkill(ctx context.Context, skill Skill) error

	// Tools returns all ADK tools for skill management, pre-configured with this manager
	Tools() ([]tool.Tool, error)
}

// skillsManager implements the Manager interface
type skillsManager struct {
	config Config
	mutex  sync.RWMutex
	skills map[string]Skill // name -> skill
}

// New creates a new skills manager instance
func New(config Config) (Manager, error) {
	if config.FileProvider == nil {
		return nil, fmt.Errorf("file provider is required")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	sm := &skillsManager{
		config: config,
		skills: make(map[string]Skill),
	}

	// Load existing skills from file provider
	if err := sm.loadSkills(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load skills: %w", err)
	}

	return sm, nil
}

// loadSkills discovers and loads all skills from the file provider
func (sm *skillsManager) loadSkills(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// List all JSON files in the provider
	files, err := sm.config.FileProvider.List(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list skill files: %w", err)
	}

	for _, file := range files {
		// Only process .json files
		if !strings.HasSuffix(file, ".json") {
			continue
		}

		data, err := sm.config.FileProvider.Read(ctx, file)
		if err != nil {
			sm.config.Logger.Warn("Failed to read skill file",
				logger.StringField("file", file),
				logger.ErrorField(err))
			continue
		}

		var skill Skill
		if err := json.Unmarshal(data, &skill); err != nil {
			sm.config.Logger.Warn("Failed to unmarshal skill file",
				logger.StringField("file", file),
				logger.ErrorField(err))
			continue
		}

		sm.skills[skill.Name] = skill
	}

	sm.config.Logger.Info("Loaded skills",
		logger.IntField("count", len(sm.skills)))

	return nil
}

// skillFileName returns the file name for a skill
func skillFileName(name string) string {
	return name + ".json"
}

// SearchSkills searches for skills by query string
func (sm *skillsManager) SearchSkills(ctx context.Context, query string) ([]Skill, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var results []Skill

	// Special case: "*" returns all skills
	if query == "*" {
		for _, skill := range sm.skills {
			results = append(results, skill)
		}
		return results, nil
	}

	// Substring match on name or description (case-insensitive)
	queryLower := strings.ToLower(query)
	for _, skill := range sm.skills {
		nameLower := strings.ToLower(skill.Name)
		descLower := strings.ToLower(skill.Description)

		if strings.Contains(nameLower, queryLower) || strings.Contains(descLower, queryLower) {
			results = append(results, skill)
		}
	}

	return results, nil
}

// RetrieveSkill retrieves a skill by exact name
func (sm *skillsManager) RetrieveSkill(ctx context.Context, name string) (*Skill, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	skill, exists := sm.skills[name]
	if !exists {
		return nil, nil
	}

	return &skill, nil
}

// UpsertSkill creates or updates a skill
func (sm *skillsManager) UpsertSkill(ctx context.Context, skill Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	// Persist to file
	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal skill: %w", err)
	}

	if err := sm.config.FileProvider.Write(ctx, skillFileName(skill.Name), data); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	// Update cache
	sm.mutex.Lock()
	sm.skills[skill.Name] = skill
	sm.mutex.Unlock()

	sm.config.Logger.Info("Upserted skill",
		logger.StringField("name", skill.Name))

	return nil
}

// Tools returns all ADK tools for skill management, pre-configured with this manager
func (sm *skillsManager) Tools() ([]tool.Tool, error) {
	var tools []tool.Tool

	searchTool, err := sm.createSearchTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create search_skills tool: %w", err)
	}
	tools = append(tools, searchTool)

	retrieveTool, err := sm.createRetrieveTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create retrieve_skill tool: %w", err)
	}
	tools = append(tools, retrieveTool)

	upsertTool, err := sm.createUpsertTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create upsert_skill tool: %w", err)
	}
	tools = append(tools, upsertTool)

	return tools, nil
}
