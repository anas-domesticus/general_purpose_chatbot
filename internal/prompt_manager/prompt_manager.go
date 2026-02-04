// Package prompt_manager provides access to system prompts and documents
// stored via a FileProvider backend.
package prompt_manager

import (
	"context"
	"fmt"
	"path"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
)

const (
	systemPromptPath = "system.md"
	docsPrefix       = "docs"
)

// PromptManager provides methods to retrieve system prompts and documents.
type PromptManager struct {
	provider storage_manager.FileProvider
}

// New creates a new PromptManager with the given file provider.
func New(provider storage_manager.FileProvider) *PromptManager {
	if provider == nil {
		panic("file provider cannot be nil")
	}
	return &PromptManager{
		provider: provider,
	}
}

// GetSystemPrompt retrieves the system prompt from system.md.
func (m *PromptManager) GetSystemPrompt(ctx context.Context) (string, error) {
	data, err := m.provider.Read(ctx, systemPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read system prompt: %w", err)
	}
	return string(data), nil
}

// GetDocument retrieves a document from the docs directory.
// The path parameter should be relative to the docs directory.
func (m *PromptManager) GetDocument(ctx context.Context, docPath string) (string, error) {
	if docPath == "" {
		return "", fmt.Errorf("document path cannot be empty")
	}

	fullPath := path.Join(docsPrefix, docPath)
	data, err := m.provider.Read(ctx, fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read document %s: %w", docPath, err)
	}
	return string(data), nil
}
