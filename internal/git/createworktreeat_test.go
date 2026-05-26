package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo creates a minimal git repository in a temp directory and returns its path.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}

func TestCreateWorktreeAt(t *testing.T) {
	t.Run("happy path creates worktree at target path", func(t *testing.T) {
		repoDir := setupGitRepo(t)
		target := filepath.Join(t.TempDir(), "my-worktree")

		err := CreateWorktreeAt(repoDir, target, "feature-branch", "HEAD")
		if err != nil {
			t.Fatalf("CreateWorktreeAt unexpectedly failed: %v", err)
		}

		if _, statErr := os.Stat(target); statErr != nil {
			t.Errorf("worktree directory not created at %s: %v", target, statErr)
		}
	})

	t.Run("pre-existing file returns ErrWorktreePathExists", func(t *testing.T) {
		repoDir := setupGitRepo(t)
		target := filepath.Join(t.TempDir(), "sentinel-file")

		sentinel := []byte("sentinel content")
		if err := os.WriteFile(target, sentinel, 0644); err != nil {
			t.Fatalf("setup: failed to create sentinel file: %v", err)
		}

		err := CreateWorktreeAt(repoDir, target, "feature-branch", "HEAD")
		if !errors.Is(err, ErrWorktreePathExists) {
			t.Errorf("expected ErrWorktreePathExists, got: %v", err)
		}

		// File content must be unchanged.
		got, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("sentinel file missing after call: %v", readErr)
		}
		if string(got) != string(sentinel) {
			t.Errorf("sentinel file content changed: got %q, want %q", got, sentinel)
		}
	})

	t.Run("pre-existing directory returns ErrWorktreePathExists", func(t *testing.T) {
		repoDir := setupGitRepo(t)
		target := filepath.Join(t.TempDir(), "existing-dir")

		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatalf("setup: failed to create directory: %v", err)
		}

		err := CreateWorktreeAt(repoDir, target, "feature-branch", "HEAD")
		if !errors.Is(err, ErrWorktreePathExists) {
			t.Errorf("expected ErrWorktreePathExists, got: %v", err)
		}

		// Directory must still exist.
		if _, statErr := os.Stat(target); statErr != nil {
			t.Errorf("directory was removed after ErrWorktreePathExists: %v", statErr)
		}
	})
}
