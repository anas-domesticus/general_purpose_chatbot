package anthropic

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestNewClaudeModel(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		modelName string
		wantErr   bool
	}{
		{
			name:      "valid api key with model name",
			apiKey:    "sk-ant-test-key",
			modelName: "claude-3-5-sonnet-20241022",
			wantErr:   false,
		},
		{
			name:      "valid api key without model name (uses default)",
			apiKey:    "sk-ant-test-key",
			modelName: "",
			wantErr:   false,
		},
		{
			name:      "empty api key",
			apiKey:    "",
			modelName: "claude-3-5-sonnet-20241022",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewClaudeModel(tt.apiKey, tt.modelName)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, model)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, model)
				assert.NotEmpty(t, model.Name())
			}
		})
	}
}

func TestClaudeModel_Name(t *testing.T) {
	model, err := NewClaudeModel("sk-ant-test-key", "claude-3-5-sonnet-20241022")
	require.NoError(t, err)

	expected := "claude-3-5-sonnet-20241022"
	assert.Equal(t, expected, model.Name())
}

func TestTransformADKToAnthropic(t *testing.T) {
	tests := []struct {
		name        string
		contents    []*genai.Content
		wantMsgLen  int
		wantSystem  string
		wantErr     bool
	}{
		{
			name:       "empty contents",
			contents:   []*genai.Content{},
			wantMsgLen: 0,
			wantSystem: "",
			wantErr:    true,
		},
		{
			name: "user message with text",
			contents: []*genai.Content{
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: "Hello, world!"},
					},
				},
			},
			wantMsgLen: 1,
			wantSystem: "",
			wantErr:    false,
		},
		{
			name: "system and user messages",
			contents: []*genai.Content{
				{
					Role: "system",
					Parts: []*genai.Part{
						{Text: "You are a helpful assistant."},
					},
				},
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: "Hello!"},
					},
				},
			},
			wantMsgLen: 1,
			wantSystem: "You are a helpful assistant.",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, system, err := transformADKToAnthropic(tt.contents)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, messages, tt.wantMsgLen)
				assert.Equal(t, tt.wantSystem, system)
			}
		})
	}
}

// TestClaudeModel_GenerateContent_Integration is an integration test that requires a real API key
func TestClaudeModel_GenerateContent_Integration(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	claudeModel, err := NewClaudeModel(apiKey, "claude-3-5-sonnet-20241022")
	require.NoError(t, err)

	req := &model.LLMRequest{
		Model: claudeModel.Name(),
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Say hello in exactly 3 words."},
				},
			},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: 50,
			Temperature:     floatPtr(0.0),
		},
	}

	ctx := context.Background()

	// Test non-streaming
	t.Run("non-streaming", func(t *testing.T) {
		iter := claudeModel.GenerateContent(ctx, req, false)
		
		responseCount := 0
		for response, err := range iter {
			require.NoError(t, err)
			assert.NotNil(t, response)
			assert.NotNil(t, response.Content)
			assert.NotEmpty(t, response.Content.Parts)
			
			responseCount++
		}
		
		assert.Greater(t, responseCount, 0, "should receive at least one response")
	})
}

// Helper functions
func floatPtr(f float32) *float32 {
	return &f
}