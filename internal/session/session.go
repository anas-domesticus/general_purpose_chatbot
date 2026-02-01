package session

import (
	"fmt"

	"google.golang.org/adk/session"
)

// StorageConfig holds configuration for session storage backend
type StorageConfig struct {
	Backend string      // "local", "s3", or "custom"
	Local   LocalConfig // Configuration for local file storage
	S3      S3Config    // Configuration for S3 storage
}

// LocalConfig represents configuration for local file storage
type LocalConfig struct {
	BaseDir string // Base directory for storing session files
}

// S3Config represents configuration for S3 storage
type S3Config struct {
	Bucket   string   // S3 bucket name
	Prefix   string   // Prefix for S3 object keys
	S3Client S3Client // S3 client implementation
}

// NewSessionService creates a JSON session service based on the storage configuration
func NewSessionService(config StorageConfig) (session.Service, error) {
	var fileProvider FileProvider

	switch config.Backend {
	case "local":
		if config.Local.BaseDir == "" {
			return nil, fmt.Errorf("local storage requires BaseDir to be set")
		}
		fileProvider = NewLocalFileProvider(config.Local.BaseDir)

	case "s3":
		if config.S3.Bucket == "" {
			return nil, fmt.Errorf("S3 storage requires Bucket to be set")
		}
		if config.S3.S3Client == nil {
			return nil, fmt.Errorf("S3 storage requires S3Client to be set")
		}
		fileProvider = NewS3FileProvider(config.S3.Bucket, config.S3.Prefix, config.S3.S3Client)

	case "custom":
		return nil, fmt.Errorf("custom storage backend requires using NewSessionServiceWithProvider")

	default:
		return nil, fmt.Errorf("unsupported storage backend: %s (must be 'local', 's3', or 'custom')", config.Backend)
	}

	return NewJSONSessionService(fileProvider), nil
}

// NewSessionServiceWithProvider creates a JSON session service with a custom file provider
func NewSessionServiceWithProvider(provider FileProvider) session.Service {
	if provider == nil {
		panic("file provider cannot be nil")
	}
	return NewJSONSessionService(provider)
}

// Convenience functions for common configurations

// NewLocalJSONSessionService creates a JSON session service with local file storage
func NewLocalJSONSessionService(baseDir string) session.Service {
	service, err := NewSessionService(StorageConfig{
		Backend: "local",
		Local:   LocalConfig{BaseDir: baseDir},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create local JSON session service: %v", err))
	}
	return service
}

// NewS3JSONSessionService creates a JSON session service with S3 storage
func NewS3JSONSessionService(bucket, prefix string, s3Client S3Client) session.Service {
	service, err := NewSessionService(StorageConfig{
		Backend: "s3",
		S3: S3Config{
			Bucket:   bucket,
			Prefix:   prefix,
			S3Client: s3Client,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create S3 JSON session service: %v", err))
	}
	return service
}
