package config

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" yaml:"cors_allowed_origins" default:"http://localhost:3000,http://localhost:8080"`
	MaxRequestSize     int64    `env:"MAX_REQUEST_SIZE" yaml:"max_request_size" default:"10485760"` // 10MB default
	RateLimitEnabled   bool     `env:"RATE_LIMIT_ENABLED" yaml:"rate_limit_enabled" default:"true"`
	RateLimitRPS       int      `env:"RATE_LIMIT_RPS" yaml:"rate_limit_rps" default:"100"`
}
