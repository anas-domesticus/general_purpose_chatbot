package config

// GeminiConfig holds Google Gemini-specific configuration
type GeminiConfig struct {
	APIKey  string `env:"GEMINI_API_KEY" yaml:"-"`
	Model   string `env:"GEMINI_MODEL" yaml:"model" default:"gemini-2.5-flash"`
	Project string `env:"GOOGLE_CLOUD_PROJECT" yaml:"project"` // Optional: for Vertex AI
	Region  string `env:"GOOGLE_CLOUD_REGION" yaml:"region"`   // Optional: for Vertex AI
}
