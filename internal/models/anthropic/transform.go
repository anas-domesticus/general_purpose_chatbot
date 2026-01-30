package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// transformADKToAnthropic converts ADK Content to Anthropic MessageParam
func transformADKToAnthropic(contents []*genai.Content) ([]anthropic.MessageParam, string, error) {
	if len(contents) == 0 {
		return nil, "", fmt.Errorf("no contents provided")
	}

	var messages []anthropic.MessageParam
	var systemPrompt string

	for _, content := range contents {
		// Handle system messages separately
		if content.Role == "system" {
			systemParts := extractTextParts(content.Parts)
			if len(systemParts) > 0 {
				if systemPrompt != "" {
					systemPrompt += "\n\n"
				}
				systemPrompt += strings.Join(systemParts, "\n")
			}
			continue
		}

		// Convert to Anthropic message
		message, err := convertContentToMessage(content)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert content: %w", err)
		}

		if message != nil {
			messages = append(messages, *message)
		}
	}

	return messages, systemPrompt, nil
}

// convertContentToMessage converts a single genai.Content to anthropic.MessageParam
func convertContentToMessage(content *genai.Content) (*anthropic.MessageParam, error) {
	if content == nil || len(content.Parts) == 0 {
		return nil, nil
	}

	// Map roles
	var role anthropic.MessageParamRole
	switch content.Role {
	case "user":
		role = anthropic.MessageParamRoleUser
	case "model", "assistant":
		role = anthropic.MessageParamRoleAssistant
	case "system":
		// System messages are handled separately, skip here
		return nil, nil
	default:
		// Default to user for unknown roles
		role = anthropic.MessageParamRoleUser
	}

	// Convert parts to content blocks
	var contentBlocks []anthropic.ContentBlockParamUnion

	for _, part := range content.Parts {
		block, err := convertPartToContentBlock(part)
		if err != nil {
			return nil, fmt.Errorf("failed to convert part: %w", err)
		}
		if block != nil {
			contentBlocks = append(contentBlocks, *block)
		}
	}

	if len(contentBlocks) == 0 {
		return nil, nil
	}

	message := anthropic.MessageParam{
		Role:    role,
		Content: contentBlocks,
	}

	return &message, nil
}

// convertPartToContentBlock converts genai.Part to Anthropic ContentBlockParamUnion
func convertPartToContentBlock(part *genai.Part) (*anthropic.ContentBlockParamUnion, error) {
	// Handle text content
	if part.Text != "" {
		return &anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{
				Text: part.Text,
			},
		}, nil
	}

	// Handle inline data (images) - simplified approach
	if part.InlineData != nil {
		// For now, convert images to text description
		// In a full implementation, you would handle image uploads properly
		imageDesc := fmt.Sprintf("[Image: %s, %d bytes]", part.InlineData.MIMEType, len(part.InlineData.Data))
		return &anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{
				Text: imageDesc,
			},
		}, nil
	}

	// Handle file data
	if part.FileData != nil {
		fileInfo := fmt.Sprintf("[File: %s, MIME: %s]", part.FileData.FileURI, part.FileData.MIMEType)
		return &anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{
				Text: fileInfo,
			},
		}, nil
	}

	// For other types like function calls, convert to text for now
	// In a full implementation, you would handle tool use properly
	if part.FunctionCall != nil {
		funcInfo := fmt.Sprintf("[Function call: %s]", part.FunctionCall.Name)
		return &anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{
				Text: funcInfo,
			},
		}, nil
	}

	if part.FunctionResponse != nil {
		respInfo := fmt.Sprintf("[Function response: %s]", part.FunctionResponse.Name)
		return &anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{
				Text: respInfo,
			},
		}, nil
	}

	// If no recognized content type, return nil
	return nil, nil
}

// transformAnthropicToADK converts Anthropic Message response to ADK LLMResponse
func transformAnthropicToADK(message *anthropic.Message) (*model.LLMResponse, error) {
	if message == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Convert content blocks to genai.Parts
	var parts []*genai.Part

	for _, block := range message.Content {
		part, err := convertContentBlockToPart(block)
		if err != nil {
			return nil, fmt.Errorf("failed to convert content block: %w", err)
		}
		if part != nil {
			parts = append(parts, part)
		}
	}

	// Create genai.Content
	content := &genai.Content{
		Role:  "model", // ADK expects "model" for assistant responses
		Parts: parts,
	}

	// Convert usage metadata if available
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	usageMetadata = &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     int32(message.Usage.InputTokens),
		CandidatesTokenCount: int32(message.Usage.OutputTokens),
		TotalTokenCount:      int32(message.Usage.InputTokens + message.Usage.OutputTokens),
	}

	// Map finish reason
	var finishReason genai.FinishReason
	switch message.StopReason {
	case anthropic.StopReasonEndTurn:
		finishReason = genai.FinishReasonStop
	case anthropic.StopReasonMaxTokens:
		finishReason = genai.FinishReasonMaxTokens
	case anthropic.StopReasonStopSequence:
		finishReason = genai.FinishReasonStop
	case anthropic.StopReasonToolUse:
		finishReason = genai.FinishReasonStop
	default:
		finishReason = genai.FinishReasonOther
	}

	response := &model.LLMResponse{
		Content:       content,
		UsageMetadata: usageMetadata,
		FinishReason:  finishReason,
		TurnComplete:  true,
		Partial:       false,
	}

	return response, nil
}

// convertContentBlockToPart converts Anthropic ContentBlockUnion to genai.Part
func convertContentBlockToPart(block anthropic.ContentBlockUnion) (*genai.Part, error) {
	switch block.Type {
	case "text":
		return &genai.Part{
			Text: block.Text,
		}, nil

	case "thinking":
		// Preserve Claude's extended thinking/reasoning in a structured format
		return &genai.Part{
			Text: fmt.Sprintf("<thinking>\n%s\n</thinking>", block.Thinking),
		}, nil

	case "tool_use":
		// Convert tool use to function call
		var args map[string]any
		if len(block.Input) > 0 {
			if err := json.Unmarshal(block.Input, &args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool input: %w", err)
			}
		}
		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: block.Name,
				Args: args,
			},
		}, nil

	default:
		// For unrecognized types, preserve with type annotation
		return &genai.Part{
			Text: fmt.Sprintf("[%s content]", block.Type),
		}, nil
	}
}

// transformToolsToAnthropic converts ADK tools to Anthropic tool format
func transformToolsToAnthropic(tools map[string]any) ([]anthropic.ToolUnionParam, error) {
	// For now, return empty tools array
	// In a full implementation, you would properly convert tool schemas
	return []anthropic.ToolUnionParam{}, nil
}

// extractTextParts extracts text content from genai.Parts
func extractTextParts(parts []*genai.Part) []string {
	var textParts []string
	for _, part := range parts {
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	return textParts
}
