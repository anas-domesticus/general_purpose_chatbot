// Package openai provides an OpenAI GPT implementation for the ADK model.LLM interface.
package openai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Finish reason constants (OpenAI uses plain strings)
const (
	finishReasonStop          = "stop"
	finishReasonLength        = "length"
	finishReasonToolCalls     = "tool_calls"
	finishReasonContentFilter = "content_filter"
	finishReasonFunctionCall  = "function_call"
)

// transformADKToOpenAI converts ADK content messages to OpenAI chat completion message params.
// System messages are included inline in the messages array (OpenAI's standard format).
func transformADKToOpenAI(contents []*genai.Content) ([]openai.ChatCompletionMessageParamUnion, error) {
	var messages []openai.ChatCompletionMessageParamUnion

	for _, content := range contents {
		if content == nil {
			continue
		}

		msg, err := convertContentToMessage(content)
		if err != nil {
			return nil, fmt.Errorf("failed to convert content: %w", err)
		}
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	return messages, nil
}

// convertContentToMessage converts a single genai.Content to an OpenAI ChatCompletionMessageParamUnion.
func convertContentToMessage(content *genai.Content) (*openai.ChatCompletionMessageParamUnion, error) {
	if content == nil || len(content.Parts) == 0 {
		return nil, nil
	}

	switch content.Role {
	case "system":
		// Combine all text parts into a single system message
		var text string
		for _, part := range content.Parts {
			if part != nil && part.Text != "" {
				if text != "" {
					text += "\n\n"
				}
				text += part.Text
			}
		}
		if text == "" {
			return nil, nil
		}
		msg := openai.SystemMessage(text)
		return &msg, nil

	case "user":
		parts := convertPartsToUserContent(content.Parts)
		if len(parts) == 0 {
			return nil, nil
		}
		// If single text part, use simple string message
		if len(parts) == 1 && parts[0].OfText != nil {
			msg := openai.UserMessage(parts[0].OfText.Text)
			return &msg, nil
		}
		// For multiple parts or images, use content array
		msg := openai.UserMessage(parts)
		return &msg, nil

	case "model", "assistant":
		return convertAssistantContent(content.Parts)

	default:
		// Default to user for unknown roles
		parts := convertPartsToUserContent(content.Parts)
		if len(parts) == 0 {
			return nil, nil
		}
		msg := openai.UserMessage(parts)
		return &msg, nil
	}
}

// convertPartsToUserContent converts genai.Parts to OpenAI user content parts.
func convertPartsToUserContent(parts []*genai.Part) []openai.ChatCompletionContentPartUnionParam {
	var result []openai.ChatCompletionContentPartUnionParam

	for _, part := range parts {
		if part == nil {
			continue
		}

		// Handle text content
		if part.Text != "" {
			result = append(result, openai.TextContentPart(part.Text))
			continue
		}

		// Handle inline image data
		if part.InlineData != nil {
			imageURL := fmt.Sprintf("data:%s;base64,%s",
				part.InlineData.MIMEType,
				base64.StdEncoding.EncodeToString(part.InlineData.Data))
			result = append(result, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL: imageURL,
			}))
			continue
		}

		// Handle function response (tool result) - this goes in a separate tool message
		// Skip here as tool results need special handling
		if part.FunctionResponse != nil {
			continue
		}
	}

	return result
}

// convertAssistantContent converts assistant parts to OpenAI assistant message.
func convertAssistantContent(parts []*genai.Part) (*openai.ChatCompletionMessageParamUnion, error) {
	var textContent string
	var toolCalls []openai.ChatCompletionMessageToolCallParam

	for _, part := range parts {
		if part == nil {
			continue
		}

		// Handle text content
		if part.Text != "" {
			if textContent != "" {
				textContent += "\n"
			}
			textContent += part.Text
			continue
		}

		// Handle function call (tool use)
		if part.FunctionCall != nil {
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function args: %w", err)
			}
			toolCall := openai.ChatCompletionMessageToolCallParam{
				ID:   part.FunctionCall.ID,
				Type: "function",
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	if textContent == "" && len(toolCalls) == 0 {
		return nil, nil
	}

	// Build assistant message with tool calls if present
	if len(toolCalls) > 0 {
		assistantParam := openai.ChatCompletionAssistantMessageParam{
			ToolCalls: toolCalls,
		}
		if textContent != "" {
			assistantParam.Content.OfString.Value = textContent
		}
		msg := openai.ChatCompletionMessageParamUnion{OfAssistant: &assistantParam}
		return &msg, nil
	}

	// Simple text-only assistant message
	msg := openai.AssistantMessage(textContent)
	return &msg, nil
}

// transformOpenAIToADK converts an OpenAI ChatCompletion response to an ADK LLMResponse.
func transformOpenAIToADK(completion *openai.ChatCompletion) (*model.LLMResponse, error) {
	if completion == nil {
		return nil, fmt.Errorf("nil completion")
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := completion.Choices[0]
	var parts []*genai.Part

	// Handle text content
	if choice.Message.Content != "" {
		parts = append(parts, &genai.Part{Text: choice.Message.Content})
	}

	// Handle tool calls
	for _, toolCall := range choice.Message.ToolCalls {
		var args map[string]any
		if toolCall.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
			}
		}
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   toolCall.ID,
				Name: toolCall.Function.Name,
				Args: args,
			},
		})
	}

	// Map OpenAI finish reason to genai FinishReason
	finishReason := mapFinishReason(choice.FinishReason)

	// Build usage metadata
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	if completion.Usage.TotalTokens > 0 {
		usageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(completion.Usage.PromptTokens),
			CandidatesTokenCount: int32(completion.Usage.CompletionTokens),
			TotalTokenCount:      int32(completion.Usage.TotalTokens),
		}
		// Handle cached tokens if available
		if completion.Usage.PromptTokensDetails.CachedTokens > 0 {
			usageMetadata.CachedContentTokenCount = int32(completion.Usage.PromptTokensDetails.CachedTokens)
		}
	}

	response := &model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: parts,
		},
		UsageMetadata: usageMetadata,
		FinishReason:  finishReason,
		TurnComplete:  true,
	}

	return response, nil
}

// mapFinishReason converts OpenAI's finish_reason string to genai.FinishReason.
func mapFinishReason(finishReason string) genai.FinishReason {
	switch finishReason {
	case finishReasonStop:
		return genai.FinishReasonStop
	case finishReasonLength:
		return genai.FinishReasonMaxTokens
	case finishReasonToolCalls:
		return genai.FinishReasonStop
	case finishReasonContentFilter:
		return genai.FinishReasonSafety
	case finishReasonFunctionCall:
		return genai.FinishReasonStop
	default:
		return genai.FinishReasonOther
	}
}

// transformToolsToOpenAI converts ADK tool definitions to OpenAI ChatCompletionToolParam.
// ADK tools are stored as tool.Tool interface objects with a Declaration() method that
// returns *genai.FunctionDeclaration containing the tool's schema.
func transformToolsToOpenAI(tools map[string]any) []openai.ChatCompletionToolParam {
	if tools == nil {
		return nil
	}

	// Define interface for tools that have declarations
	// ADK tools implement this interface to expose their function declarations
	type toolWithDeclaration interface {
		Declaration() *genai.FunctionDeclaration
	}

	var openaiTools []openai.ChatCompletionToolParam

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

		// Build the parameters schema
		parameters := openai.FunctionParameters{}

		if decl.ParametersJsonSchema != nil {
			// Type assert the schema to map[string]any
			if schema, ok := decl.ParametersJsonSchema.(map[string]any); ok {
				// Copy all schema properties
				for k, v := range schema {
					parameters[k] = v
				}
			}
		}

		// Ensure type is set to object
		if _, hasType := parameters["type"]; !hasType {
			parameters["type"] = "object"
		}

		// Create the tool param
		toolParam := openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name:        name,
				Description: openai.String(description),
				Parameters:  parameters,
			},
		}

		openaiTools = append(openaiTools, toolParam)
	}

	return openaiTools
}

// CreateToolResultMessage creates a tool result message for OpenAI.
// This is a helper function for handling function responses that need
// to be sent back to the model after tool execution.
func CreateToolResultMessage(toolCallID string, result any) (openai.ChatCompletionMessageParamUnion, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("failed to marshal tool result: %w", err)
	}

	msg := openai.ToolMessage(string(resultJSON), toolCallID)
	return msg, nil
}
