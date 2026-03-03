package anthropic

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	// maxInputTokenBudget is a conservative token budget for input content.
	// Set well below the 200K API limit to account for token estimation
	// inaccuracy and output token reservation.
	maxInputTokenBudget = 160000

	// charsPerToken is the approximate character-to-token ratio used for
	// token estimation. A lower value produces higher (more conservative)
	// estimates, reducing the chance of hitting the API limit.
	charsPerToken = 4
)

// estimateStringTokens estimates the token count for a string.
func estimateStringTokens(s string) int {
	return (len(s) + charsPerToken - 1) / charsPerToken
}

// estimateBlockTokens estimates the token count for a single content block.
func estimateBlockTokens(block anthropic.ContentBlockParamUnion) int {
	tokens := 0

	if block.OfText != nil {
		tokens += estimateStringTokens(block.OfText.Text)
	}

	if block.OfImage != nil {
		// Conservative estimate for images. Actual cost depends on
		// dimensions which we can't determine from the param.
		tokens += 1600
	}

	if block.OfToolUse != nil {
		tokens += estimateStringTokens(block.OfToolUse.Name)
		if block.OfToolUse.Input != nil {
			data, err := json.Marshal(block.OfToolUse.Input)
			if err == nil {
				tokens += estimateStringTokens(string(data))
			}
		}
	}

	if block.OfToolResult != nil {
		for _, sub := range block.OfToolResult.Content {
			tokens += estimateToolResultBlockTokens(sub)
		}
	}

	return tokens
}

// estimateMessageTokens estimates the token count for a single message,
// including a small overhead for role and message framing tokens.
func estimateMessageTokens(msg anthropic.MessageParam) int {
	tokens := 4 // overhead for role and framing
	for _, block := range msg.Content {
		tokens += estimateBlockTokens(block)
	}
	return tokens
}

// estimateToolResultBlockTokens estimates tokens for a tool result content block,
// which uses a separate union type from regular content blocks.
func estimateToolResultBlockTokens(block anthropic.ToolResultBlockParamContentUnion) int {
	tokens := 0
	if block.OfText != nil {
		tokens += estimateStringTokens(block.OfText.Text)
	}
	if block.OfImage != nil {
		tokens += 1600
	}
	return tokens
}

// estimateSystemBlocksTokens estimates the total token count for system prompt blocks.
func estimateSystemBlocksTokens(blocks []anthropic.TextBlockParam) int {
	total := 0
	for _, block := range blocks {
		total += estimateStringTokens(block.Text)
	}
	return total
}

// estimateToolsTokens estimates the total token count for tool definitions.
func estimateToolsTokens(tools []anthropic.ToolUnionParam) int {
	total := 0
	for _, t := range tools {
		if t.OfTool != nil {
			total += estimateStringTokens(t.OfTool.Name)
			if t.OfTool.Description.Valid() {
				total += estimateStringTokens(t.OfTool.Description.Value)
			}
			if t.OfTool.InputSchema.Properties != nil {
				data, err := json.Marshal(t.OfTool.InputSchema.Properties)
				if err == nil {
					total += estimateStringTokens(string(data))
				}
			}
		}
	}
	return total
}

// isToolResultMessage reports whether a message contains any tool result blocks.
func isToolResultMessage(msg anthropic.MessageParam) bool {
	for _, block := range msg.Content {
		if block.OfToolResult != nil {
			return true
		}
	}
	return false
}

// truncateMessages removes the oldest messages to fit within the token budget.
// It preserves message validity by ensuring:
//   - The result starts with a user message (Anthropic API requirement)
//   - Tool use/result pairs are not orphaned (only truncates at clean boundaries)
//
// fixedTokenOverhead accounts for system prompt and tool definition tokens.
// Returns the (possibly truncated) messages and the number of messages removed.
func truncateMessages(
	messages []anthropic.MessageParam,
	fixedTokenOverhead int,
) ([]anthropic.MessageParam, int) {
	budget := maxInputTokenBudget - fixedTokenOverhead
	if budget <= 0 {
		budget = 1 // ensure we don't reject everything due to overhead alone
	}

	// Calculate per-message token estimates and total
	msgTokens := make([]int, len(messages))
	totalMessageTokens := 0
	for i, msg := range messages {
		msgTokens[i] = estimateMessageTokens(msg)
		totalMessageTokens += msgTokens[i]
	}

	if totalMessageTokens <= budget {
		return messages, 0
	}

	// Scan from the front, accumulating removed tokens.
	// At each position, check if the next message is a valid starting point:
	// it must be a user message that is not a tool result (to avoid orphaned
	// tool use/result pairs).
	removedTokens := 0
	for i := 0; i < len(messages)-1; i++ {
		removedTokens += msgTokens[i]
		remaining := totalMessageTokens - removedTokens

		nextMsg := messages[i+1]
		isValidStart := nextMsg.Role == anthropic.MessageParamRoleUser && !isToolResultMessage(nextMsg)

		if isValidStart && remaining <= budget {
			return messages[i+1:], i + 1
		}
	}

	// Could not fit within budget at any clean boundary.
	// Return from the last valid starting point to preserve the most
	// recent context possible.
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == anthropic.MessageParamRoleUser && !isToolResultMessage(messages[i]) {
			return messages[i:], i
		}
	}

	// Ultimate fallback: return the last message only.
	return messages[len(messages)-1:], len(messages) - 1
}
