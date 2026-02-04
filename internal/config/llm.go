package config

// LLM provider constants
const (
	ProviderClaude = "claude"
	ProviderGemini = "gemini"
	ProviderOpenAI = "openai"
)

// LLMConfig holds LLM provider selection configuration
type LLMConfig struct {
	// Provider specifies which LLM provider to use: "claude", "gemini", or "openai"
	Provider string `env:"LLM_PROVIDER" yaml:"provider" default:"claude"`
}
