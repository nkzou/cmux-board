package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotGitRepo is returned when the path is not a git repository.
var ErrNotGitRepo = fmt.Errorf("path is not a git repository")

// ErrBranchNotFound is returned when the branch ref does not resolve.
var ErrBranchNotFound = fmt.Errorf("default branch ref not found")

// ValidateRepoPath checks that absPath exists on disk and is a real git repository.
// A real git repository is defined as:
//   - absPath/.git is a directory (standard clone), OR
//   - absPath/.git is a regular file whose first line begins with "gitdir:" (git worktree
//     or submodule).
//
// Returns ErrNotGitRepo if neither condition holds, a wrapped os error on I/O failure.
func ValidateRepoPath(absPath string) error {
	gitEntry := filepath.Join(absPath, ".git")
	fi, err := os.Stat(gitEntry)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: no .git entry at %s", ErrNotGitRepo, absPath)
		}
		return fmt.Errorf("failed to stat %s: %w", gitEntry, err)
	}
	if fi.IsDir() {
		return nil // standard clone
	}
	// Regular file — check for "gitdir:" pointer (linked worktree / submodule).
	data, err := os.ReadFile(gitEntry)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", gitEntry, err)
	}
	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	if strings.HasPrefix(firstLine, "gitdir:") {
		return nil
	}
	return fmt.Errorf("%w: .git file at %s does not start with 'gitdir:'", ErrNotGitRepo, absPath)
}

// ValidateDefaultBranch checks that branchRef resolves within the repo at repoPath by
// running: git -C <repoPath> rev-parse --verify <branchRef>
//
// Returns ErrBranchNotFound if the command exits non-zero (ref not found),
// a wrapped exec error on invocation failure.
func ValidateDefaultBranch(repoPath, branchRef string) error {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branchRef)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %q in %s (%s)",
			ErrBranchNotFound, branchRef, repoPath, strings.TrimSpace(string(out)))
	}
	return nil
}

// ResolveDefaultBranch attempts to determine the default branch for a repo by running
// git symbolic-ref --short refs/remotes/origin/HEAD. Falls back to HEAD if that fails.
func ResolveDefaultBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(out))
		// symbolic-ref returns "origin/main" — strip the remote prefix.
		if idx := strings.Index(branch, "/"); idx >= 0 {
			branch = branch[idx+1:]
		}
		if branch != "" {
			return branch, nil
		}
	}
	// Fallback: use HEAD.
	return "HEAD", nil
}
