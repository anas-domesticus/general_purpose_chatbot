package anthropic

import (
	"context"
	"os"
	"testing"

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
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClaudeModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if model == nil {
					t.Error("expected model to be non-nil")
					return
				}
				if model.Name() == "" {
					t.Error("expected model name to be non-empty")
				}
			}
		})
	}
}

func TestClaudeModel_Name(t *testing.T) {
	model, err := NewClaudeModel("sk-ant-test-key", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	expected := "claude-3-5-sonnet-20241022"
	if got := model.Name(); got != expected {
		t.Errorf("Name() = %v, want %v", got, expected)
	}
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
					Parts: []genai.Part{
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
					Parts: []genai.Part{
						{Text: "You are a helpful assistant."},
					},
				},
				{
					Role: "user",
					Parts: []genai.Part{
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
			if (err != nil) != tt.wantErr {
				t.Errorf("transformADKToAnthropic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(messages) != tt.wantMsgLen {
					t.Errorf("transformADKToAnthropic() messages length = %v, want %v", len(messages), tt.wantMsgLen)
				}
				if system != tt.wantSystem {
					t.Errorf("transformADKToAnthropic() system = %v, want %v", system, tt.wantSystem)
				}
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

	model, err := NewClaudeModel(apiKey, "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	req := &model.LLMRequest{
		Model: model.Name(),
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []genai.Part{
					{Text: "Say hello in exactly 3 words."},
				},
			},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: intPtr(50),
			Temperature:     floatPtr(0.0),
		},
	}

	ctx := context.Background()

	// Test non-streaming
	t.Run("non-streaming", func(t *testing.T) {
		iter := model.GenerateContent(ctx, req, false)
		
		responseCount := 0
		for response, err := range iter {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if response == nil {
				t.Fatal("response is nil")
			}
			if response.Content == nil {
				t.Fatal("response content is nil")
			}
			if len(response.Content.Parts) == 0 {
				t.Fatal("response has no parts")
			}
			
			responseCount++
		}
		
		if responseCount == 0 {
			t.Fatal("no responses received")
		}
	})

	// Test streaming
	t.Run("streaming", func(t *testing.T) {
		iter := model.GenerateContent(ctx, req, true)
		
		responseCount := 0
		finalResponseReceived := false
		
		for response, err := range iter {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if response == nil {
				t.Fatal("response is nil")
			}
			
			responseCount++
			
			if response.TurnComplete {
				finalResponseReceived = true
			}
		}
		
		if responseCount == 0 {
			t.Fatal("no responses received")
		}
		if !finalResponseReceived {
			t.Fatal("final response not received")
		}
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func floatPtr(f float32) *float32 {
	return &f
}