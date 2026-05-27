package initwizard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevin-zou/cmux-board/internal/config"
)

// repoIDPattern matches valid repo IDs: starts with alphanumeric, rest alphanumeric or dash.
var repoIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// maxRepoPathRetries is the maximum number of attempts per path entry.
const maxRepoPathRetries = 3

// ValidatorFunc validates an absolute path is a git repo.
type ValidatorFunc func(path string) error

// BranchResolverFunc resolves the default branch for a repo at path.
type BranchResolverFunc func(path string) string

// defaultValidator wraps config.ValidateRepoPath.
func defaultValidator(path string) error {
	return config.ValidateRepoPath(path)
}

// defaultBranchResolver tries git symbolic-ref to resolve the default branch.
// Falls back to "main" on any failure.
func defaultBranchResolver(path string) string {
	cmd := exec.Command("git", "-C", path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(out))
	// symbolic-ref returns "origin/main" format; strip "origin/" prefix if present.
	if idx := strings.Index(branch, "/"); idx >= 0 {
		branch = branch[idx+1:]
	}
	if branch == "" {
		return "main"
	}
	return branch
}

// RegisterRepos runs the optional repo registration loop.
// If repoPaths is non-empty (from --repo flag), each path is registered non-interactively
// with defaults and the function returns immediately.
// In interactive mode, the loop repeats until the user enters an empty path.
// Returns the slice of config.RepoEntry values to be merged into config.json.
// On skip (zero paths), returns nil, nil.
// validator and branchResolver may be nil; defaults are used when nil.
func RegisterRepos(
	ctx context.Context,
	w io.Writer,
	r io.Reader,
	repoPaths []string,
	validator ValidatorFunc,
	branchResolver BranchResolverFunc,
) ([]config.RepoEntry, error) {
	if validator == nil {
		validator = defaultValidator
	}
	if branchResolver == nil {
		branchResolver = defaultBranchResolver
	}

	var entries []config.RepoEntry
	usedIDs := map[string]bool{}

	if len(repoPaths) > 0 {
		// Flag-override mode: register each path non-interactively.
		for _, path := range repoPaths {
			if err := validator(path); err != nil {
				return nil, fmt.Errorf("invalid repo path %q: %w", path, err)
			}
			entry := buildEntryWithDefaults(path, branchResolver)
			// Ensure unique ID in this session.
			entry.ID = uniqueID(entry.ID, usedIDs)
			usedIDs[entry.ID] = true
			entries = append(entries, entry)
			fmt.Fprintf(w, "Registered: %s → %s\n", entry.ID, entry.Path)
		}
		fmt.Fprintln(w, "You can add more repos later with: cmux-board repos add <path>")
		return entries, nil
	}

	// Interactive mode.
	reader := bufio.NewReader(r)
	for {
		path, err := promptRepoPath(reader, w, validator)
		if err != nil {
			return entries, err
		}
		if path == "" {
			// User pressed Enter with empty input → skip.
			break
		}

		// Display name
		defaultName := filepath.Base(path)
		name, err := promptWithDefault(reader, w, "Display name", defaultName)
		if err != nil {
			return entries, err
		}
		if name == "" {
			name = defaultName
		}

		// Repo ID
		defaultID := config.DeriveRepoID(name)
		repoID, err := promptRepoID(reader, w, defaultID, usedIDs)
		if err != nil {
			return entries, err
		}
		usedIDs[repoID] = true

		// Default branch
		resolvedBranch := branchResolver(path)
		branch, err := promptWithDefault(reader, w, "Default branch", resolvedBranch)
		if err != nil {
			return entries, err
		}
		if branch == "" {
			branch = resolvedBranch
		}

		entry := config.RepoEntry{
			ID:            repoID,
			Name:          name,
			Path:          path,
			DefaultBranch: branch,
		}
		entries = append(entries, entry)
		fmt.Fprintf(w, "Registered: %s → %s\n", entry.ID, entry.Path)

		// Ask to add another.
		fmt.Fprint(w, "Add another repo? [y/N]: ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			break
		}
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			break
		}
	}

	fmt.Fprintln(w, "You can add more repos later with: cmux-board repos add <path>")
	return entries, nil
}

// promptRepoPath prompts for an absolute path, retrying up to maxRepoPathRetries on
// validation failure. Returns empty string if user presses Enter with no input.
func promptRepoPath(reader *bufio.Reader, w io.Writer, validator ValidatorFunc) (string, error) {
	fmt.Fprint(w, "Repository path (absolute, leave empty to skip): ")
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", nil
	}
	path := strings.TrimSpace(line)
	if path == "" {
		return "", nil
	}

	// Validate on first attempt.
	if err := validator(path); err == nil {
		return path, nil
	}

	// Retry up to maxRepoPathRetries - 1 more times.
	for attempt := 1; attempt < maxRepoPathRetries; attempt++ {
		fmt.Fprintf(w, "Path is not a valid git repo. Try again (attempt %d/%d): ", attempt+1, maxRepoPathRetries)
		line, err = reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", fmt.Errorf("reading repo path: %w", err)
		}
		path = strings.TrimSpace(line)
		if path == "" {
			return "", nil
		}
		if err := validator(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("failed to provide a valid git repo path after %d attempts", maxRepoPathRetries)
}

// promptWithDefault prints a prompt with a default value in brackets.
func promptWithDefault(reader *bufio.Reader, w io.Writer, label, defaultVal string) (string, error) {
	fmt.Fprintf(w, "%s [%s]: ", label, defaultVal)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return defaultVal, nil
	}
	val := strings.TrimSpace(line)
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

// promptRepoID prompts for a repo ID, validating format and uniqueness.
func promptRepoID(reader *bufio.Reader, w io.Writer, defaultID string, used map[string]bool) (string, error) {
	for {
		fmt.Fprintf(w, "Repo ID [%s]: ", defaultID)
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return defaultID, nil
		}
		id := strings.TrimSpace(line)
		if id == "" {
			id = defaultID
		}
		if !repoIDPattern.MatchString(id) {
			fmt.Fprintln(w, "Repo ID must match ^[a-z0-9][a-z0-9-]*$. Try again.")
			continue
		}
		if used[id] {
			fmt.Fprintf(w, "Repo ID %q is already used in this session. Try another.\n", id)
			continue
		}
		return id, nil
	}
}

// buildEntryWithDefaults creates a RepoEntry for a path using all defaults.
func buildEntryWithDefaults(path string, branchResolver BranchResolverFunc) config.RepoEntry {
	name := filepath.Base(path)
	id := config.DeriveRepoID(name)
	if id == "" {
		id = "repo"
	}
	branch := branchResolver(path)
	return config.RepoEntry{
		ID:            id,
		Name:          name,
		Path:          path,
		DefaultBranch: branch,
	}
}

// uniqueID ensures id is not in used by appending a numeric suffix.
func uniqueID(id string, used map[string]bool) string {
	if !used[id] {
		return id
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", id, i)
		if !used[candidate] {
			return candidate
		}
	}
}
