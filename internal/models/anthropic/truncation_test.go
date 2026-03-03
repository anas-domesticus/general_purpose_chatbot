package anthropic

import (
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestEstimateStringTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"short", "hi", 1},
		{"exact multiple", "abcd", 1},
		{"with remainder", "abcde", 2},
		{"longer text", strings.Repeat("a", 100), 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateStringTokens(tt.input)
			if got != tt.want {
				t.Errorf("estimateStringTokens(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEstimateBlockTokens_Text(t *testing.T) {
	block := anthropic.NewTextBlock("hello world")
	tokens := estimateBlockTokens(block)
	if tokens == 0 {
		t.Error("expected non-zero token estimate for text block")
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	msg := anthropic.NewUserMessage(anthropic.NewTextBlock("hello"))
	tokens := estimateMessageTokens(msg)
	// Should include overhead (4) + text tokens
	if tokens <= 4 {
		t.Errorf("expected tokens > 4 (overhead), got %d", tokens)
	}
}

func TestIsToolResultMessage(t *testing.T) {
	userMsg := anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))
	if isToolResultMessage(userMsg) {
		t.Error("text-only user message should not be a tool result message")
	}

	toolResultMsg := anthropic.NewUserMessage(
		anthropic.NewToolResultBlock("tool-1", "result text", false),
	)
	if !isToolResultMessage(toolResultMsg) {
		t.Error("message with tool result should be detected")
	}
}

func TestEstimateBlockTokens_ToolUse(t *testing.T) {
	block := anthropic.NewToolUseBlock("call-1", map[string]any{"query": "hello world"}, "search")
	tokens := estimateBlockTokens(block)
	if tokens == 0 {
		t.Error("expected non-zero token estimate for tool use block")
	}
}

func TestEstimateBlockTokens_ToolResult(t *testing.T) {
	block := anthropic.NewToolResultBlock("call-1", "search results here", false)
	tokens := estimateBlockTokens(block)
	if tokens == 0 {
		t.Error("expected non-zero token estimate for tool result block")
	}
}

func TestEstimateBlockTokens_Image(t *testing.T) {
	block := anthropic.NewImageBlockBase64("image/png", "abc123")
	tokens := estimateBlockTokens(block)
	if tokens != 1600 {
		t.Errorf("expected 1600 token estimate for image block, got %d", tokens)
	}
}

func TestEstimateBlockTokens_Empty(t *testing.T) {
	block := anthropic.ContentBlockParamUnion{}
	tokens := estimateBlockTokens(block)
	if tokens != 0 {
		t.Errorf("expected 0 for empty block, got %d", tokens)
	}
}

func TestEstimateToolResultBlockTokens(t *testing.T) {
	t.Run("text content", func(t *testing.T) {
		// Build a ToolResultBlockParamContentUnion with text
		// NewToolResultBlock wraps text internally, so access the content from a tool result block
		block := anthropic.NewToolResultBlock("call-1", "some result text", false)
		// The block.OfToolResult.Content contains ToolResultBlockParamContentUnion entries
		if block.OfToolResult == nil || len(block.OfToolResult.Content) == 0 {
			t.Fatal("expected tool result to have content")
		}
		tokens := estimateToolResultBlockTokens(block.OfToolResult.Content[0])
		if tokens == 0 {
			t.Error("expected non-zero token estimate for tool result text content")
		}
	})

	t.Run("empty content", func(t *testing.T) {
		block := anthropic.ToolResultBlockParamContentUnion{}
		tokens := estimateToolResultBlockTokens(block)
		if tokens != 0 {
			t.Errorf("expected 0 for empty content, got %d", tokens)
		}
	})
}

func TestEstimateSystemBlocksTokens(t *testing.T) {
	t.Run("nil blocks", func(t *testing.T) {
		tokens := estimateSystemBlocksTokens(nil)
		if tokens != 0 {
			t.Errorf("expected 0 for nil blocks, got %d", tokens)
		}
	})

	t.Run("single block", func(t *testing.T) {
		blocks := []anthropic.TextBlockParam{{Text: "You are a helpful assistant."}}
		tokens := estimateSystemBlocksTokens(blocks)
		if tokens == 0 {
			t.Error("expected non-zero tokens for system block")
		}
	})

	t.Run("multiple blocks", func(t *testing.T) {
		blocks := []anthropic.TextBlockParam{
			{Text: "You are a helpful assistant."},
			{Text: "Always respond in JSON format."},
		}
		singleBlock := []anthropic.TextBlockParam{{Text: "You are a helpful assistant."}}
		multiTokens := estimateSystemBlocksTokens(blocks)
		singleTokens := estimateSystemBlocksTokens(singleBlock)
		if multiTokens <= singleTokens {
			t.Errorf("multiple blocks should have more tokens than single: %d vs %d", multiTokens, singleTokens)
		}
	})
}

func TestEstimateToolsTokens(t *testing.T) {
	t.Run("nil tools", func(t *testing.T) {
		tokens := estimateToolsTokens(nil)
		if tokens != 0 {
			t.Errorf("expected 0 for nil tools, got %d", tokens)
		}
	})

	t.Run("single tool", func(t *testing.T) {
		schema := anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]any{
				"query": map[string]any{"type": "string", "description": "search query"},
			},
		}
		tool := anthropic.ToolUnionParamOfTool(schema, "web_search")
		tool.OfTool.Description = anthropic.String("Search the web for information")
		tokens := estimateToolsTokens([]anthropic.ToolUnionParam{tool})
		if tokens == 0 {
			t.Error("expected non-zero tokens for tool definition")
		}
	})

	t.Run("tool without description", func(t *testing.T) {
		schema := anthropic.ToolInputSchemaParam{Type: "object"}
		tool := anthropic.ToolUnionParamOfTool(schema, "ping")
		tokens := estimateToolsTokens([]anthropic.ToolUnionParam{tool})
		// Should still count the name
		if tokens == 0 {
			t.Error("expected non-zero tokens for tool name")
		}
	})

	t.Run("tool with nil OfTool", func(t *testing.T) {
		tool := anthropic.ToolUnionParam{} // no OfTool set
		tokens := estimateToolsTokens([]anthropic.ToolUnionParam{tool})
		if tokens != 0 {
			t.Errorf("expected 0 for tool with nil OfTool, got %d", tokens)
		}
	})
}

func TestEstimateMessageTokens_MultipleBlocks(t *testing.T) {
	msg := anthropic.NewUserMessage(
		anthropic.NewTextBlock("first part"),
		anthropic.NewTextBlock("second part"),
	)
	tokens := estimateMessageTokens(msg)
	singleMsg := anthropic.NewUserMessage(anthropic.NewTextBlock("first part"))
	singleTokens := estimateMessageTokens(singleMsg)
	if tokens <= singleTokens {
		t.Errorf("multi-block message should have more tokens: %d vs %d", tokens, singleTokens)
	}
}

// makeTextMsg creates a message with a text block of approximately the given token count.
func makeTextMsg(role string, approxTokens int) anthropic.MessageParam {
	text := strings.Repeat("a", approxTokens*charsPerToken)
	block := anthropic.NewTextBlock(text)
	if role == "user" {
		return anthropic.NewUserMessage(block)
	}
	return anthropic.NewAssistantMessage(block)
}

func TestTruncateMessages_NoTruncationNeeded(t *testing.T) {
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 100),
		makeTextMsg("assistant", 100),
	}

	result, removed := truncateMessages(messages, 0)
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if len(result) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(result))
	}
}

func TestTruncateMessages_TruncatesOldest(t *testing.T) {
	// Create messages that total well over maxInputTokenBudget
	// Each message has ~50000 tokens worth of text
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 50000),     // old - should be removed
		makeTextMsg("assistant", 50000), // old - should be removed
		makeTextMsg("user", 50000),     // old - should be removed
		makeTextMsg("assistant", 50000), // old - should be removed
		makeTextMsg("user", 50000),     // recent - should be kept
		makeTextMsg("assistant", 50000), // recent - should be kept
	}

	result, removed := truncateMessages(messages, 0)
	if removed == 0 {
		t.Fatal("expected some messages to be removed")
	}
	// Result should start with a user message
	if result[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("truncated messages should start with user, got %s", result[0].Role)
	}
	// Should have fewer messages
	if len(result) >= len(messages) {
		t.Errorf("expected fewer messages after truncation, got %d (original %d)", len(result), len(messages))
	}
}

func TestTruncateMessages_SkipsToolResultBoundary(t *testing.T) {
	// Build: user, assistant(tool_use), user(tool_result), assistant, user, assistant
	// Total tokens must exceed budget to trigger truncation.
	// Truncation should NOT start at the tool_result user message.
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 60000),
		// assistant with tool use
		anthropic.NewAssistantMessage(
			anthropic.NewToolUseBlock("tool-1", map[string]any{"q": strings.Repeat("x", 60000*charsPerToken)}, "search"),
		),
		// user with tool result (NOT a valid truncation point)
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("tool-1", strings.Repeat("r", 60000*charsPerToken), false),
		),
		makeTextMsg("assistant", 100),
		makeTextMsg("user", 100),     // this IS a valid truncation point
		makeTextMsg("assistant", 100),
	}

	result, removed := truncateMessages(messages, 0)
	if removed == 0 {
		t.Fatal("expected truncation")
	}
	// The first message should be a user text message, not a tool result
	if isToolResultMessage(result[0]) {
		t.Error("truncation should not start at a tool result message")
	}
	if result[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("truncated messages should start with user, got %s", result[0].Role)
	}
}

func TestTruncateMessages_RespectsFixedOverhead(t *testing.T) {
	// Use many small messages so different budgets yield different truncation points.
	// 10 user/assistant pairs of ~20000 tokens each = ~200000 total message tokens.
	// With budget=160000, need to remove ~40000 → 2 messages.
	// With overhead=80000, budget=80000, need to remove ~120000 → 6 messages.
	messages := make([]anthropic.MessageParam, 0, 20)
	for range 10 {
		messages = append(messages, makeTextMsg("user", 10000))
		messages = append(messages, makeTextMsg("assistant", 10000))
	}

	_, removedNoOverhead := truncateMessages(messages, 0)
	_, removedWithOverhead := truncateMessages(messages, 80000)

	if removedWithOverhead <= removedNoOverhead {
		t.Errorf("higher overhead should cause more truncation: removed %d (overhead=80000) vs %d (overhead=0)",
			removedWithOverhead, removedNoOverhead)
	}
}

func TestTruncateMessages_EmptyMessages(t *testing.T) {
	result, removed := truncateMessages(nil, 0)
	if removed != 0 || result != nil {
		t.Errorf("nil messages should return nil, 0; got %v, %d", result, removed)
	}

	result, removed = truncateMessages([]anthropic.MessageParam{}, 0)
	if removed != 0 || len(result) != 0 {
		t.Errorf("empty messages should return empty, 0; got %d messages, %d removed", len(result), removed)
	}
}

func TestTruncateMessages_SingleMessage(t *testing.T) {
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 200000), // over budget but only one message
	}

	result, removed := truncateMessages(messages, 0)
	// Should still return the single message (can't truncate further)
	if len(result) != 1 {
		t.Errorf("expected 1 message, got %d", len(result))
	}
	// The "removed" count reflects fallback behavior
	_ = removed
}

func TestTruncateMessages_PreciseRemovalCount(t *testing.T) {
	// 5 user/assistant pairs of ~40000 tokens each = ~400000 total.
	// Budget is 160000, so need to remove ~240000 tokens = 6 messages (3 pairs).
	// Remaining: 2 pairs = ~160000 tokens.
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 40000),     // pair 1
		makeTextMsg("assistant", 40000),
		makeTextMsg("user", 40000),     // pair 2
		makeTextMsg("assistant", 40000),
		makeTextMsg("user", 40000),     // pair 3
		makeTextMsg("assistant", 40000),
		makeTextMsg("user", 40000),     // pair 4
		makeTextMsg("assistant", 40000),
		makeTextMsg("user", 40000),     // pair 5
		makeTextMsg("assistant", 40000),
	}

	result, removed := truncateMessages(messages, 0)
	if removed == 0 {
		t.Fatal("expected truncation")
	}
	// Verify result starts with user
	if result[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("expected user first, got %s", result[0].Role)
	}
	// Verify removed + remaining = original
	if removed+len(result) != len(messages) {
		t.Errorf("removed (%d) + remaining (%d) != original (%d)", removed, len(result), len(messages))
	}
}

func TestTruncateMessages_MultiToolUseChain(t *testing.T) {
	// Simulate: user, assistant(tool), user(result), assistant(tool), user(result), assistant, user, assistant
	// The only valid truncation points are the plain user messages (indices 0 and 6).
	// Total must exceed 160K budget to trigger truncation.
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 80000), // index 0 - valid start, large
		anthropic.NewAssistantMessage(
			anthropic.NewToolUseBlock("t1", map[string]any{"a": "b"}, "tool1"),
		), // index 1
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("t1", "result1", false),
		), // index 2 - NOT valid (tool result)
		anthropic.NewAssistantMessage(
			anthropic.NewToolUseBlock("t2", map[string]any{"c": "d"}, "tool2"),
		), // index 3
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("t2", "result2", false),
		), // index 4 - NOT valid (tool result)
		makeTextMsg("assistant", 80000), // index 5 - large
		makeTextMsg("user", 100),        // index 6 - valid start
		makeTextMsg("assistant", 100),   // index 7
	}

	result, removed := truncateMessages(messages, 0)
	if removed == 0 {
		t.Fatal("expected truncation")
	}
	// Should skip all tool result messages and land on the plain user at index 6
	if isToolResultMessage(result[0]) {
		t.Error("should not start at a tool result message")
	}
	if result[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("expected user, got %s", result[0].Role)
	}
}

func TestTruncateMessages_FallbackWhenNoBoundaryFitsBudget(t *testing.T) {
	// All messages are huge. Even the last user/assistant pair exceeds budget.
	// Should fall back to the last valid starting point.
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 100000),
		makeTextMsg("assistant", 100000),
		makeTextMsg("user", 100000),     // last valid start
		makeTextMsg("assistant", 100000),
	}

	result, removed := truncateMessages(messages, 0)
	// Should fall back to the last valid user message (index 2)
	if removed != 2 {
		t.Errorf("expected 2 removed (fallback to last valid start), got %d", removed)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(result))
	}
	if result[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("expected user first, got %s", result[0].Role)
	}
}

func TestTruncateMessages_AllToolResults(t *testing.T) {
	// Pathological: all user messages are tool results except none.
	// Should hit the ultimate fallback and return the last message.
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("t1", strings.Repeat("x", 100000*charsPerToken), false),
		),
		makeTextMsg("assistant", 100000),
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("t2", strings.Repeat("x", 100000*charsPerToken), false),
		),
		makeTextMsg("assistant", 100),
	}

	result, removed := truncateMessages(messages, 0)
	// Should still return something (ultimate fallback)
	if len(result) == 0 {
		t.Fatal("expected at least one message in result")
	}
	if removed+len(result) != len(messages) {
		t.Errorf("removed (%d) + remaining (%d) != original (%d)", removed, len(result), len(messages))
	}
}

func TestTruncateMessages_OverheadExceedsBudget(t *testing.T) {
	// Fixed overhead larger than maxInputTokenBudget: budget clamps to 1.
	// Should still return something reasonable.
	messages := []anthropic.MessageParam{
		makeTextMsg("user", 100),
		makeTextMsg("assistant", 100),
	}

	result, removed := truncateMessages(messages, maxInputTokenBudget+10000)
	// Messages exceed budget of 1, so truncation happens.
	// Should fall back to last valid user message.
	if len(result) == 0 {
		t.Fatal("expected at least one message")
	}
	if result[0].Role != anthropic.MessageParamRoleUser {
		t.Errorf("expected user first, got %s", result[0].Role)
	}
	_ = removed
}

func TestTruncateMessages_PreservesMessageOrder(t *testing.T) {
	// After truncation, remaining messages should preserve their original order.
	messages := make([]anthropic.MessageParam, 0, 20)
	for range 10 {
		messages = append(messages, makeTextMsg("user", 20000))
		messages = append(messages, makeTextMsg("assistant", 20000))
	}

	result, removed := truncateMessages(messages, 0)
	if removed == 0 {
		t.Fatal("expected truncation")
	}
	// Check alternating user/assistant pattern is maintained
	for i, msg := range result {
		expectedRole := anthropic.MessageParamRoleUser
		if i%2 == 1 {
			expectedRole = anthropic.MessageParamRoleAssistant
		}
		if msg.Role != expectedRole {
			t.Errorf("message %d: expected role %s, got %s", i, expectedRole, msg.Role)
		}
	}
}
