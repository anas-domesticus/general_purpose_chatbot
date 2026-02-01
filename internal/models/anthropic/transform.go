// Package anthropic provides an Anthropic Claude implementation for the ADK model.LLM interface.
package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// transformADKToAnthropic converts ADK content messages to Anthropic message params.
// It extracts system messages separately since Anthropic requires them as a top-level parameter.
// Returns the messages, system prompt, and any error.
func transformADKToAnthropic(contents []*genai.Content) ([]anthropic.MessageParam, string, error) {
	var messages []anthropic.MessageParam
	var systemPrompt strings.Builder

	for _, content := range contents {
		if content == nil {
			continue
		}

		// Handle system messages - extract them for the system parameter
		if content.Role == "system" {
			for _, part := range content.Parts {
				if part != nil && part.Text != "" {
					if systemPrompt.Len() > 0 {
						systemPrompt.WriteString("\n\n")
					}
					systemPrompt.WriteString(part.Text)
				}
			}
			continue
		}

		// Convert user/assistant messages
		msg, err := convertContentToMessage(content)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert content: %w", err)
		}
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	return messages, systemPrompt.String(), nil
}

// convertContentToMessage converts a single genai.Content to an Anthropic MessageParam.
func convertContentToMessage(content *genai.Content) (*anthropic.MessageParam, error) {
	if content == nil || len(content.Parts) == 0 {
		return nil, nil
	}

	var blocks []anthropic.ContentBlockParamUnion

	for _, part := range content.Parts {
		if part == nil {
			continue
		}

		block, err := convertPartToContentBlock(part)
		if err != nil {
			return nil, err
		}
		if block != (anthropic.ContentBlockParamUnion{}) {
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 {
		return nil, nil
	}

	// Map ADK roles to Anthropic roles
	var msg anthropic.MessageParam
	switch content.Role {
	case "user":
		msg = anthropic.NewUserMessage(blocks...)
	case "model", "assistant":
		msg = anthropic.NewAssistantMessage(blocks...)
	default:
		// Default to user for unknown roles
		msg = anthropic.NewUserMessage(blocks...)
	}

	return &msg, nil
}

// convertPartToContentBlock converts a genai.Part to an Anthropic ContentBlockParamUnion.
func convertPartToContentBlock(part *genai.Part) (anthropic.ContentBlockParamUnion, error) {
	if part == nil {
		return anthropic.ContentBlockParamUnion{}, nil
	}

	// Handle text content
	if part.Text != "" {
		return anthropic.NewTextBlock(part.Text), nil
	}

	// Handle inline image data
	if part.InlineData != nil {
		mediaType := part.InlineData.MIMEType
		// Map MIME types to Anthropic's supported types
		var anthropicMediaType string
		switch mediaType {
		case "image/jpeg":
			anthropicMediaType = "image/jpeg"
		case "image/png":
			anthropicMediaType = "image/png"
		case "image/gif":
			anthropicMediaType = "image/gif"
		case "image/webp":
			anthropicMediaType = "image/webp"
		default:
			// Default to jpeg if unknown
			anthropicMediaType = "image/jpeg"
		}

		// Encode data to base64
		encodedData := base64.StdEncoding.EncodeToString(part.InlineData.Data)
		return anthropic.NewImageBlockBase64(anthropicMediaType, encodedData), nil
	}

	// Handle function call (tool use)
	if part.FunctionCall != nil {
		return anthropic.NewToolUseBlock(
			part.FunctionCall.ID,
			part.FunctionCall.Args,
			part.FunctionCall.Name,
		), nil
	}

	// Handle function response (tool result)
	if part.FunctionResponse != nil {
		// Serialize the response to JSON string
		responseJSON, err := json.Marshal(part.FunctionResponse.Response)
		if err != nil {
			return anthropic.ContentBlockParamUnion{}, fmt.Errorf("failed to marshal function response: %w", err)
		}
		return anthropic.NewToolResultBlock(part.FunctionResponse.ID, string(responseJSON), false), nil
	}

	// Unknown part type - skip
	return anthropic.ContentBlockParamUnion{}, nil
}

// transformAnthropicToADK converts an Anthropic Message response to an ADK LLMResponse.
func transformAnthropicToADK(msg *anthropic.Message) (*model.LLMResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil message")
	}

	var parts []*genai.Part

	for _, block := range msg.Content {
		part, err := convertContentBlockToPart(block)
		if err != nil {
			return nil, fmt.Errorf("failed to convert content block: %w", err)
		}
		if part != nil {
			parts = append(parts, part)
		}
	}

	// Map Anthropic stop reason to genai FinishReason
	finishReason := mapStopReason(msg.StopReason)

	response := &model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: parts,
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(msg.Usage.InputTokens),
			CandidatesTokenCount: int32(msg.Usage.OutputTokens),
			TotalTokenCount:      int32(msg.Usage.InputTokens + msg.Usage.OutputTokens),
		},
		FinishReason: finishReason,
		TurnComplete: true,
	}

	return response, nil
}

// convertContentBlockToPart converts an Anthropic ContentBlockUnion to a genai.Part.
func convertContentBlockToPart(block anthropic.ContentBlockUnion) (*genai.Part, error) {
	switch block.Type {
	case "text":
		return &genai.Part{Text: block.Text}, nil

	case "tool_use":
		// Parse the input JSON
		var args map[string]any
		if len(block.Input) > 0 {
			if err := json.Unmarshal(block.Input, &args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool input: %w", err)
			}
		}

		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   block.ID,
				Name: block.Name,
				Args: args,
			},
		}, nil

	case "thinking":
		// Return thinking content as text with a marker
		return &genai.Part{
			Text:    block.Thinking,
			Thought: true,
		}, nil

	default:
		// Unknown block type - skip
		return nil, nil
	}
}

// mapStopReason converts Anthropic's StopReason to genai.FinishReason.
func mapStopReason(stopReason anthropic.StopReason) genai.FinishReason {
	switch stopReason {
	case anthropic.StopReasonEndTurn:
		return genai.FinishReasonStop
	case anthropic.StopReasonMaxTokens:
		return genai.FinishReasonMaxTokens
	case anthropic.StopReasonToolUse:
		return genai.FinishReasonStop
	case anthropic.StopReasonStopSequence:
		return genai.FinishReasonStop
	case anthropic.StopReasonRefusal:
		return genai.FinishReasonSafety
	default:
		return genai.FinishReasonOther
	}
}

// transformToolsToAnthropic converts ADK tool definitions to Anthropic ToolUnionParam.
// ADK tools are stored as tool.Tool interface objects with a Declaration() method that
// returns *genai.FunctionDeclaration containing the tool's schema.
func transformToolsToAnthropic(tools map[string]any) ([]anthropic.ToolUnionParam, error) {
	if tools == nil {
		return nil, nil
	}

	// Define interface for tools that have declarations
	// ADK tools implement this interface to expose their function declarations
	type toolWithDeclaration interface {
		Declaration() *genai.FunctionDeclaration
	}

	var anthropicTools []anthropic.ToolUnionParam

	for _, toolDef := range tools {
		// Type assert to tool with declaration method
		toolObj, ok := toolDef.(toolWithDeclaration)
		if !ok {
			continue
		}

		// Get the function declaration
		decl := toolObj.Declaration()
		if decl == nil {
			continue
		}

		// Extract name and description
		name := decl.Name
		if name == "" {
			continue
		}

		description := decl.Description

		// Build the input schema from ParametersJsonSchema
		inputSchema := anthropic.ToolInputSchemaParam{
			Type: "object",
		}

		if decl.ParametersJsonSchema != nil {
			// Type assert the schema to map[string]any
			if schema, ok := decl.ParametersJsonSchema.(map[string]any); ok {
				// Extract properties
				if props, ok := schema["properties"]; ok {
					inputSchema.Properties = props
				}

				// Extract required fields
				if req, ok := schema["required"].([]any); ok {
					required := make([]string, 0, len(req))
					for _, r := range req {
						if s, ok := r.(string); ok {
							required = append(required, s)
						}
					}
					inputSchema.Required = required
				}
			}
		}

		// Create the tool param
		toolParam := anthropic.ToolUnionParamOfTool(inputSchema, name)
		if description != "" {
			toolParam.OfTool.Description = anthropic.String(description)
		}

		anthropicTools = append(anthropicTools, toolParam)
	}

	return anthropicTools, nil
}
