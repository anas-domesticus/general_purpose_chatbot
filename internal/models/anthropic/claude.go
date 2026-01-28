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

// ClaudeModel implements the model.LLM interface for Anthropic Claude models
type ClaudeModel struct {
	client    anthropic.Client
	modelName string
	logger    *slog.Logger
}

// NewClaudeModel creates a new Claude model instance
func NewClaudeModel(apiKey, modelName string, opts ...option.RequestOption) (*ClaudeModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}
	if modelName == "" {
		modelName = string(anthropic.ModelClaudeSonnet4_5_20250929) // Default to latest Sonnet 4.5
	}

	client := anthropic.NewClient(
		append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)...,
	)

	logger := slog.With("component", "claude_model", "model", modelName)

	return &ClaudeModel{
		client:    client,
		modelName: modelName,
		logger:    logger,
	}, nil
}

// Name returns the name of the model
func (c *ClaudeModel) Name() string {
	return c.modelName
}

// GenerateContent implements the model.LLM interface
func (c *ClaudeModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	c.logger.Info("generating content", "stream", stream, "contents_count", len(req.Contents))

	// For now, implement non-streaming only to get basic functionality working
	// We'll add streaming support later
	return c.generateContentNonStream(ctx, req)
}

// generateContentNonStream handles non-streaming generation
func (c *ClaudeModel) generateContentNonStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Transform ADK request to Anthropic format
		messages, systemPrompt, err := transformADKToAnthropic(req.Contents)
		if err != nil {
			yield(nil, fmt.Errorf("failed to transform request: %w", err))
			return
		}

		// Build Anthropic request
		anthropicReq := anthropic.MessageNewParams{
			Model:     anthropic.Model(c.modelName),
			MaxTokens: 4000, // Default max tokens
			Messages:  messages,
		}

		// Add system prompt if present
		if systemPrompt != "" {
			anthropicReq.System = []anthropic.TextBlockParam{
				{Text: systemPrompt},
			}
		}

		// Apply config overrides if present
		if req.Config != nil {
			if req.Config.MaxOutputTokens > 0 {
				anthropicReq.MaxTokens = int64(req.Config.MaxOutputTokens)
			}
			if req.Config.Temperature != nil {
				anthropicReq.Temperature = anthropic.Float(float64(*req.Config.Temperature))
			}
			if req.Config.TopP != nil {
				anthropicReq.TopP = anthropic.Float(float64(*req.Config.TopP))
			}
		}

		// Add tools if present
		if len(req.Tools) > 0 {
			tools, err := transformToolsToAnthropic(req.Tools)
			if err != nil {
				yield(nil, fmt.Errorf("failed to transform tools: %w", err))
				return
			}
			anthropicReq.Tools = tools
		}

		c.logger.Debug("sending request to anthropic", "messages_count", len(messages))

		// Call Anthropic API
		resp, err := c.client.Messages.New(ctx, anthropicReq)
		if err != nil {
			yield(nil, fmt.Errorf("claude api error: %w", err))
			return
		}

		// Transform response back to ADK format
		llmResponse, err := transformAnthropicToADK(resp)
		if err != nil {
			yield(nil, fmt.Errorf("failed to transform response: %w", err))
			return
		}

		c.logger.Debug("received response from anthropic", "content_blocks", len(resp.Content))

		// Yield the response
		yield(llmResponse, nil)
	}
}