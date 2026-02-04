package skills_manager

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager/mocks"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() logger.Logger {
	return logger.NewLogger(logger.Config{
		Level:  logger.ErrorLevel,
		Format: "text",
	})
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*mocks.FileProvider)
		config      func(*mocks.FileProvider) Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with empty skills",
			setupMock: func(m *mocks.FileProvider) {
				m.EXPECT().List(mock.Anything, "").Return([]string{}, nil)
			},
			config: func(m *mocks.FileProvider) Config {
				return Config{
					FileProvider: m,
					Logger:       testLogger(),
				}
			},
			expectError: false,
		},
		{
			name:      "missing file provider",
			setupMock: func(m *mocks.FileProvider) {},
			config: func(m *mocks.FileProvider) Config {
				return Config{
					Logger: testLogger(),
				}
			},
			expectError: true,
			errorMsg:    "file provider is required",
		},
		{
			name:      "missing logger",
			setupMock: func(m *mocks.FileProvider) {},
			config: func(m *mocks.FileProvider) Config {
				return Config{
					FileProvider: m,
				}
			},
			expectError: true,
			errorMsg:    "logger is required",
		},
		{
			name: "file provider list error",
			setupMock: func(m *mocks.FileProvider) {
				m.EXPECT().List(mock.Anything, "").Return(nil, errors.New("storage error"))
			},
			config: func(m *mocks.FileProvider) Config {
				return Config{
					FileProvider: m,
					Logger:       testLogger(),
				}
			},
			expectError: true,
			errorMsg:    "failed to load skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider := mocks.NewFileProvider(t)
			tt.setupMock(mockProvider)
			cfg := tt.config(mockProvider)

			mgr, err := New(cfg)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, mgr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mgr)
			}
		})
	}
}

func TestNew_LoadsExistingSkills(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skill1 := Skill{
		Name:        "greeting",
		Description: "A greeting skill",
		Text:        "Hello, world!",
	}
	skill1Data, _ := json.Marshal(skill1)

	skill2 := Skill{
		Name:        "farewell",
		Description: "A farewell skill",
		Text:        "Goodbye!",
	}
	skill2Data, _ := json.Marshal(skill2)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{
		"greeting.json",
		"farewell.json",
		"readme.txt", // Should be ignored (not .json)
	}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "greeting.json").Return(skill1Data, nil)
	mockProvider.EXPECT().Read(mock.Anything, "farewell.json").Return(skill2Data, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Verify both skills were loaded
	skills, err := mgr.SearchSkills(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, skills, 2)
}

func TestNew_SkipsInvalidJSON(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	validSkill := Skill{
		Name:        "valid",
		Description: "Valid skill",
		Text:        "Content",
	}
	validData, _ := json.Marshal(validSkill)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{
		"valid.json",
		"invalid.json",
	}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "valid.json").Return(validData, nil)
	mockProvider.EXPECT().Read(mock.Anything, "invalid.json").Return([]byte("not valid json"), nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	skills, err := mgr.SearchSkills(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "valid", skills[0].Name)
}

func TestNew_SkipsUnreadableFiles(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	validSkill := Skill{
		Name:        "valid",
		Description: "Valid skill",
		Text:        "Content",
	}
	validData, _ := json.Marshal(validSkill)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{
		"valid.json",
		"unreadable.json",
	}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "valid.json").Return(validData, nil)
	mockProvider.EXPECT().Read(mock.Anything, "unreadable.json").Return(nil, errors.New("permission denied"))

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	skills, err := mgr.SearchSkills(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "valid", skills[0].Name)
}

func TestSearchSkills_Wildcard(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skill1 := Skill{Name: "skill1", Description: "First skill", Text: "Text 1"}
	skill2 := Skill{Name: "skill2", Description: "Second skill", Text: "Text 2"}
	skill1Data, _ := json.Marshal(skill1)
	skill2Data, _ := json.Marshal(skill2)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{"skill1.json", "skill2.json"}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "skill1.json").Return(skill1Data, nil)
	mockProvider.EXPECT().Read(mock.Anything, "skill2.json").Return(skill2Data, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	skills, err := mgr.SearchSkills(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, skills, 2)
}

func TestSearchSkills_SubstringMatchOnName(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skills := []Skill{
		{Name: "greeting-formal", Description: "Formal greeting", Text: "Good day"},
		{Name: "greeting-casual", Description: "Casual greeting", Text: "Hey"},
		{Name: "farewell", Description: "Say goodbye", Text: "Bye"},
	}

	var files []string
	for _, s := range skills {
		files = append(files, s.Name+".json")
	}
	mockProvider.EXPECT().List(mock.Anything, "").Return(files, nil)

	for _, s := range skills {
		data, _ := json.Marshal(s)
		mockProvider.EXPECT().Read(mock.Anything, s.Name+".json").Return(data, nil)
	}

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Search by name substring
	results, err := mgr.SearchSkills(ctx, "greeting")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Contains(t, r.Name, "greeting")
	}
}

func TestSearchSkills_SubstringMatchOnDescription(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skills := []Skill{
		{Name: "intro", Description: "A greeting skill", Text: "Hello"},
		{Name: "outro", Description: "A farewell skill", Text: "Bye"},
	}

	var files []string
	for _, s := range skills {
		files = append(files, s.Name+".json")
	}
	mockProvider.EXPECT().List(mock.Anything, "").Return(files, nil)

	for _, s := range skills {
		data, _ := json.Marshal(s)
		mockProvider.EXPECT().Read(mock.Anything, s.Name+".json").Return(data, nil)
	}

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Search by description substring
	results, err := mgr.SearchSkills(ctx, "greeting")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "intro", results[0].Name)
}

func TestSearchSkills_CaseInsensitive(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skill := Skill{Name: "GREETING", Description: "A GREETING Skill", Text: "Hello"}
	skillData, _ := json.Marshal(skill)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{"GREETING.json"}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "GREETING.json").Return(skillData, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Search with lowercase
	results, err := mgr.SearchSkills(ctx, "greeting")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Search with mixed case
	results, err = mgr.SearchSkills(ctx, "GrEeTiNg")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchSkills_NoMatches(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skill := Skill{Name: "greeting", Description: "Say hello", Text: "Hello"}
	skillData, _ := json.Marshal(skill)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{"greeting.json"}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "greeting.json").Return(skillData, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	results, err := mgr.SearchSkills(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchSkills_EmptyStore(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)
	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{}, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	results, err := mgr.SearchSkills(ctx, "*")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRetrieveSkill_Found(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skill := Skill{
		Name:        "greeting",
		Description: "A greeting skill",
		Text:        "Hello, world!",
	}
	skillData, _ := json.Marshal(skill)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{"greeting.json"}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "greeting.json").Return(skillData, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := mgr.RetrieveSkill(ctx, "greeting")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "greeting", result.Name)
	assert.Equal(t, "A greeting skill", result.Description)
	assert.Equal(t, "Hello, world!", result.Text)
}

func TestRetrieveSkill_NotFound(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)
	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{}, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := mgr.RetrieveSkill(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRetrieveSkill_ExactNameMatch(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	skills := []Skill{
		{Name: "greeting", Description: "Basic greeting", Text: "Hello"},
		{Name: "greeting-formal", Description: "Formal greeting", Text: "Good day"},
	}

	var files []string
	for _, s := range skills {
		files = append(files, s.Name+".json")
	}
	mockProvider.EXPECT().List(mock.Anything, "").Return(files, nil)

	for _, s := range skills {
		data, _ := json.Marshal(s)
		mockProvider.EXPECT().Read(mock.Anything, s.Name+".json").Return(data, nil)
	}

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Should return exact match, not partial
	result, err := mgr.RetrieveSkill(ctx, "greeting")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "greeting", result.Name)
	assert.Equal(t, "Basic greeting", result.Description)
}

func TestUpsertSkill_CreateNew(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)
	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{}, nil)

	newSkill := Skill{
		Name:        "new-skill",
		Description: "A new skill",
		Text:        "New content",
	}

	mockProvider.EXPECT().Write(mock.Anything, "new-skill.json", mock.Anything).
		Run(func(_ context.Context, path string, data []byte) {
			var saved Skill
			err := json.Unmarshal(data, &saved)
			assert.NoError(t, err)
			assert.Equal(t, newSkill.Name, saved.Name)
			assert.Equal(t, newSkill.Description, saved.Description)
			assert.Equal(t, newSkill.Text, saved.Text)
		}).
		Return(nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mgr.UpsertSkill(ctx, newSkill)
	require.NoError(t, err)

	// Verify skill is now retrievable
	result, err := mgr.RetrieveSkill(ctx, "new-skill")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, newSkill.Name, result.Name)
}

func TestUpsertSkill_UpdateExisting(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)

	originalSkill := Skill{
		Name:        "existing",
		Description: "Original description",
		Text:        "Original text",
	}
	originalData, _ := json.Marshal(originalSkill)

	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{"existing.json"}, nil)
	mockProvider.EXPECT().Read(mock.Anything, "existing.json").Return(originalData, nil)

	updatedSkill := Skill{
		Name:        "existing",
		Description: "Updated description",
		Text:        "Updated text",
	}

	mockProvider.EXPECT().Write(mock.Anything, "existing.json", mock.Anything).
		Run(func(_ context.Context, path string, data []byte) {
			var saved Skill
			err := json.Unmarshal(data, &saved)
			assert.NoError(t, err)
			assert.Equal(t, updatedSkill.Description, saved.Description)
			assert.Equal(t, updatedSkill.Text, saved.Text)
		}).
		Return(nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mgr.UpsertSkill(ctx, updatedSkill)
	require.NoError(t, err)

	// Verify skill is updated in cache
	result, err := mgr.RetrieveSkill(ctx, "existing")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Updated description", result.Description)
	assert.Equal(t, "Updated text", result.Text)
}

func TestUpsertSkill_EmptyName(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)
	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{}, nil)

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mgr.UpsertSkill(ctx, Skill{
		Name:        "",
		Description: "No name skill",
		Text:        "Content",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "skill name is required")
}

func TestUpsertSkill_WriteError(t *testing.T) {
	mockProvider := mocks.NewFileProvider(t)
	mockProvider.EXPECT().List(mock.Anything, "").Return([]string{}, nil)
	mockProvider.EXPECT().Write(mock.Anything, "failing-skill.json", mock.Anything).
		Return(errors.New("disk full"))

	mgr, err := New(Config{
		FileProvider: mockProvider,
		Logger:       testLogger(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = mgr.UpsertSkill(ctx, Skill{
		Name:        "failing-skill",
		Description: "This will fail",
		Text:        "Content",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write skill file")
}

func TestSkillFileName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"simple", "simple.json"},
		{"with-dashes", "with-dashes.json"},
		{"with_underscores", "with_underscores.json"},
		{"", ".json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := skillFileName(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}
