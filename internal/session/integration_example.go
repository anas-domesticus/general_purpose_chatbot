package session

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/adk/session"
)

// CreateSessionServiceFromConfig creates a session service based on configuration
// This is an example of how you might integrate this into your main application
func CreateSessionServiceFromConfig(cfg *SessionConfig) (session.Service, error) {
	switch cfg.Backend {
	case "local":
		return createLocalSessionService(cfg.LocalConfig)
	case "s3":
		return createS3SessionService(cfg.S3Config)
	default:
		return session.InMemoryService(), nil // fallback to ADK default
	}
}

// SessionConfig represents the configuration for session storage
type SessionConfig struct {
	Backend     string       `yaml:"backend" json:"backend"`         // "local", "s3", or "memory"
	LocalConfig LocalConfig  `yaml:"local" json:"local"`
	S3Config    S3Config     `yaml:"s3" json:"s3"`
}

// LocalConfig represents configuration for local file storage
type LocalConfig struct {
	BaseDir string `yaml:"base_dir" json:"base_dir"`
}

// S3Config represents configuration for S3 storage
type S3Config struct {
	Bucket    string `yaml:"bucket" json:"bucket"`
	Prefix    string `yaml:"prefix" json:"prefix"`
	Region    string `yaml:"region" json:"region"`
	Profile   string `yaml:"profile" json:"profile"`
}

// createLocalSessionService creates a local file-based session service
func createLocalSessionService(cfg LocalConfig) (session.Service, error) {
	if cfg.BaseDir == "" {
		return nil, fmt.Errorf("local session service requires base_dir to be set")
	}

	// Ensure directory exists
	if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory %s: %w", cfg.BaseDir, err)
	}

	return NewLocalJSONSessionService(cfg.BaseDir), nil
}

// createS3SessionService creates an S3-based session service
func createS3SessionService(cfg S3Config) (session.Service, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 session service requires bucket to be set")
	}

	// Load AWS configuration
	var awsCfg aws.Config
	var err error

	if cfg.Profile != "" {
		// Load config with profile
		awsCfg, err = config.LoadDefaultConfig(
			context.Background(),
			config.WithSharedConfigProfile(cfg.Profile),
		)
	} else {
		// Load default config
		awsCfg, err = config.LoadDefaultConfig(context.Background())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override region if specified
	if cfg.Region != "" {
		awsCfg.Region = cfg.Region
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)
	awsS3Client := NewAWSS3Client(s3Client)

	return NewS3JSONSessionService(cfg.Bucket, cfg.Prefix, awsS3Client), nil
}

// Example usage in main.go:
/*

func main() {
	// ... existing code ...

	// Load session configuration from your config system
	sessionConfig := &SessionConfig{
		Backend: os.Getenv("SESSION_BACKEND"), // "local" or "s3"
		LocalConfig: LocalConfig{
			BaseDir: os.Getenv("SESSION_LOCAL_DIR"),
		},
		S3Config: S3Config{
			Bucket: os.Getenv("SESSION_S3_BUCKET"),
			Prefix: os.Getenv("SESSION_S3_PREFIX"),
			Region: os.Getenv("SESSION_S3_REGION"),
			Profile: os.Getenv("AWS_PROFILE"),
		},
	}

	// Create session service based on configuration
	sessionService, err := session.CreateSessionServiceFromConfig(sessionConfig)
	if err != nil {
		log.Fatalf("Failed to create session service: %v", err)
	}

	// Use the session service with your agent loader
	agentLoader := agents.NewLoader(claudeModel, cfg.MCP, sessionService)

	// ... rest of existing code ...
}

Environment Variables for configuration:
- SESSION_BACKEND=local|s3|memory
- SESSION_LOCAL_DIR=/path/to/sessions
- SESSION_S3_BUCKET=my-session-bucket
- SESSION_S3_PREFIX=sessions
- SESSION_S3_REGION=us-east-1
- AWS_PROFILE=my-profile

*/