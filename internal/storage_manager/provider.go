// Package storage_manager provides unified storage abstraction for application persistence.
// It supports local filesystem and S3 backends, allowing different components
// (sessions, config, etc.) to get prefix-scoped file providers for isolated storage.
package storage_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileProvider defines the interface for file storage operations.
// Implementations can support local filesystem, S3, or other storage backends.
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

// LocalFileProvider implements FileProvider for local filesystem.
type LocalFileProvider struct {
	baseDir string
}

// NewLocalFileProvider creates a new local file provider.
func NewLocalFileProvider(baseDir string) *LocalFileProvider {
	return &LocalFileProvider{
		baseDir: baseDir,
	}
}

// Read reads a file from the local filesystem.
func (p *LocalFileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(p.baseDir, path)) //nolint:gosec // G304: Path is constructed from trusted baseDir
}

// Write writes data to a local file.
func (p *LocalFileProvider) Write(ctx context.Context, path string, data []byte) error {
	fullPath := filepath.Join(p.baseDir, path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return os.WriteFile(fullPath, data, 0o600)
}

// Exists checks if a file exists on the local filesystem.
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

// Delete removes a file from the local filesystem.
func (p *LocalFileProvider) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(p.baseDir, path)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, consider it deleted
	}
	return err
}

// List returns files matching a prefix in the local filesystem.
func (p *LocalFileProvider) List(ctx context.Context, prefix string) ([]string, error) {
	searchPath := filepath.Join(p.baseDir, prefix)

	var result []string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if !info.IsDir() {
			rel, err := filepath.Rel(p.baseDir, path)
			if err == nil {
				result = append(result, rel)
			}
		}

		return nil
	})

	if err != nil && os.IsNotExist(err) {
		return []string{}, nil
	}

	return result, err
}

// S3FileProvider implements FileProvider for AWS S3.
type S3FileProvider struct {
	bucket   string
	prefix   string
	s3Client S3Client
}

// NewS3FileProvider creates a new S3 file provider.
func NewS3FileProvider(bucket, prefix string, s3Client S3Client) *S3FileProvider {
	return &S3FileProvider{
		bucket:   bucket,
		prefix:   prefix,
		s3Client: s3Client,
	}
}

// Read reads a file from S3.
func (p *S3FileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	key := p.getKey(path)
	return p.s3Client.GetObject(ctx, p.bucket, key)
}

// Write writes data to S3.
func (p *S3FileProvider) Write(ctx context.Context, path string, data []byte) error {
	key := p.getKey(path)
	return p.s3Client.PutObject(ctx, p.bucket, key, data)
}

// Exists checks if a file exists in S3.
// Returns (false, nil) only for "not found" errors.
// Returns (false, error) for real errors (network, permissions, etc.).
func (p *S3FileProvider) Exists(ctx context.Context, path string) (bool, error) {
	key := p.getKey(path)
	err := p.s3Client.HeadObject(ctx, p.bucket, key)
	if err != nil {
		// Check if it's a "not found" error - this is expected for empty buckets
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		// Real error - propagate it
		return false, err
	}
	return true, nil
}

// Delete removes a file from S3.
func (p *S3FileProvider) Delete(ctx context.Context, path string) error {
	key := p.getKey(path)
	return p.s3Client.DeleteObject(ctx, p.bucket, key)
}

// List returns files matching a prefix in S3.
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

// getKey constructs the full S3 key by combining prefix and path.
func (p *S3FileProvider) getKey(path string) string {
	if p.prefix == "" {
		return path
	}
	return p.prefix + "/" + path
}

// PrefixedFileProvider wraps a FileProvider to add a prefix to all paths.
// This allows multiple components to share the same underlying storage
// while maintaining isolated namespaces.
type PrefixedFileProvider struct {
	provider FileProvider
	prefix   string
}

// NewPrefixedFileProvider creates a new prefixed file provider.
func NewPrefixedFileProvider(provider FileProvider, prefix string) *PrefixedFileProvider {
	return &PrefixedFileProvider{
		provider: provider,
		prefix:   prefix,
	}
}

// Read reads a file with the prefix applied.
func (p *PrefixedFileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	return p.provider.Read(ctx, p.prefixPath(path))
}

// Write writes data with the prefix applied.
func (p *PrefixedFileProvider) Write(ctx context.Context, path string, data []byte) error {
	return p.provider.Write(ctx, p.prefixPath(path), data)
}

// Exists checks if a file exists with the prefix applied.
func (p *PrefixedFileProvider) Exists(ctx context.Context, path string) (bool, error) {
	return p.provider.Exists(ctx, p.prefixPath(path))
}

// Delete removes a file with the prefix applied.
func (p *PrefixedFileProvider) Delete(ctx context.Context, path string) error {
	return p.provider.Delete(ctx, p.prefixPath(path))
}

// List returns files matching a prefix, with the provider prefix applied.
func (p *PrefixedFileProvider) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := p.prefixPath(prefix)
	files, err := p.provider.List(ctx, fullPrefix)
	if err != nil {
		return nil, err
	}

	// Remove the provider prefix from results
	var result []string
	prefixLen := len(p.prefixPath(""))
	for _, file := range files {
		if len(file) >= prefixLen {
			result = append(result, file[prefixLen:])
		}
	}

	return result, nil
}

// prefixPath combines the prefix with the given path.
func (p *PrefixedFileProvider) prefixPath(path string) string {
	if p.prefix == "" {
		return path
	}
	return p.prefix + "/" + path
}
