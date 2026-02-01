package session

import (
	"fmt"
	"google.golang.org/adk/session"
)

// SessionServiceBuilder helps create JSON session services with different storage backends
type SessionServiceBuilder struct {
	fileProvider FileProvider
}

// NewSessionServiceBuilder creates a new builder
func NewSessionServiceBuilder() *SessionServiceBuilder {
	return &SessionServiceBuilder{}
}

// WithLocalFileStorage configures the builder to use local file storage
func (b *SessionServiceBuilder) WithLocalFileStorage(baseDir string) *SessionServiceBuilder {
	b.fileProvider = NewLocalFileProvider(baseDir)
	return b
}

// WithS3Storage configures the builder to use S3 storage
func (b *SessionServiceBuilder) WithS3Storage(bucket, prefix string, s3Client S3Client) *SessionServiceBuilder {
	b.fileProvider = NewS3FileProvider(bucket, prefix, s3Client)
	return b
}

// WithCustomFileProvider allows using a custom file provider implementation
func (b *SessionServiceBuilder) WithCustomFileProvider(provider FileProvider) *SessionServiceBuilder {
	b.fileProvider = provider
	return b
}

// Build creates the session service
func (b *SessionServiceBuilder) Build() (session.Service, error) {
	if b.fileProvider == nil {
		return nil, fmt.Errorf("file provider must be configured")
	}

	return NewJSONSessionService(b.fileProvider), nil
}

// Convenience functions for common configurations

// NewLocalJSONSessionService creates a JSON session service with local file storage
func NewLocalJSONSessionService(baseDir string) session.Service {
	builder := NewSessionServiceBuilder()
	service, err := builder.WithLocalFileStorage(baseDir).Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to create local JSON session service: %v", err))
	}
	return service
}

// NewS3JSONSessionService creates a JSON session service with S3 storage
func NewS3JSONSessionService(bucket, prefix string, s3Client S3Client) session.Service {
	builder := NewSessionServiceBuilder()
	service, err := builder.WithS3Storage(bucket, prefix, s3Client).Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to create S3 JSON session service: %v", err))
	}
	return service
}
