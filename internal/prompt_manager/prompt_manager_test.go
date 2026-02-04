package prompt_manager

import (
	"context"
	"errors"
	"testing"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNew(t *testing.T) {
	t.Run("creates manager with valid provider", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)
		manager := New(mockProvider)
		assert.NotNil(t, manager)
	})

	t.Run("panics with nil provider", func(t *testing.T) {
		assert.Panics(t, func() {
			New(nil)
		})
	})
}

func TestPromptManager_GetSystemPrompt(t *testing.T) {
	ctx := context.Background()

	t.Run("returns system prompt successfully", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)
		expectedContent := "You are a helpful assistant."

		mockProvider.EXPECT().
			Read(mock.Anything, "system.md").
			Return([]byte(expectedContent), nil)

		manager := New(mockProvider)
		result, err := manager.GetSystemPrompt(ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedContent, result)
	})

	t.Run("returns error when read fails", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)

		mockProvider.EXPECT().
			Read(mock.Anything, "system.md").
			Return(nil, errors.New("file not found"))

		manager := New(mockProvider)
		result, err := manager.GetSystemPrompt(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read system prompt")
		assert.Empty(t, result)
	})
}

func TestPromptManager_GetDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("returns document successfully", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)
		expectedContent := "# Documentation\n\nThis is a test document."

		mockProvider.EXPECT().
			Read(mock.Anything, "docs/guide.md").
			Return([]byte(expectedContent), nil)

		manager := New(mockProvider)
		result, err := manager.GetDocument(ctx, "guide.md")

		assert.NoError(t, err)
		assert.Equal(t, expectedContent, result)
	})

	t.Run("returns document from nested path", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)
		expectedContent := "API reference content"

		mockProvider.EXPECT().
			Read(mock.Anything, "docs/api/reference.md").
			Return([]byte(expectedContent), nil)

		manager := New(mockProvider)
		result, err := manager.GetDocument(ctx, "api/reference.md")

		assert.NoError(t, err)
		assert.Equal(t, expectedContent, result)
	})

	t.Run("returns error for empty path", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)

		manager := New(mockProvider)
		result, err := manager.GetDocument(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "document path cannot be empty")
		assert.Empty(t, result)
	})

	t.Run("returns error when read fails", func(t *testing.T) {
		mockProvider := mocks.NewFileProvider(t)

		mockProvider.EXPECT().
			Read(mock.Anything, "docs/missing.md").
			Return(nil, errors.New("file not found"))

		manager := New(mockProvider)
		result, err := manager.GetDocument(ctx, "missing.md")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read document")
		assert.Contains(t, err.Error(), "missing.md")
		assert.Empty(t, result)
	})
}
