package storage_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitFileProvider implements FileProvider backed by a git repository.
// Each write/delete operation creates a commit automatically.
type GitFileProvider struct {
	repoPath    string
	repo        *git.Repository
	authorName  string
	authorEmail string
	mu          sync.Mutex
}

// GitProviderOptions holds options for creating a GitFileProvider.
type GitProviderOptions struct {
	// Path is the path to the git repository.
	Path string
	// AuthorName is the name used for commits.
	AuthorName string
	// AuthorEmail is the email used for commits.
	AuthorEmail string
	// InitIfMissing initializes a new repo if the path doesn't contain one.
	InitIfMissing bool
}

// NewGitFileProvider creates a new git-backed file provider.
func NewGitFileProvider(opts GitProviderOptions) (*GitFileProvider, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	// Set defaults for author info
	authorName := opts.AuthorName
	if authorName == "" {
		authorName = "GitFileProvider"
	}
	authorEmail := opts.AuthorEmail
	if authorEmail == "" {
		authorEmail = "gitfileprovider@localhost"
	}

	var repo *git.Repository
	var err error

	// Try to open existing repository
	repo, err = git.PlainOpen(opts.Path)
	if err != nil {
		if err == git.ErrRepositoryNotExists && opts.InitIfMissing {
			// Create directory if needed
			if err := os.MkdirAll(opts.Path, 0o750); err != nil {
				return nil, fmt.Errorf("failed to create repository directory: %w", err)
			}
			// Initialize new repository
			repo, err = git.PlainInit(opts.Path, false)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize git repository: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to open git repository: %w", err)
		}
	}

	return &GitFileProvider{
		repoPath:    opts.Path,
		repo:        repo,
		authorName:  authorName,
		authorEmail: authorEmail,
	}, nil
}

// Read reads a file from the git working tree.
func (p *GitFileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(p.repoPath, path)
	return os.ReadFile(fullPath) //nolint:gosec // G304: Path is constructed from trusted repoPath
}

// Write writes data to a file and commits the change.
func (p *GitFileProvider) Write(ctx context.Context, path string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	fullPath := filepath.Join(p.repoPath, path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the file
	if err := os.WriteFile(fullPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Stage the file
	worktree, err := p.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if _, err := worktree.Add(path); err != nil {
		return fmt.Errorf("failed to stage file: %w", err)
	}

	// Commit the change
	commitMsg := fmt.Sprintf("[auto] Write %s", path)
	_, err = worktree.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  p.authorName,
			Email: p.authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// Exists checks if a file exists in the working tree.
func (p *GitFileProvider) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(p.repoPath, path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Delete removes a file and commits the deletion.
func (p *GitFileProvider) Delete(ctx context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	fullPath := filepath.Join(p.repoPath, path)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil // File doesn't exist, consider it deleted
	}

	// Remove the file from filesystem
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// Stage the deletion
	worktree, err := p.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if _, err := worktree.Remove(path); err != nil {
		// If the file wasn't tracked, that's fine
		if !strings.Contains(err.Error(), "file does not exist") {
			return fmt.Errorf("failed to stage deletion: %w", err)
		}
	}

	// Commit the deletion
	commitMsg := fmt.Sprintf("[auto] Delete %s", path)
	_, err = worktree.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  p.authorName,
			Email: p.authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		// If there's nothing to commit (file wasn't tracked), that's ok
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("failed to commit deletion: %w", err)
	}

	return nil
}

// List returns files matching a prefix in the working tree.
func (p *GitFileProvider) List(ctx context.Context, prefix string) ([]string, error) {
	searchPath := filepath.Join(p.repoPath, prefix)

	var result []string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// Skip the .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			rel, err := filepath.Rel(p.repoPath, path)
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
