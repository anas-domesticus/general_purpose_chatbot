package storage_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitFileProvider implements FileProvider backed by a git repository.
// Each write/delete operation creates a commit automatically.
// If a remote is configured, changes are pushed after a debounce period.
type GitFileProvider struct {
	repoPath    string
	repo        *git.Repository
	authorName  string
	authorEmail string
	mu          sync.Mutex

	// Remote configuration
	remoteURL string
	branch    string
	auth      transport.AuthMethod

	// Push debouncing
	pushDebounceDelay time.Duration
	pushTimer         *time.Timer
	pushMu            sync.Mutex
	pendingPush       bool

	// Lifecycle
	closed   bool
	closedMu sync.RWMutex
	wg       sync.WaitGroup
}

// GitAuthConfig holds authentication credentials for git remotes.
type GitAuthConfig struct {
	// Username for HTTPS authentication.
	Username string
	// Password or token for HTTPS authentication.
	Password string
	// SSHKeyPath is the path to an SSH private key file.
	SSHKeyPath string
	// SSHKeyPassword is the password for an encrypted SSH key (optional).
	SSHKeyPassword string
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

	// RemoteURL is the URL of the remote repository (SSH or HTTPS).
	// If set and the local repo doesn't exist, it will be cloned.
	RemoteURL string
	// Branch is the branch to use (defaults to "main").
	Branch string
	// Auth holds authentication credentials for the remote.
	Auth *GitAuthConfig
	// PushDebounceDelay is the delay before pushing after the last commit.
	// Defaults to 5 seconds.
	PushDebounceDelay time.Duration
}

// buildAuthMethod creates a transport.AuthMethod from GitAuthConfig.
func buildAuthMethod(cfg *GitAuthConfig, remoteURL string) (transport.AuthMethod, error) {
	if cfg == nil {
		return nil, nil
	}

	// Determine if SSH or HTTPS based on URL
	isSSH := strings.HasPrefix(remoteURL, "git@") || strings.HasPrefix(remoteURL, "ssh://")

	if isSSH {
		if cfg.SSHKeyPath != "" {
			auth, err := ssh.NewPublicKeysFromFile("git", cfg.SSHKeyPath, cfg.SSHKeyPassword)
			if err != nil {
				return nil, fmt.Errorf("failed to load SSH key: %w", err)
			}
			return auth, nil
		}
		// Try SSH agent as fallback
		auth, err := ssh.NewSSHAgentAuth("git")
		if err != nil {
			return nil, fmt.Errorf("failed to use SSH agent: %w", err)
		}
		return auth, nil
	}

	// HTTPS authentication
	if cfg.Username != "" && cfg.Password != "" {
		return &http.BasicAuth{
			Username: cfg.Username,
			Password: cfg.Password,
		}, nil
	}

	return nil, nil
}

// NewGitFileProvider creates a new git-backed file provider.
func NewGitFileProvider(opts GitProviderOptions) (*GitFileProvider, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	// Set defaults
	authorName := opts.AuthorName
	if authorName == "" {
		authorName = "GitFileProvider"
	}
	authorEmail := opts.AuthorEmail
	if authorEmail == "" {
		authorEmail = "gitfileprovider@localhost"
	}
	branch := opts.Branch
	if branch == "" {
		branch = "main"
	}
	debounceDelay := opts.PushDebounceDelay
	if debounceDelay == 0 {
		debounceDelay = 5 * time.Second
	}

	// Build auth method if remote is configured
	var auth transport.AuthMethod
	if opts.RemoteURL != "" {
		var err error
		auth, err = buildAuthMethod(opts.Auth, opts.RemoteURL)
		if err != nil {
			return nil, fmt.Errorf("failed to build auth method: %w", err)
		}
	}

	var repo *git.Repository
	var err error

	// Try to open existing repository
	repo, err = git.PlainOpen(opts.Path)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			if opts.RemoteURL != "" {
				// Try to clone from remote
				if err := os.MkdirAll(filepath.Dir(opts.Path), 0o750); err != nil {
					return nil, fmt.Errorf("failed to create parent directory: %w", err)
				}
				repo, err = git.PlainClone(opts.Path, false, &git.CloneOptions{
					URL:           opts.RemoteURL,
					Auth:          auth,
					ReferenceName: plumbing.NewBranchReferenceName(branch),
					SingleBranch:  true,
				})
				if err != nil {
					// If remote is empty, init locally and configure remote
					if err == transport.ErrEmptyRemoteRepository {
						if err := os.MkdirAll(opts.Path, 0o750); err != nil {
							return nil, fmt.Errorf("failed to create repository directory: %w", err)
						}
						repo, err = git.PlainInit(opts.Path, false)
						if err != nil {
							return nil, fmt.Errorf("failed to initialize git repository: %w", err)
						}
						if err := configureRemote(repo, opts.RemoteURL); err != nil {
							return nil, fmt.Errorf("failed to configure remote: %w", err)
						}
					} else {
						return nil, fmt.Errorf("failed to clone repository: %w", err)
					}
				}
			} else if opts.InitIfMissing {
				// Initialize new repository (existing behavior)
				if err := os.MkdirAll(opts.Path, 0o750); err != nil {
					return nil, fmt.Errorf("failed to create repository directory: %w", err)
				}
				repo, err = git.PlainInit(opts.Path, false)
				if err != nil {
					return nil, fmt.Errorf("failed to initialize git repository: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to open git repository: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to open git repository: %w", err)
		}
	} else if opts.RemoteURL != "" {
		// Repo exists, ensure remote is configured
		if err := configureRemote(repo, opts.RemoteURL); err != nil {
			return nil, fmt.Errorf("failed to configure remote: %w", err)
		}
	}

	return &GitFileProvider{
		repoPath:          opts.Path,
		repo:              repo,
		authorName:        authorName,
		authorEmail:       authorEmail,
		remoteURL:         opts.RemoteURL,
		branch:            branch,
		auth:              auth,
		pushDebounceDelay: debounceDelay,
	}, nil
}

// configureRemote ensures the "origin" remote is configured with the given URL.
func configureRemote(repo *git.Repository, remoteURL string) error {
	_, err := repo.Remote("origin")
	if err == git.ErrRemoteNotFound {
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteURL},
		})
		return err
	}
	return nil
}

// Read reads a file from the git working tree.
func (p *GitFileProvider) Read(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(p.repoPath, path)
	return os.ReadFile(fullPath) //nolint:gosec // G304: Path is constructed from trusted repoPath
}

// Write writes data to a file and commits the change.
func (p *GitFileProvider) Write(ctx context.Context, path string, data []byte) error {
	p.closedMu.RLock()
	if p.closed {
		p.closedMu.RUnlock()
		return fmt.Errorf("provider is closed")
	}
	p.closedMu.RUnlock()

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

	// Schedule push if remote is configured
	p.schedulePush()

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
	p.closedMu.RLock()
	if p.closed {
		p.closedMu.RUnlock()
		return fmt.Errorf("provider is closed")
	}
	p.closedMu.RUnlock()

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

	// Schedule push if remote is configured
	p.schedulePush()

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

// schedulePush schedules a push after the debounce delay.
// If called multiple times within the delay, only one push occurs.
func (p *GitFileProvider) schedulePush() {
	if p.remoteURL == "" {
		return
	}

	p.pushMu.Lock()
	defer p.pushMu.Unlock()

	// Cancel existing timer if any
	if p.pushTimer != nil {
		p.pushTimer.Stop()
	}

	p.pendingPush = true
	p.pushTimer = time.AfterFunc(p.pushDebounceDelay, func() {
		p.executePush()
	})
}

// executePush performs the actual push operation.
func (p *GitFileProvider) executePush() {
	p.closedMu.RLock()
	if p.closed {
		p.closedMu.RUnlock()
		return
	}
	p.closedMu.RUnlock()

	p.pushMu.Lock()
	if !p.pendingPush {
		p.pushMu.Unlock()
		return
	}
	p.pendingPush = false
	p.pushMu.Unlock()

	p.wg.Add(1)
	defer p.wg.Done()

	// Get current HEAD to determine local branch name
	head, err := p.repo.Head()
	if err != nil {
		log.Printf("git push failed: could not get HEAD: %v", err)
		return
	}

	localBranch := head.Name().Short()
	// Push local branch to configured remote branch name
	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", localBranch, p.branch))
	err = p.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       p.auth,
		RefSpecs:   []config.RefSpec{refSpec},
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Printf("git push failed: %v", err)
	}
}

// Close flushes any pending push and releases resources.
// It blocks until the pending push completes or timeout.
func (p *GitFileProvider) Close() error {
	p.closedMu.Lock()
	if p.closed {
		p.closedMu.Unlock()
		return nil
	}
	p.closed = true
	p.closedMu.Unlock()

	// Cancel debounce timer and push immediately if pending
	p.pushMu.Lock()
	if p.pushTimer != nil {
		p.pushTimer.Stop()
		p.pushTimer = nil
	}
	shouldPush := p.pendingPush
	p.pendingPush = false
	p.pushMu.Unlock()

	if shouldPush && p.remoteURL != "" {
		p.wg.Add(1)
		// Get current HEAD to determine local branch name
		head, err := p.repo.Head()
		if err != nil {
			p.wg.Done()
			log.Printf("git push on close failed: could not get HEAD: %v", err)
		} else {
			localBranch := head.Name().Short()
			refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", localBranch, p.branch))
			err = p.repo.Push(&git.PushOptions{
				RemoteName: "origin",
				Auth:       p.auth,
				RefSpecs:   []config.RefSpec{refSpec},
			})
			p.wg.Done()

			if err != nil && err != git.NoErrAlreadyUpToDate {
				log.Printf("git push on close failed: %v", err)
			}
		}
	}

	// Wait for any in-flight operations with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for pending operations")
	}
}
