package openai

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"google.golang.org/adk/model"
)

// Model implements the model.LLM interface for OpenAI's GPT models.
type Model struct {
	client    *openai.Client
	modelName string
	logger    *slog.Logger
}

// New creates a new OpenAI model instance.
func New(apiKey, modelName string) (*Model, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if modelName == "" {
		return nil, fmt.Errorf("model name is required")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	return &Model{
		client:    &client,
		modelName: modelName,
		logger:    slog.Default(),
	}, nil
}

// Name returns the model name.
func (o *Model) Name() string {
	return o.modelName
}

// GenerateContent generates content using the OpenAI model.
// This implementation only supports non-streaming mode.
func (o *Model) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if stream {
			yield(nil, fmt.Errorf("streaming not supported"))
			return
		}

		response, err := o.generateContentNonStreaming(ctx, req)
		yield(response, err)
	}
}

// generateContentNonStreaming performs a non-streaming content generation request.
//
//nolint:gocyclo,revive // API integration requires handling many response conditions
func (o *Model) generateContentNonStreaming(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	// Transform ADK request to OpenAI format
	messages, err := transformADKToOpenAI(req.Contents)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Extract system instruction from Config.SystemInstruction
	// This is where ADK places the llmagent's Instruction field
	if req.Config != nil && req.Config.SystemInstruction != nil {
		var systemText string
		for _, part := range req.Config.SystemInstruction.Parts {
			if part != nil && part.Text != "" {
				if systemText != "" {
					systemText += "\n\n"
				}
				systemText += part.Text
			}
		}
		if systemText != "" {
			// Prepend system message to the messages array
			systemMsg := openai.SystemMessage(systemText)
			messages = append([]openai.ChatCompletionMessageParamUnion{systemMsg}, messages...)
		}
	}

	// Determine max tokens - default to 4096 if not specified
	var maxTokens int64 = 4096
	if req.Config != nil && req.Config.MaxOutputTokens > 0 {
		maxTokens = int64(req.Config.MaxOutputTokens)
	}

	// Build the chat completion params
	params := openai.ChatCompletionNewParams{
		Model:     o.modelName,
		MaxTokens: openai.Int(maxTokens),
		Messages:  messages,
	}

	// Add temperature if specified
	if req.Config != nil && req.Config.Temperature != nil {
		params.Temperature = openai.Float(float64(*req.Config.Temperature))
	}

	// Add top_p if specified
	if req.Config != nil && req.Config.TopP != nil {
		params.TopP = openai.Float(float64(*req.Config.TopP))
	}

	// Add stop sequences if specified
	if req.Config != nil && len(req.Config.StopSequences) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: req.Config.StopSequences,
		}
	}

	// Transform and add tools if present
	if req.Tools != nil {
		tools := transformToolsToOpenAI(req.Tools)
		if len(tools) > 0 {
			params.Tools = tools
		}
	}

	// Make the API call
	completion, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai API error: %w", err)
	}

	// Transform the response
	response, err := transformOpenAIToADK(completion)
	if err != nil {
		return nil, fmt.Errorf("failed to transform response: %w", err)
	}

	return response, nil
}
