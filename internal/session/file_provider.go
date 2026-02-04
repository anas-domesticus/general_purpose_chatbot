// Package session provides session storage and management.
package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FileProvider defines the interface for file storage operations
// Implementations can support local filesystem, S3, or other storage backends
type FileProvider interface {
	// Read reads the entire content of a file
	Read(ctx context.Context, path string) ([]byte, error)

	// Write writes data to a file, creating it if it doesn't exist
	Write(ctx context.Context, path string, data []byte) error

	// Exists checks if a file exists
	Exists(ctx context.Context, path string) (bool, error)

	// Delete removes a file
	Delete(ctx context.Context, path string) error

	// List returns a list of files matching a prefix
	List(ctx context.Context, prefix string) ([]string, error)
}

// LocalFileProvider implements FileProvider for local filesystem
type LocalFileProvider struct {
	baseDir string
}

// NewLocalFileProvider creates a new local file provider
func NewLocalFileProvider(baseDir string) *LocalFileProvider {
	return &LocalFileProvider{
		baseDir: baseDir,
	}
}

// Read reads a file from the local filesystem
func (p *LocalFileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(p.baseDir, path))
}

// Write writes data to a local file
func (p *LocalFileProvider) Write(ctx context.Context, path string, data []byte) error {
	fullPath := filepath.Join(p.baseDir, path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return os.WriteFile(fullPath, data, 0o644)
}

// Exists checks if a file exists on the local filesystem
func (p *LocalFileProvider) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(p.baseDir, path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Delete removes a file from the local filesystem
func (p *LocalFileProvider) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(p.baseDir, path)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, consider it deleted
	}
	return err
}

// List returns files matching a prefix in the local filesystem
func (p *LocalFileProvider) List(ctx context.Context, prefix string) ([]string, error) {
	searchPath := filepath.Join(p.baseDir, prefix)

	// Use filepath.Walk to find all JSON files under the prefix path
	var result []string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If the search path doesn't exist, return empty list (not an error)
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// Only include JSON files
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			rel, err := filepath.Rel(p.baseDir, path)
			if err == nil {
				result = append(result, rel)
			}
		}

		return nil
	})

	// If the directory doesn't exist, return empty list (not an error)
	if err != nil && os.IsNotExist(err) {
		return []string{}, nil
	}

	return result, err
}

// S3FileProvider implements FileProvider for AWS S3
type S3FileProvider struct {
	bucket   string
	prefix   string
	s3Client S3Client // Interface to allow for mocking
}

// S3Client defines the S3 operations we need
type S3Client interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte) error
	HeadObject(ctx context.Context, bucket, key string) error
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}

// NewS3FileProvider creates a new S3 file provider
func NewS3FileProvider(bucket, prefix string, s3Client S3Client) *S3FileProvider {
	return &S3FileProvider{
		bucket:   bucket,
		prefix:   prefix,
		s3Client: s3Client,
	}
}

// Read reads a file from S3
func (p *S3FileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	key := p.getKey(path)
	return p.s3Client.GetObject(ctx, p.bucket, key)
}

// Write writes data to S3
func (p *S3FileProvider) Write(ctx context.Context, path string, data []byte) error {
	key := p.getKey(path)
	return p.s3Client.PutObject(ctx, p.bucket, key, data)
}

// Exists checks if a file exists in S3
func (p *S3FileProvider) Exists(ctx context.Context, path string) (bool, error) {
	key := p.getKey(path)
	err := p.s3Client.HeadObject(ctx, p.bucket, key)
	if err != nil {
		// Check if it's a "not found" error
		// This would depend on the S3 client implementation
		return false, nil
	}
	return true, nil
}

// Delete removes a file from S3
func (p *S3FileProvider) Delete(ctx context.Context, path string) error {
	key := p.getKey(path)
	return p.s3Client.DeleteObject(ctx, p.bucket, key)
}

// List returns files matching a prefix in S3
func (p *S3FileProvider) List(ctx context.Context, prefix string) ([]string, error) {
	s3Prefix := p.getKey(prefix)
	keys, err := p.s3Client.ListObjects(ctx, p.bucket, s3Prefix)
	if err != nil {
		return nil, err
	}

	// Remove the S3 prefix to get relative paths
	var result []string
	prefixLen := len(p.getKey(""))
	for _, key := range keys {
		if len(key) > prefixLen {
			result = append(result, key[prefixLen:])
		}
	}

	return result, nil
}

// getKey constructs the full S3 key by combining prefix and path
func (p *S3FileProvider) getKey(path string) string {
	if p.prefix == "" {
		return path
	}
	return p.prefix + "/" + path
}
