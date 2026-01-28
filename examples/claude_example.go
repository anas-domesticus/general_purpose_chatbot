package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Create Claude model instance
	claudeModel, err := anthropic.NewClaudeModel(apiKey, "claude-3-5-sonnet-20241022")
	if err != nil {
		log.Fatalf("Failed to create Claude model: %v", err)
	}

	fmt.Printf("Created Claude model: %s\n", claudeModel.Name())

	// Create a sample request
	request := &model.LLMRequest{
		Model: claudeModel.Name(),
		Contents: []*genai.Content{
			{
				Role: "system",
				Parts: []*genai.Part{
					{Text: "You are a helpful assistant that provides concise, accurate answers."},
				},
			},
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "What are the three main benefits of using Go for backend development?"},
				},
			},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: 200,
			Temperature:     floatPtr(0.7),
		},
	}

	ctx := context.Background()

	// Generate response
	fmt.Println("\n--- Claude Response ---")
	iter := claudeModel.GenerateContent(ctx, request, false)
	
	for response, err := range iter {
		if err != nil {
			log.Fatalf("Error generating content: %v", err)
		}
		
		if response.Content != nil && len(response.Content.Parts) > 0 {
			for _, part := range response.Content.Parts {
				if part.Text != "" {
					fmt.Println(part.Text)
				}
			}
		}
		
		// Print usage metadata if available
		if response.UsageMetadata != nil {
			fmt.Printf("\n--- Token Usage ---\n")
			fmt.Printf("Input tokens: %d\n", response.UsageMetadata.PromptTokenCount)
			fmt.Printf("Output tokens: %d\n", response.UsageMetadata.CandidatesTokenCount)
			fmt.Printf("Total tokens: %d\n", response.UsageMetadata.TotalTokenCount)
		}
	}
}

func floatPtr(f float32) *float32 {
	return &f
}