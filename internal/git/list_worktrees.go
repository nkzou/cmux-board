package git

import (
	"context"
	"fmt"
	"os/exec"
)

// ListWorktrees calls `git -C repoPath worktree list --porcelain` and returns
// the parsed worktree entries. Entries include the main worktree.
// ctx is honored for cancellation.
func ListWorktrees(ctx context.Context, repoPath string) ([]Worktree, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list (repo=%s): %w", repoPath, err)
	}
	return parseWorktreeList(string(output)), nil
}
