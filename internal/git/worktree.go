package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type WorktreeManager struct {
	repoPath string
	baseDir  string
}

func NewWorktreeManagerFromPaths(repoPath, baseDir string) *WorktreeManager {
	return &WorktreeManager{
		repoPath: repoPath,
		baseDir:  baseDir,
	}
}

// CreateWorktreeAt is defined in createworktreeat.go (T-005a)

func (m *WorktreeManager) isValidWorktree(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// Worktrees have a .git file (not directory) pointing to the main repo
	return !info.IsDir()
}

func (m *WorktreeManager) RemoveWorktree(worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = m.repoPath

	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "not a working tree") {
			return fmt.Errorf("failed to remove worktree: %s: %w", string(output), err)
		}
	}

	if _, err := os.Stat(worktreePath); err == nil {
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}
	}

	return nil
}

func (m *WorktreeManager) ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(string(output)), nil
}

type Worktree struct {
	Path   string
	HEAD   string
	Branch string
}

func parseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(output, "\n") {
		if after, found := strings.CutPrefix(line, "worktree "); found {
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: after}
		} else if after, found := strings.CutPrefix(line, "HEAD "); found {
			current.HEAD = after
		} else if after, found := strings.CutPrefix(line, "branch refs/heads/"); found {
			current.Branch = after
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}

func (m *WorktreeManager) GetDefaultBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = m.repoPath

	output, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		branch = strings.TrimPrefix(branch, "refs/remotes/origin/")
		return branch, nil
	}

	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = m.repoPath
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	return "main", nil
}

func (m *WorktreeManager) DeleteBranch(branchName string) error {
	cmd := exec.Command("git", "branch", "-D", branchName)
	cmd.Dir = m.repoPath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch: %s: %w", string(output), err)
	}

	return nil
}

func (m *WorktreeManager) BranchExists(branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = m.repoPath
	return cmd.Run() == nil
}

func (m *WorktreeManager) CreateBranch(branchName, baseBranch string) error {
	cmd := exec.Command("git", "branch", branchName, baseBranch)
	cmd.Dir = m.repoPath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %s: %w", string(output), err)
	}

	return nil
}

func (m *WorktreeManager) CheckoutBranch(branchName string) error {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = m.repoPath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout branch: %s: %w", string(output), err)
	}

	return nil
}

func (m *WorktreeManager) SetupBranch(branchName, baseBranch string) error {
	if !m.BranchExists(branchName) {
		if err := m.CreateBranch(branchName, baseBranch); err != nil {
			return err
		}
	}
	return m.CheckoutBranch(branchName)
}

func (m *WorktreeManager) HasUncommittedChanges(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func sanitizeBranchName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	s := b.String()
	// collapse consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func ResolveMainRepo(path string) string {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return path
	}

	if info.IsDir() {
		return path
	}

	content, err := os.ReadFile(gitPath)
	if err != nil {
		return path
	}

	line := strings.TrimSpace(string(content))
	if after, found := strings.CutPrefix(line, "gitdir: "); found {
		if idx, _, hasWorktrees := strings.Cut(after, "/.git/worktrees/"); hasWorktrees {
			return idx
		}
	}

	return path
}
