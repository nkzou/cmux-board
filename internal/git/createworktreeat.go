package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ErrWorktreePathExists is returned by CreateWorktreeAt when absPath already exists on disk.
// The caller is responsible for path allocation and uniquification.
var ErrWorktreePathExists = errors.New("worktree path already exists")

// CreateWorktreeAt creates a git worktree at an exact absolute path.
// If absPath already exists (file, directory, or existing worktree), returns ErrWorktreePathExists
// and performs no other action. The caller is responsible for path allocation and uniquification.
func CreateWorktreeAt(repoPath, absPath, branchName, baseBranch string) error {
	// Pre-check: refuse if path exists
	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("%w: %s", ErrWorktreePathExists, absPath)
	}
	// Execute: git worktree add <absPath> -b <branchName> <baseBranch>
	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", absPath, "-b", branchName, baseBranch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree at %s: %w\noutput: %s", absPath, err, out)
	}
	return nil
}
