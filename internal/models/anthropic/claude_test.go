package anthropic

import (
	"context"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
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
			name:      "valid inputs",
			apiKey:    "test-api-key",
			modelName: "claude-3-5-sonnet-20241022",
			wantErr:   false,
		},
		{
			name:      "empty api key",
			apiKey:    "",
			modelName: "claude-3-5-sonnet-20241022",
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
			model, err := NewClaudeModel(tt.apiKey, tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClaudeModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && model == nil {
				t.Error("NewClaudeModel() returned nil model without error")
			}
			if !tt.wantErr && model.Name() != tt.modelName {
				t.Errorf("NewClaudeModel() Name() = %v, want %v", model.Name(), tt.modelName)
			}
		})
	}
}

func TestClaudeModel_Name(t *testing.T) {
	m, err := NewClaudeModel("test-key", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("NewClaudeModel() error = %v", err)
	}

	if got := m.Name(); got != "claude-3-5-sonnet-20241022" {
		t.Errorf("Name() = %v, want %v", got, "claude-3-5-sonnet-20241022")
	}
}

func TestClaudeModel_GenerateContent_StreamingNotSupported(t *testing.T) {
	m, err := NewClaudeModel("test-key", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("NewClaudeModel() error = %v", err)
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

func TestTransformADKToAnthropic(t *testing.T) {
	tests := []struct {
		name          string
		contents      []*genai.Content
		wantMsgCount  int
		wantSysPrompt bool
		wantErr       bool
	}{
		{
			name: "single user message",
			contents: []*genai.Content{
				{
					Role:  "user",
					Parts: []*genai.Part{{Text: "Hello"}},
				},
			},
			wantMsgCount:  1,
			wantSysPrompt: false,
			wantErr:       false,
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
			wantMsgCount:  1,
			wantSysPrompt: true,
			wantErr:       false,
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
			wantMsgCount:  3,
			wantSysPrompt: false,
			wantErr:       false,
		},
		{
			name:          "nil contents",
			contents:      nil,
			wantMsgCount:  0,
			wantSysPrompt: false,
			wantErr:       false,
		},
		{
			name: "model role mapped to assistant",
			contents: []*genai.Content{
				{
					Role:  "model",
					Parts: []*genai.Part{{Text: "I am a model"}},
				},
			},
			wantMsgCount:  1,
			wantSysPrompt: false,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, sysPrompt, err := transformADKToAnthropic(tt.contents)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformADKToAnthropic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(msgs) != tt.wantMsgCount {
				t.Errorf("transformADKToAnthropic() message count = %v, want %v", len(msgs), tt.wantMsgCount)
			}
			if (sysPrompt != "") != tt.wantSysPrompt {
				t.Errorf("transformADKToAnthropic() has system prompt = %v, want %v", sysPrompt != "", tt.wantSysPrompt)
			}
		})
	}
}

func TestConvertPartToContentBlock(t *testing.T) {
	tests := []struct {
		name    string
		part    *genai.Part
		wantNil bool
		wantErr bool
	}{
		{
			name:    "nil part",
			part:    nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "text part",
			part:    &genai.Part{Text: "Hello world"},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "image part",
			part: &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: "image/png",
					Data:     []byte{0x89, 0x50, 0x4E, 0x47},
				},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "function call part",
			part: &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   "call_123",
					Name: "get_weather",
					Args: map[string]any{"location": "NYC"},
				},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "function response part",
			part: &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       "call_123",
					Name:     "get_weather",
					Response: map[string]any{"temp": 72},
				},
			},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := convertPartToContentBlock(tt.part)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertPartToContentBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Check if block is zero value (nil equivalent for struct)
			isEmpty := block.OfText == nil && block.OfImage == nil && block.OfToolUse == nil && block.OfToolResult == nil
			if isEmpty != tt.wantNil {
				t.Errorf("convertPartToContentBlock() isEmpty = %v, wantNil %v", isEmpty, tt.wantNil)
			}
		})
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		name       string
		stopReason anthropic.StopReason
		want       genai.FinishReason
	}{
		{"end_turn", anthropic.StopReasonEndTurn, genai.FinishReasonStop},
		{"max_tokens", anthropic.StopReasonMaxTokens, genai.FinishReasonMaxTokens},
		{"tool_use", anthropic.StopReasonToolUse, genai.FinishReasonStop},
		{"stop_sequence", anthropic.StopReasonStopSequence, genai.FinishReasonStop},
		{"refusal", anthropic.StopReasonRefusal, genai.FinishReasonSafety},
		{"unknown", anthropic.StopReason("unknown"), genai.FinishReasonOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStopReason(tt.stopReason)
			if got != tt.want {
				t.Errorf("mapStopReason(%v) = %v, want %v", tt.stopReason, got, tt.want)
			}
		})
	}
}

func TestTransformToolsToAnthropic(t *testing.T) {
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
				"get_weather": map[string]any{
					"name":        "get_weather",
					"description": "Get weather for a location",
					"inputSchema": map[string]any{
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
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "tool without name is skipped",
			tools: map[string]any{
				"invalid": map[string]any{
					"description": "No name",
				},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "multiple tools",
			tools: map[string]any{
				"tool1": map[string]any{
					"name":        "tool1",
					"description": "First tool",
				},
				"tool2": map[string]any{
					"name":        "tool2",
					"description": "Second tool",
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := transformToolsToAnthropic(tt.tools)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformToolsToAnthropic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tools) != tt.wantCount {
				t.Errorf("transformToolsToAnthropic() count = %v, want %v", len(tools), tt.wantCount)
			}
		})
	}
}
