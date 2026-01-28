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

	// Handle inline data (images)
	if part.InlineData != nil {
		// Convert to image block
		return &anthropic.ContentBlockParamUnion{
			OfImage: &anthropic.ImageBlockParam{
				Source: anthropic.ImageBlockParamSource{
					Type:      anthropic.ImageBlockParamSourceTypeBase64,
					MediaType: anthropic.String(part.InlineData.MIMEType),
					Data:      anthropic.String(string(part.InlineData.Data)),
				},
			},
		}, nil
	}

	// Handle file data
	if part.FileData != nil {
		// For files, we'll treat them as text content with file info
		// This is a simplified approach - in production you might want more sophisticated handling
		fileInfo := fmt.Sprintf("[File: %s, MIME: %s]", part.FileData.FileURI, part.FileData.MIMEType)
		return &anthropic.ContentBlockParamUnion{
			OfText: &anthropic.TextBlockParam{
				Text: fileInfo,
			},
		}, nil
	}

	// Handle function calls (tool use)
	if part.FunctionCall != nil {
		// Convert function call to tool use
		argsJSON, err := json.Marshal(part.FunctionCall.Args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal function args: %w", err)
		}

		return &anthropic.ContentBlockParamUnion{
			OfToolUse: &anthropic.ToolUseBlockParam{
				ID:   part.FunctionCall.Name, // Use function name as ID for now
				Name: part.FunctionCall.Name,
				Input: map[string]interface{}{
					"args": string(argsJSON),
				},
			},
		}, nil
	}

	// Handle function responses (tool results)
	if part.FunctionResponse != nil {
		responseJSON, err := json.Marshal(part.FunctionResponse.Response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal function response: %w", err)
		}

		return &anthropic.ContentBlockParamUnion{
			OfToolResult: &anthropic.ToolResultBlockParam{
				ToolUseID: part.FunctionResponse.Name,
				Content: []anthropic.ToolResultContentBlockParamUnion{
					{
						OfText: &anthropic.TextBlockParam{
							Text: string(responseJSON),
						},
					},
				},
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
		Role:  "assistant", // Anthropic responses are always from the assistant
		Parts: parts,
	}

	// Convert usage metadata if available
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	if message.Usage != nil {
		usageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(message.Usage.InputTokens),
			CandidatesTokenCount: int32(message.Usage.OutputTokens),
			TotalTokenCount:      int32(message.Usage.InputTokens + message.Usage.OutputTokens),
		}
	}

	// Map finish reason
	var finishReason genai.FinishReason
	switch message.StopReason {
	case anthropic.MessageStopReasonEndTurn:
		finishReason = genai.FinishReasonStop
	case anthropic.MessageStopReasonMaxTokens:
		finishReason = genai.FinishReasonMaxTokens
	case anthropic.MessageStopReasonStopSequence:
		finishReason = genai.FinishReasonStop
	case anthropic.MessageStopReasonToolUse:
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

// convertContentBlockToPart converts Anthropic ContentBlock to genai.Part
func convertContentBlockToPart(block anthropic.ContentBlock) (*genai.Part, error) {
	switch blockType := block.AsAny().(type) {
	case anthropic.TextBlock:
		return &genai.Part{
			Text: blockType.Text,
		}, nil

	case anthropic.ImageBlock:
		// Convert image block back to inline data
		return &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: *blockType.Source.MediaType,
				Data:     []byte(*blockType.Source.Data),
			},
		}, nil

	case anthropic.ToolUseBlock:
		// Convert tool use to function call
		argsJSON, err := json.Marshal(blockType.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool input: %w", err)
		}

		var args map[string]interface{}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool args: %w", err)
		}

		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: blockType.Name,
				Args: args,
			},
		}, nil

	default:
		// Unknown block type, convert to text
		blockJSON, err := json.Marshal(block)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal unknown block type: %w", err)
		}

		return &genai.Part{
			Text: fmt.Sprintf("[Unknown block type: %s]", string(blockJSON)),
		}, nil
	}
}

// transformToolsToAnthropic converts ADK tools to Anthropic tool format
func transformToolsToAnthropic(tools map[string]any) ([]anthropic.ToolUnionParam, error) {
	var anthropicTools []anthropic.ToolUnionParam

	for name, toolDef := range tools {
		// Convert tool definition to Anthropic format
		// This is a simplified implementation - you may need to adjust based on your tool schema
		toolJSON, err := json.Marshal(toolDef)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool %s: %w", name, err)
		}

		var toolSchema map[string]interface{}
		if err := json.Unmarshal(toolJSON, &toolSchema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool schema for %s: %w", name, err)
		}

		// Extract description and input schema
		description := ""
		if desc, ok := toolSchema["description"].(string); ok {
			description = desc
		}

		// Create Anthropic tool
		anthropicTool := anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        name,
				Description: anthropic.String(description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type: anthropic.String("object"),
					Properties: map[string]interface{}{
						// This is simplified - you should properly convert the schema
						"input": toolSchema,
					},
				},
			},
		}

		anthropicTools = append(anthropicTools, anthropicTool)
	}

	return anthropicTools, nil
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