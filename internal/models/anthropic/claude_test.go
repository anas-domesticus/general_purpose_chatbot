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
		wantSysBlocks int
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
			wantSysBlocks: 0,
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
			wantSysBlocks: 1,
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
			wantSysBlocks: 0,
			wantErr:       false,
		},
		{
			name:          "nil contents",
			contents:      nil,
			wantMsgCount:  0,
			wantSysBlocks: 0,
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
			wantSysBlocks: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, sysBlocks, err := transformADKToAnthropic(tt.contents)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformADKToAnthropic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(msgs) != tt.wantMsgCount {
				t.Errorf("transformADKToAnthropic() message count = %v, want %v", len(msgs), tt.wantMsgCount)
			}
			if len(sysBlocks) != tt.wantSysBlocks {
				t.Errorf("transformADKToAnthropic() system blocks count = %v, want %v", len(sysBlocks), tt.wantSysBlocks)
			}

			// Verify cache control on last system block if present
			if len(sysBlocks) > 0 {
				lastBlock := sysBlocks[len(sysBlocks)-1]
				// Check that cache control has been set (non-zero value)
				emptyCC := anthropic.CacheControlEphemeralParam{}
				if lastBlock.CacheControl == emptyCC {
					t.Error("transformADKToAnthropic() last system block should have cache control")
				}
			}

			// Verify cache control on last user message if present
			hasUserMsg := false
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == anthropic.MessageParamRoleUser && len(msgs[i].Content) > 0 {
					hasUserMsg = true
					lastBlock := msgs[i].Content[len(msgs[i].Content)-1]
					emptyCC := anthropic.CacheControlEphemeralParam{}
					// Check if any block type has cache control set (non-zero value)
					hasCacheControl := (lastBlock.OfText != nil && lastBlock.OfText.CacheControl != emptyCC) ||
						(lastBlock.OfImage != nil && lastBlock.OfImage.CacheControl != emptyCC) ||
						(lastBlock.OfToolResult != nil && lastBlock.OfToolResult.CacheControl != emptyCC)
					if !hasCacheControl {
						t.Error("transformADKToAnthropic() last user message should have cache control on last block")
					}
					break
				}
			}
			_ = hasUserMsg // Used in check above
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
			name: "function call part with nil args",
			part: &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   "call_456",
					Name: "no_args_tool",
					Args: nil, // nil Args should not cause API errors
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

// mockTool is a test helper that implements the toolWithDeclaration interface
type mockTool struct {
	decl *genai.FunctionDeclaration
}

func (m *mockTool) Declaration() *genai.FunctionDeclaration {
	return m.decl
}

func TestTransformAnthropicToADK_CacheUsage(t *testing.T) {
	msg := &anthropic.Message{
		ID: "msg_123",
		Content: []anthropic.ContentBlockUnion{
			{
				Type: "text",
				Text: "Hello",
			},
		},
		Model:      "claude-3-5-sonnet-20241022",
		StopReason: anthropic.StopReasonEndTurn,
		Usage: anthropic.Usage{
			InputTokens:              100,
			OutputTokens:             50,
			CacheCreationInputTokens: 500,
			CacheReadInputTokens:     200,
		},
	}

	response, err := transformAnthropicToADK(msg)
	if err != nil {
		t.Fatalf("transformAnthropicToADK() error = %v", err)
	}

	if response.UsageMetadata == nil {
		t.Fatal("transformAnthropicToADK() UsageMetadata is nil")
	}

	// Verify prompt tokens include both new input and cache creation
	expectedPromptTokens := int32(100 + 500)
	if response.UsageMetadata.PromptTokenCount != expectedPromptTokens {
		t.Errorf("PromptTokenCount = %v, want %v", response.UsageMetadata.PromptTokenCount, expectedPromptTokens)
	}

	// Verify cached content token count
	expectedCachedTokens := int32(200)
	if response.UsageMetadata.CachedContentTokenCount != expectedCachedTokens {
		t.Errorf("CachedContentTokenCount = %v, want %v", response.UsageMetadata.CachedContentTokenCount, expectedCachedTokens)
	}

	// Verify output tokens
	expectedOutputTokens := int32(50)
	if response.UsageMetadata.CandidatesTokenCount != expectedOutputTokens {
		t.Errorf("CandidatesTokenCount = %v, want %v", response.UsageMetadata.CandidatesTokenCount, expectedOutputTokens)
	}

	// Verify total includes all token types
	expectedTotal := int32(100 + 500 + 200 + 50)
	if response.UsageMetadata.TotalTokenCount != expectedTotal {
		t.Errorf("TotalTokenCount = %v, want %v", response.UsageMetadata.TotalTokenCount, expectedTotal)
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
			tools, err := transformToolsToAnthropic(tt.tools)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformToolsToAnthropic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tools) != tt.wantCount {
				t.Errorf("transformToolsToAnthropic() count = %v, want %v", len(tools), tt.wantCount)
			}

			// Verify cache control on last tool if present
			if len(tools) > 0 {
				lastTool := tools[len(tools)-1]
				emptyCC := anthropic.CacheControlEphemeralParam{}
				if lastTool.OfTool != nil && lastTool.OfTool.CacheControl == emptyCC {
					t.Error("transformToolsToAnthropic() last tool should have cache control")
				}
			}
		})
	}
}
