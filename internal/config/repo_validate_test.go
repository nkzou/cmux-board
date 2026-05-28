package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateRepoPath_StandardClone(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRepoPath(dir); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRepoPath_GitdirFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /some/path/.git\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRepoPath(dir); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRepoPath_Missing(t *testing.T) {
	dir := t.TempDir()
	// No .git entry.
	err := ValidateRepoPath(dir)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestValidateRepoPath_PlainFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("not-gitdir\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := ValidateRepoPath(dir)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestValidateDefaultBranch_Exists(t *testing.T) {
	dir := t.TempDir()
	// Initialize a real git repo with an empty commit so HEAD resolves.
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")

	if err := ValidateDefaultBranch(dir, "HEAD"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateDefaultBranch_Missing(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")

	err := ValidateDefaultBranch(dir, "refs/heads/nonexistent")
	if !errors.Is(err, ErrBranchNotFound) {
		t.Errorf("expected ErrBranchNotFound, got %v", err)
	}
}

func TestValidateRepoPath_PathDoesNotExist(t *testing.T) {
	err := ValidateRepoPath("/tmp/cmux-board-test-nonexistent-xyzzy")
	if !errors.Is(err, ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
}

// runGit runs git with the given args in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
