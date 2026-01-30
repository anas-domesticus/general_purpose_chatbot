package anthropic

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"google.golang.org/adk/model"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// ClaudeModel implements the model.LLM interface for Anthropic Claude models
type ClaudeModel struct {
	client         anthropic.Client
	modelName      string
	logger         *slog.Logger
	structLogger   logger.Logger
	retryConfig    RetryConfig
	circuitBreaker *CircuitBreaker
	enableThinking bool
	thinkingBudget int64
}

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		BackoffFactor:  2.0,
	}
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	state            CircuitState
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		state:            CircuitClosed,
	}
}

// NewClaudeModel creates a new Claude model instance with enhanced error handling
func NewClaudeModel(apiKey, modelName string, opts ...option.RequestOption) (*ClaudeModel, error) {
	return NewClaudeModelWithConfig(apiKey, modelName, ClaudeConfig{}, opts...)
}

// ClaudeConfig holds configuration options for Claude model
type ClaudeConfig struct {
	Logger         logger.Logger
	RetryConfig    *RetryConfig
	CircuitBreaker *CircuitBreaker
	EnableThinking bool  // Enable extended thinking/reasoning
	ThinkingBudget int64 // Token budget for thinking (0 = model default)
}

// NewClaudeModelWithConfig creates a new Claude model instance with full configuration
func NewClaudeModelWithConfig(apiKey, modelName string, config ClaudeConfig, opts ...option.RequestOption) (*ClaudeModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}
	if modelName == "" {
		modelName = string(anthropic.ModelClaudeSonnet4_5_20250929) // Default to latest Sonnet 4.5
	}

	client := anthropic.NewClient(
		append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)...,
	)

	slogLogger := slog.With("component", "claude_model", "model", modelName)

	// Set up retry configuration
	retryConfig := DefaultRetryConfig()
	if config.RetryConfig != nil {
		retryConfig = *config.RetryConfig
	}

	// Set up circuit breaker
	circuitBreaker := config.CircuitBreaker
	if circuitBreaker == nil {
		circuitBreaker = NewCircuitBreaker(5, 30*time.Second) // 5 failures, 30s reset
	}

	model := &ClaudeModel{
		client:         client,
		modelName:      modelName,
		logger:         slogLogger,
		structLogger:   config.Logger,
		retryConfig:    retryConfig,
		circuitBreaker: circuitBreaker,
		enableThinking: config.EnableThinking,
		thinkingBudget: config.ThinkingBudget,
	}

	// Log initialization
	if model.structLogger != nil {
		model.structLogger.Info("Claude model initialized",
			logger.StringField("model", modelName),
			logger.IntField("max_retries", retryConfig.MaxRetries),
			logger.DurationField("initial_backoff", retryConfig.InitialBackoff))
	}

	return model, nil
}

// Name returns the name of the model
func (c *ClaudeModel) Name() string {
	return c.modelName
}

// GenerateContent implements the model.LLM interface
func (c *ClaudeModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	c.logger.Info("generating content", "stream", stream, "contents_count", len(req.Contents))

	// For now,
	//implement non-streaming only to get basic functionality working
	// We'll add streaming support later
	return c.generateContentNonStream(ctx, req)
}

// generateContentNonStream handles non-streaming generation with retry logic and circuit breaker
func (c *ClaudeModel) generateContentNonStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Check circuit breaker
		if err := c.checkCircuitBreaker(); err != nil {
			c.logError("circuit breaker open", err)
			yield(nil, err)
			return
		}

		// Transform ADK request to Anthropic format
		messages, systemPrompt, err := transformADKToAnthropic(req.Contents)
		if err != nil {
			c.logError("failed to transform request", err, logger.StringField("transform_type", "adk_to_anthropic"))
			yield(nil, fmt.Errorf("failed to transform request: %w", err))
			return
		}

		// Extract SystemInstruction from Config (ADK's way of passing agent instructions)
		if req.Config != nil && req.Config.SystemInstruction != nil {
			for _, part := range req.Config.SystemInstruction.Parts {
				if part.Text != "" {
					if systemPrompt != "" {
						systemPrompt += "\n\n"
					}
					systemPrompt += part.Text
				}
			}
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
				c.logError("failed to transform tools", err, logger.StringField("transform_type", "tools_to_anthropic"))
				yield(nil, fmt.Errorf("failed to transform tools: %w", err))
				return
			}
			anthropicReq.Tools = tools
		}

		// Add extended thinking configuration if enabled
		if c.enableThinking {
			budget := c.thinkingBudget
			if budget == 0 {
				budget = 10000 // Default thinking budget
			}
			anthropicReq.Thinking = anthropic.ThinkingConfigParamOfEnabled(budget)
		}

		// Execute request with retry logic
		resp, err := c.executeWithRetry(ctx, anthropicReq)
		if err != nil {
			c.recordFailure()
			c.logError("claude api request failed after retries", err,
				logger.IntField("max_retries", c.retryConfig.MaxRetries))
			yield(nil, fmt.Errorf("claude api error after retries: %w", err))
			return
		}

		// Record success
		c.recordSuccess()

		// Transform response back to ADK format
		llmResponse, err := transformAnthropicToADK(resp)
		if err != nil {
			c.logError("failed to transform response", err, logger.StringField("transform_type", "anthropic_to_adk"))
			yield(nil, fmt.Errorf("failed to transform response: %w", err))
			return
		}

		c.logSuccess("generation completed successfully",
			logger.IntField("content_blocks", len(resp.Content)),
			logger.IntField("input_tokens", int(resp.Usage.InputTokens)),
			logger.IntField("output_tokens", int(resp.Usage.OutputTokens)))

		// Yield the response
		yield(llmResponse, nil)
	}
}

// executeWithRetry executes the Anthropic API call with exponential backoff retry
func (c *ClaudeModel) executeWithRetry(ctx context.Context, req anthropic.MessageNewParams) (*anthropic.Message, error) {
	var lastErr error
	backoff := c.retryConfig.InitialBackoff

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		c.logger.Debug("attempting anthropic api call",
			"attempt", attempt+1,
			"max_attempts", c.retryConfig.MaxRetries+1)

		// Make the API call
		resp, err := c.client.Messages.New(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if we should retry based on error type
		if !c.shouldRetry(err) {
			c.logError("non-retryable error encountered", err,
				logger.IntField("attempt", attempt+1))
			return nil, err
		}

		// Don't wait after the last attempt
		if attempt == c.retryConfig.MaxRetries {
			break
		}

		c.logWarn("anthropic api call failed, retrying",
			logger.ErrorField(err),
			logger.IntField("attempt", attempt+1),
			logger.DurationField("backoff", backoff))

		// Wait with exponential backoff
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		// Calculate next backoff duration
		backoff = time.Duration(float64(backoff) * c.retryConfig.BackoffFactor)
		if backoff > c.retryConfig.MaxBackoff {
			backoff = c.retryConfig.MaxBackoff
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.retryConfig.MaxRetries, lastErr)
}

// shouldRetry determines if an error is retryable
func (c *ClaudeModel) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Retry on network/temporary errors
	if strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "temporary") ||
		strings.Contains(errMsg, "rate limit") {
		return true
	}

	// Check for specific HTTP status codes that should be retried
	if strings.Contains(errMsg, "500") || // Internal Server Error
		strings.Contains(errMsg, "502") || // Bad Gateway
		strings.Contains(errMsg, "503") || // Service Unavailable
		strings.Contains(errMsg, "504") || // Gateway Timeout
		strings.Contains(errMsg, "429") { // Too Many Requests
		return true
	}

	return false
}

// Circuit breaker methods
func (c *CircuitBreaker) CanExecute() bool {
	now := time.Now()

	switch c.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if now.Sub(c.lastFailureTime) >= c.resetTimeout {
			c.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

func (c *CircuitBreaker) RecordSuccess() {
	c.failureCount = 0
	c.state = CircuitClosed
}

func (c *CircuitBreaker) RecordFailure() {
	c.failureCount++
	c.lastFailureTime = time.Now()

	if c.failureCount >= c.failureThreshold {
		c.state = CircuitOpen
	}
}

// checkCircuitBreaker checks if the circuit breaker allows execution
func (c *ClaudeModel) checkCircuitBreaker() error {
	if !c.circuitBreaker.CanExecute() {
		return errors.New("circuit breaker is open - too many recent failures")
	}
	return nil
}

// recordSuccess records a successful operation
func (c *ClaudeModel) recordSuccess() {
	c.circuitBreaker.RecordSuccess()
}

// recordFailure records a failed operation
func (c *ClaudeModel) recordFailure() {
	c.circuitBreaker.RecordFailure()
}

// Logging helper methods
func (c *ClaudeModel) logError(msg string, err error, fields ...logger.LogField) {
	if c.structLogger != nil {
		allFields := append([]logger.LogField{logger.ErrorField(err)}, fields...)
		c.structLogger.Error(msg, allFields...)
	}
	c.logger.Error(msg, "error", err)
}

func (c *ClaudeModel) logWarn(msg string, fields ...logger.LogField) {
	if c.structLogger != nil {
		c.structLogger.Warn(msg, fields...)
	}
	c.logger.Warn(msg)
}

func (c *ClaudeModel) logSuccess(msg string, fields ...logger.LogField) {
	if c.structLogger != nil {
		c.structLogger.Info(msg, fields...)
	}
	c.logger.Info(msg)
}
