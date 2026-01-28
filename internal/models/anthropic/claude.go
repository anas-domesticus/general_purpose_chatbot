package anthropic

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeModel implements the model.LLM interface for Claude models
type ClaudeModel struct {
	client    anthropic.Client
	modelName string
}

// NewClaudeModel creates a new Claude model instance
func NewClaudeModel(apiKey, modelName string) (*ClaudeModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}
	if modelName == "" {
		modelName = "claude-3-5-sonnet-20241022" // Default model
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &ClaudeModel{
		client:    client,
		modelName: modelName,
	}, nil
}

// Name returns the model name
func (c *ClaudeModel) Name() string {
	return c.modelName
}

// GenerateContent implements the model.LLM interface
func (c *ClaudeModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Transform ADK request to Anthropic format
		messages, system, err := c.transformContents(req.Contents)
		if err != nil {
			yield(nil, fmt.Errorf("failed to transform contents: %w", err))
			return
		}

		// Create basic Anthropic request parameters
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(c.modelName),
			MaxTokens: 4096,
			Messages:  messages,
		}

		// Add system message if present
		if system != "" {
			params.System = []anthropic.TextBlockParam{
				{Text: system},
			}
		}

		// Apply configuration if provided
		if req.Config != nil {
			if req.Config.MaxOutputTokens > 0 {
				params.MaxTokens = int64(req.Config.MaxOutputTokens)
			}
			if req.Config.Temperature != nil {
				params.Temperature = anthropic.Float(float64(*req.Config.Temperature))
			}
			if req.Config.TopP != nil {
				params.TopP = anthropic.Float(float64(*req.Config.TopP))
			}
		}

		// Make the API call
		resp, err := c.client.Messages.New(ctx, params)
		if err != nil {
			yield(nil, fmt.Errorf("claude API error: %w", err))
			return
		}

		// Transform response back to ADK format
		adkResponse := c.transformResponse(resp)
		yield(adkResponse, nil)
	}
}

// transformContents converts genai.Content to Anthropic format
func (c *ClaudeModel) transformContents(contents []*genai.Content) ([]anthropic.MessageParam, string, error) {
	var messages []anthropic.MessageParam
	var systemMessage strings.Builder

	for _, content := range contents {
		role := strings.ToLower(content.Role)
		
		// Handle system messages separately
		if role == "system" {
			for _, part := range content.Parts {
				if part.Text != "" {
					if systemMessage.Len() > 0 {
						systemMessage.WriteString("\n")
					}
					systemMessage.WriteString(part.Text)
				}
			}
			continue
		}

		// Convert role to Anthropic format
		var anthropicRole anthropic.MessageParamRole
		switch role {
		case "user":
			anthropicRole = anthropic.MessageParamRoleUser
		case "assistant", "model":
			anthropicRole = anthropic.MessageParamRoleAssistant
		default:
			anthropicRole = anthropic.MessageParamRoleUser // Default to user
		}

		// Create message using the pattern from examples
		if len(content.Parts) > 0 {
			// For simplicity, just take the first text part for now
			var textContent string
			for _, part := range content.Parts {
				if part.Text != "" {
					textContent = part.Text
					break
				}
			}

			if textContent != "" {
				message := anthropic.NewUserMessage(anthropic.NewTextBlock(textContent))
				if anthropicRole == anthropic.MessageParamRoleAssistant {
					message = anthropic.NewAssistantMessage(anthropic.NewTextBlock(textContent))
				}
				messages = append(messages, message)
			}
		}
	}

	return messages, systemMessage.String(), nil
}

// transformResponse converts Anthropic response to ADK format
func (c *ClaudeModel) transformResponse(resp *anthropic.Message) *model.LLMResponse {
	response := &model.LLMResponse{}

	// Extract text content from response
	var textParts []string
	for _, content := range resp.Content {
		if content.Type == "text" {
			textParts = append(textParts, content.Text)
		}
	}

	// Create genai.Content for the response
	text := strings.Join(textParts, "\n")
	if text != "" {
		response.Content = &genai.Content{
			Role: "model",
			Parts: []*genai.Part{
				{Text: text},
			},
		}
	}

	// Set usage statistics if available
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		response.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.InputTokens),
			CandidatesTokenCount: int32(resp.Usage.OutputTokens),
			TotalTokenCount:      int32(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		}
	}

	return response
}