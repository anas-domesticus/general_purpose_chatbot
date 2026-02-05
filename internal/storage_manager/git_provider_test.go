package storage_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestGitFileProvider_NewGitFileProvider(t *testing.T) {
	t.Run("fails with empty path", func(t *testing.T) {
		_, err := NewGitFileProvider(GitProviderOptions{})
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("fails when repo doesn't exist and InitIfMissing is false", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "nonexistent")

		_, err := NewGitFileProvider(GitProviderOptions{
			Path:          repoPath,
			InitIfMissing: false,
		})
		if err == nil {
			t.Error("expected error when repo doesn't exist")
		}
	})

	t.Run("creates repo when InitIfMissing is true", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "newrepo")

		provider, err := NewGitFileProvider(GitProviderOptions{
			Path:          repoPath,
			InitIfMissing: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected provider to be non-nil")
		}

		// Verify .git directory exists
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			t.Error("expected .git directory to exist")
		}
	})

	t.Run("opens existing repo", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Initialize a repo manually
		_, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init test repo: %v", err)
		}

		provider, err := NewGitFileProvider(GitProviderOptions{
			Path: tmpDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("expected provider to be non-nil")
		}
	})
}

func TestGitFileProvider_ReadWrite(t *testing.T) {
	provider := createTestGitProvider(t)
	ctx := context.Background()

	t.Run("write and read file", func(t *testing.T) {
		content := []byte("hello world")
		err := provider.Write(ctx, "test.txt", content)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		data, err := provider.Read(ctx, "test.txt")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		if string(data) != string(content) {
			t.Errorf("expected %q, got %q", content, data)
		}
	})

	t.Run("write creates commit", func(t *testing.T) {
		content := []byte("commit test")
		err := provider.Write(ctx, "commit-test.txt", content)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Check commit history
		ref, err := provider.repo.Head()
		if err != nil {
			t.Fatalf("failed to get HEAD: %v", err)
		}

		commit, err := provider.repo.CommitObject(ref.Hash())
		if err != nil {
			t.Fatalf("failed to get commit: %v", err)
		}

		expectedMsg := "[auto] Write commit-test.txt"
		if commit.Message != expectedMsg {
			t.Errorf("expected commit message %q, got %q", expectedMsg, commit.Message)
		}
	})

	t.Run("write creates nested directories", func(t *testing.T) {
		content := []byte("nested content")
		err := provider.Write(ctx, "a/b/c/nested.txt", content)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		data, err := provider.Read(ctx, "a/b/c/nested.txt")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		if string(data) != string(content) {
			t.Errorf("expected %q, got %q", content, data)
		}
	})

	t.Run("read nonexistent file returns error", func(t *testing.T) {
		_, err := provider.Read(ctx, "nonexistent.txt")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestGitFileProvider_Exists(t *testing.T) {
	provider := createTestGitProvider(t)
	ctx := context.Background()

	t.Run("returns false for nonexistent file", func(t *testing.T) {
		exists, err := provider.Exists(ctx, "nonexistent.txt")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected file to not exist")
		}
	})

	t.Run("returns true for existing file", func(t *testing.T) {
		err := provider.Write(ctx, "exists.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		exists, err := provider.Exists(ctx, "exists.txt")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected file to exist")
		}
	})
}

func TestGitFileProvider_Delete(t *testing.T) {
	provider := createTestGitProvider(t)
	ctx := context.Background()

	t.Run("delete existing file", func(t *testing.T) {
		// Create a file first
		err := provider.Write(ctx, "to-delete.txt", []byte("delete me"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Delete it
		err = provider.Delete(ctx, "to-delete.txt")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify it's gone
		exists, err := provider.Exists(ctx, "to-delete.txt")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected file to be deleted")
		}
	})

	t.Run("delete nonexistent file is idempotent", func(t *testing.T) {
		err := provider.Delete(ctx, "never-existed.txt")
		if err != nil {
			t.Errorf("Delete should not error for nonexistent file: %v", err)
		}
	})

	t.Run("delete creates commit", func(t *testing.T) {
		// Create and delete a file
		err := provider.Write(ctx, "delete-commit.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		err = provider.Delete(ctx, "delete-commit.txt")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Check commit message
		ref, err := provider.repo.Head()
		if err != nil {
			t.Fatalf("failed to get HEAD: %v", err)
		}

		commit, err := provider.repo.CommitObject(ref.Hash())
		if err != nil {
			t.Fatalf("failed to get commit: %v", err)
		}

		expectedMsg := "[auto] Delete delete-commit.txt"
		if commit.Message != expectedMsg {
			t.Errorf("expected commit message %q, got %q", expectedMsg, commit.Message)
		}
	})
}

func TestGitFileProvider_List(t *testing.T) {
	provider := createTestGitProvider(t)
	ctx := context.Background()

	t.Run("list empty directory returns empty slice", func(t *testing.T) {
		files, err := provider.List(ctx, "empty-prefix")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected empty slice, got %v", files)
		}
	})

	t.Run("list returns files with prefix", func(t *testing.T) {
		// Create some files
		_ = provider.Write(ctx, "list/a.txt", []byte("a"))
		_ = provider.Write(ctx, "list/b.txt", []byte("b"))
		_ = provider.Write(ctx, "list/sub/c.txt", []byte("c"))
		_ = provider.Write(ctx, "other/d.txt", []byte("d"))

		files, err := provider.List(ctx, "list")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d: %v", len(files), files)
		}

		// Verify expected files are present
		expected := map[string]bool{
			"list/a.txt":     true,
			"list/b.txt":     true,
			"list/sub/c.txt": true,
		}
		for _, f := range files {
			if !expected[f] {
				t.Errorf("unexpected file in list: %s", f)
			}
		}
	})

	t.Run("list root returns all files", func(t *testing.T) {
		provider := createTestGitProvider(t)

		_ = provider.Write(ctx, "root1.txt", []byte("1"))
		_ = provider.Write(ctx, "root2.txt", []byte("2"))
		_ = provider.Write(ctx, "sub/file.txt", []byte("3"))

		files, err := provider.List(ctx, "")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d: %v", len(files), files)
		}
	})
}

// createTestGitProvider creates a GitFileProvider in a temporary directory for testing.
func createTestGitProvider(t *testing.T) *GitFileProvider {
	t.Helper()
	tmpDir := t.TempDir()

	provider, err := NewGitFileProvider(GitProviderOptions{
		Path:          tmpDir,
		AuthorName:    "Test User",
		AuthorEmail:   "test@example.com",
		InitIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to create test provider: %v", err)
	}

	return provider
}
