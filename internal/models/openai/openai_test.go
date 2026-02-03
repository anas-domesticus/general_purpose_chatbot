package openai

import (
	"context"
	"testing"

	"github.com/openai/openai-go"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestNewOpenAIModel(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		modelName string
		wantErr   bool
	}{
		{
			name:      "valid inputs",
			apiKey:    "test-api-key",
			modelName: "gpt-4o",
			wantErr:   false,
		},
		{
			name:      "empty api key",
			apiKey:    "",
			modelName: "gpt-4o",
			wantErr:   true,
		},
		{
			name:      "empty model name",
			apiKey:    "test-api-key",
			modelName: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewOpenAIModel(tt.apiKey, tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && m == nil {
				t.Error("NewOpenAIModel() returned nil model without error")
			}
			if !tt.wantErr && m.Name() != tt.modelName {
				t.Errorf("NewOpenAIModel() Name() = %v, want %v", m.Name(), tt.modelName)
			}
		})
	}
}

func TestOpenAIModel_Name(t *testing.T) {
	m, err := NewOpenAIModel("test-key", "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIModel() error = %v", err)
	}

	if got := m.Name(); got != "gpt-4o" {
		t.Errorf("Name() = %v, want %v", got, "gpt-4o")
	}
}

func TestOpenAIModel_GenerateContent_StreamingNotSupported(t *testing.T) {
	m, err := NewOpenAIModel("test-key", "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIModel() error = %v", err)
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Hello"}},
			},
		},
	}

	// Test streaming returns error
	iter := m.GenerateContent(context.Background(), req, true)
	for _, err := range iter {
		if err == nil {
			t.Error("GenerateContent() with stream=true should return error")
		}
		break
	}
}

func TestTransformADKToOpenAI(t *testing.T) {
	tests := []struct {
		name         string
		contents     []*genai.Content
		wantMsgCount int
		wantErr      bool
	}{
		{
			name: "single user message",
			contents: []*genai.Content{
				{
					Role:  "user",
					Parts: []*genai.Part{{Text: "Hello"}},
				},
			},
			wantMsgCount: 1,
			wantErr:      false,
		},
		{
			name: "system and user messages",
			contents: []*genai.Content{
				{
					Role:  "system",
					Parts: []*genai.Part{{Text: "You are helpful"}},
				},
				{
					Role:  "user",
					Parts: []*genai.Part{{Text: "Hello"}},
				},
			},
			wantMsgCount: 2,
			wantErr:      false,
		},
		{
			name: "multi-turn conversation",
			contents: []*genai.Content{
				{
					Role:  "user",
					Parts: []*genai.Part{{Text: "Hello"}},
				},
				{
					Role:  "assistant",
					Parts: []*genai.Part{{Text: "Hi there!"}},
				},
				{
					Role:  "user",
					Parts: []*genai.Part{{Text: "How are you?"}},
				},
			},
			wantMsgCount: 3,
			wantErr:      false,
		},
		{
			name:         "nil contents",
			contents:     nil,
			wantMsgCount: 0,
			wantErr:      false,
		},
		{
			name: "model role mapped to assistant",
			contents: []*genai.Content{
				{
					Role:  "model",
					Parts: []*genai.Part{{Text: "I am a model"}},
				},
			},
			wantMsgCount: 1,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, err := transformADKToOpenAI(tt.contents)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformADKToOpenAI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(msgs) != tt.wantMsgCount {
				t.Errorf("transformADKToOpenAI() message count = %v, want %v", len(msgs), tt.wantMsgCount)
			}
		})
	}
}

func TestConvertPartsToUserContent(t *testing.T) {
	tests := []struct {
		name      string
		parts     []*genai.Part
		wantCount int
		wantErr   bool
	}{
		{
			name:      "nil parts",
			parts:     nil,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "text part",
			parts:     []*genai.Part{{Text: "Hello world"}},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "image part",
			parts: []*genai.Part{
				{
					InlineData: &genai.Blob{
						MIMEType: "image/png",
						Data:     []byte{0x89, 0x50, 0x4E, 0x47},
					},
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple parts",
			parts: []*genai.Part{
				{Text: "Look at this:"},
				{
					InlineData: &genai.Blob{
						MIMEType: "image/jpeg",
						Data:     []byte{0xFF, 0xD8, 0xFF},
					},
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertPartsToUserContent(tt.parts)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertPartsToUserContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(result) != tt.wantCount {
				t.Errorf("convertPartsToUserContent() count = %v, want %v", len(result), tt.wantCount)
			}
		})
	}
}

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		name         string
		finishReason string
		want         genai.FinishReason
	}{
		{"stop", "stop", genai.FinishReasonStop},
		{"length", "length", genai.FinishReasonMaxTokens},
		{"tool_calls", "tool_calls", genai.FinishReasonStop},
		{"content_filter", "content_filter", genai.FinishReasonSafety},
		{"function_call", "function_call", genai.FinishReasonStop},
		{"unknown", "unknown", genai.FinishReasonOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapFinishReason(tt.finishReason)
			if got != tt.want {
				t.Errorf("mapFinishReason(%v) = %v, want %v", tt.finishReason, got, tt.want)
			}
		})
	}
}

// mockTool is a test helper that implements the toolWithDeclaration interface
type mockTool struct {
	decl *genai.FunctionDeclaration
}

func (m *mockTool) Declaration() *genai.FunctionDeclaration {
	return m.decl
}

func TestTransformToolsToOpenAI(t *testing.T) {
	tests := []struct {
		name      string
		tools     map[string]any
		wantCount int
		wantErr   bool
	}{
		{
			name:      "nil tools",
			tools:     nil,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "empty tools",
			tools:     map[string]any{},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "single tool",
			tools: map[string]any{
				"get_weather": &mockTool{
					decl: &genai.FunctionDeclaration{
						Name:        "get_weather",
						Description: "Get weather for a location",
						ParametersJsonSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{
									"type":        "string",
									"description": "City name",
								},
							},
							"required": []any{"location"},
						},
					},
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "tool without name is skipped",
			tools: map[string]any{
				"invalid": &mockTool{
					decl: &genai.FunctionDeclaration{
						Name:        "",
						Description: "No name",
					},
				},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "multiple tools",
			tools: map[string]any{
				"tool1": &mockTool{
					decl: &genai.FunctionDeclaration{
						Name:        "tool1",
						Description: "First tool",
					},
				},
				"tool2": &mockTool{
					decl: &genai.FunctionDeclaration{
						Name:        "tool2",
						Description: "Second tool",
					},
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := transformToolsToOpenAI(tt.tools)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformToolsToOpenAI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tools) != tt.wantCount {
				t.Errorf("transformToolsToOpenAI() count = %v, want %v", len(tools), tt.wantCount)
			}
		})
	}
}

func TestTransformOpenAIToADK(t *testing.T) {
	tests := []struct {
		name       string
		completion *openai.ChatCompletion
		wantErr    bool
		wantParts  int
	}{
		{
			name:       "nil completion",
			completion: nil,
			wantErr:    true,
			wantParts:  0,
		},
		{
			name: "empty choices",
			completion: &openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{},
			},
			wantErr:   true,
			wantParts: 0,
		},
		{
			name: "simple text response",
			completion: &openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "Hello!",
						},
						FinishReason: "stop",
					},
				},
				Usage: openai.CompletionUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			wantErr:   false,
			wantParts: 1,
		},
		{
			name: "response with tool calls",
			completion: &openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							ToolCalls: []openai.ChatCompletionMessageToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: openai.ChatCompletionMessageToolCallFunction{
										Name:      "get_weather",
										Arguments: `{"location":"NYC"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			},
			wantErr:   false,
			wantParts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := transformOpenAIToADK(tt.completion)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformOpenAIToADK() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if response == nil {
					t.Error("transformOpenAIToADK() returned nil response without error")
					return
				}
				if response.Content == nil {
					t.Error("transformOpenAIToADK() returned nil content")
					return
				}
				if len(response.Content.Parts) != tt.wantParts {
					t.Errorf("transformOpenAIToADK() parts count = %v, want %v", len(response.Content.Parts), tt.wantParts)
				}
			}
		})
	}
}

func TestTransformOpenAIToADK_UsageMetadata(t *testing.T) {
	completion := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: "Hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: openai.CompletionUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			PromptTokensDetails: openai.CompletionUsagePromptTokensDetails{
				CachedTokens: 25,
			},
		},
	}

	response, err := transformOpenAIToADK(completion)
	if err != nil {
		t.Fatalf("transformOpenAIToADK() error = %v", err)
	}

	if response.UsageMetadata == nil {
		t.Fatal("transformOpenAIToADK() UsageMetadata is nil")
	}

	if response.UsageMetadata.PromptTokenCount != 100 {
		t.Errorf("PromptTokenCount = %v, want 100", response.UsageMetadata.PromptTokenCount)
	}

	if response.UsageMetadata.CandidatesTokenCount != 50 {
		t.Errorf("CandidatesTokenCount = %v, want 50", response.UsageMetadata.CandidatesTokenCount)
	}

	if response.UsageMetadata.TotalTokenCount != 150 {
		t.Errorf("TotalTokenCount = %v, want 150", response.UsageMetadata.TotalTokenCount)
	}

	if response.UsageMetadata.CachedContentTokenCount != 25 {
		t.Errorf("CachedContentTokenCount = %v, want 25", response.UsageMetadata.CachedContentTokenCount)
	}
}

func TestCreateToolResultMessage(t *testing.T) {
	result := map[string]any{"temperature": 72, "unit": "fahrenheit"}
	msg, err := CreateToolResultMessage("call_123", result)
	if err != nil {
		t.Fatalf("CreateToolResultMessage() error = %v", err)
	}

	// Verify the message is a tool message
	if msg.OfTool == nil {
		t.Error("CreateToolResultMessage() did not create a tool message")
	}
}
