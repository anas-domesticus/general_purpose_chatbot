package anthropic

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"google.golang.org/adk/model"
)

// ClaudeModel implements the model.LLM interface for Anthropic's Claude models.
type ClaudeModel struct {
	client    *anthropic.Client
	modelName string
	logger    *slog.Logger
}

// NewClaudeModel creates a new Claude model instance.
func NewClaudeModel(apiKey, modelName string) (*ClaudeModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if modelName == "" {
		return nil, fmt.Errorf("model name is required")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &ClaudeModel{
		client:    &client,
		modelName: modelName,
		logger:    slog.Default(),
	}, nil
}

// Name returns the model name.
func (c *ClaudeModel) Name() string {
	return c.modelName
}

// GenerateContent generates content using the Claude model.
// This implementation only supports non-streaming mode.
func (c *ClaudeModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if stream {
			yield(nil, fmt.Errorf("streaming not supported"))
			return
		}

		response, err := c.generateContentNonStreaming(ctx, req)
		yield(response, err)
	}
}

// generateContentNonStreaming performs a non-streaming content generation request.
func (c *ClaudeModel) generateContentNonStreaming(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	// Transform ADK request to Anthropic format
	messages, systemBlocks, err := transformADKToAnthropic(req.Contents)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// IMPORTANT: Extract system instruction from Config.SystemInstruction
	// This is where ADK places the llmagent's Instruction field
	if req.Config != nil && req.Config.SystemInstruction != nil {
		for _, part := range req.Config.SystemInstruction.Parts {
			if part != nil && part.Text != "" {
				systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
					Text: part.Text,
				})
			}
		}
	}

	// Add cache control to the last system block if we have any
	if len(systemBlocks) > 0 {
		lastIdx := len(systemBlocks) - 1
		cacheControl := anthropic.NewCacheControlEphemeralParam()
		systemBlocks[lastIdx].CacheControl = cacheControl
	}

	// Determine max tokens - default to 4096 if not specified
	maxTokens := int64(4096)
	if req.Config != nil && req.Config.MaxOutputTokens > 0 {
		maxTokens = int64(req.Config.MaxOutputTokens)
	}

	// Build the message params
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	// Add system blocks if present
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}

	// Add temperature if specified
	if req.Config != nil && req.Config.Temperature != nil {
		params.Temperature = anthropic.Float(float64(*req.Config.Temperature))
	}

	// Add top_p if specified
	if req.Config != nil && req.Config.TopP != nil {
		params.TopP = anthropic.Float(float64(*req.Config.TopP))
	}

	// Add top_k if specified
	if req.Config != nil && req.Config.TopK != nil {
		params.TopK = anthropic.Int(int64(*req.Config.TopK))
	}

	// Add stop sequences if specified
	if req.Config != nil && len(req.Config.StopSequences) > 0 {
		params.StopSequences = req.Config.StopSequences
	}

	// Transform and add tools if present
	if req.Tools != nil {
		tools, err := transformToolsToAnthropic(req.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to transform tools: %w", err)
		}
		if len(tools) > 0 {
			params.Tools = tools
		}
	}

	// Make the API call
	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Transform the response
	response, err := transformAnthropicToADK(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to transform response: %w", err)
	}

	return response, nil
}
