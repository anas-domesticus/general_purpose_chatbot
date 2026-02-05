package storage_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BackendType represents the type of storage backend.
type BackendType string

const (
	// BackendLocal uses the local filesystem for storage.
	BackendLocal BackendType = "local"
	// BackendS3 uses AWS S3 for storage.
	BackendS3 BackendType = "s3"
	// BackendGit uses a git repository for storage.
	BackendGit BackendType = "git"
)

// Config holds the configuration for the StorageManager.
type Config struct {
	// Backend specifies the storage backend type (local, s3, or git).
	Backend BackendType

	// LocalConfig holds configuration for local filesystem storage.
	LocalConfig *LocalConfig

	// S3Config holds configuration for S3 storage.
	S3Config *S3Config

	// GitConfig holds configuration for git repository storage.
	GitConfig *GitConfig
}

// LocalConfig holds configuration for local filesystem storage.
type LocalConfig struct {
	// BaseDir is the root directory for all storage.
	BaseDir string
}

// S3Config holds configuration for S3 storage.
type S3Config struct {
	// Bucket is the S3 bucket name.
	Bucket string
	// Prefix is an optional prefix for all keys in the bucket.
	Prefix string
	// Client is the AWS S3 client. If nil, a default client will be created.
	Client *s3.Client
}

// GitConfig holds configuration for git repository storage.
type GitConfig struct {
	// Path is the path to the git repository.
	Path string
	// AuthorName is the name used for commits (defaults to "GitFileProvider").
	AuthorName string
	// AuthorEmail is the email used for commits (defaults to "gitfileprovider@localhost").
	AuthorEmail string
	// InitIfMissing initializes a new repo if the path doesn't contain one.
	InitIfMissing bool
}

// StorageManager provides unified storage management for the application.
// It creates prefix-scoped file providers for different components like
// sessions, config, and other persistent data.
type StorageManager struct {
	config   Config
	provider FileProvider
}

// New creates a new StorageManager with the given configuration.
func New(config Config) (*StorageManager, error) {
	var provider FileProvider

	switch config.Backend {
	case BackendLocal:
		if config.LocalConfig == nil {
			return nil, fmt.Errorf("local config is required for local backend")
		}
		if config.LocalConfig.BaseDir == "" {
			return nil, fmt.Errorf("base directory is required for local backend")
		}
		provider = NewLocalFileProvider(config.LocalConfig.BaseDir)

	case BackendS3:
		if config.S3Config == nil {
			return nil, fmt.Errorf("s3 config is required for s3 backend")
		}
		if config.S3Config.Bucket == "" {
			return nil, fmt.Errorf("bucket is required for s3 backend")
		}
		if config.S3Config.Client == nil {
			return nil, fmt.Errorf("s3 client is required for s3 backend")
		}
		s3Client := NewAWSS3Client(config.S3Config.Client)
		provider = NewS3FileProvider(config.S3Config.Bucket, config.S3Config.Prefix, s3Client)

	case BackendGit:
		if config.GitConfig == nil {
			return nil, fmt.Errorf("git config is required for git backend")
		}
		if config.GitConfig.Path == "" {
			return nil, fmt.Errorf("path is required for git backend")
		}
		var err error
		provider, err = NewGitFileProvider(GitProviderOptions{
			Path:          config.GitConfig.Path,
			AuthorName:    config.GitConfig.AuthorName,
			AuthorEmail:   config.GitConfig.AuthorEmail,
			InitIfMissing: config.GitConfig.InitIfMissing,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create git provider: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported backend type: %s", config.Backend)
	}

	return &StorageManager{
		config:   config,
		provider: provider,
	}, nil
}

// NewWithProvider creates a new StorageManager with a custom FileProvider.
// This is useful for testing or when using a custom storage implementation.
func NewWithProvider(provider FileProvider) *StorageManager {
	return &StorageManager{
		provider: provider,
	}
}

// GetProvider returns a prefix-scoped FileProvider for the given namespace.
// Each namespace gets its own isolated storage area within the backend.
//
// Example namespaces:
//   - "sessions" for session data
//   - "config" for application configuration
//   - "cache" for cached data
func (m *StorageManager) GetProvider(namespace string) FileProvider {
	if namespace == "" {
		return m.provider
	}
	return NewPrefixedFileProvider(m.provider, namespace)
}

// GetRootProvider returns the root FileProvider without any prefix.
// Use this with caution as it provides access to all storage.
func (m *StorageManager) GetRootProvider() FileProvider {
	return m.provider
}

// Backend returns the configured backend type.
func (m *StorageManager) Backend() BackendType {
	return m.config.Backend
}
