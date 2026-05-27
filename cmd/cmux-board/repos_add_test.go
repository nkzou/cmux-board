package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/config"
)

// setupTempConfigDir sets CMUX_BOARD_CONFIG_DIR to a new temp dir for the duration of t.
// Returns the temp dir path. The directory is empty (no config.json yet).
func setupTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	return dir
}

// initTempGitRepo initialises a bare git repo in a new temp dir and returns its path.
func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@test.com")
	runCmd(t, dir, "git", "config", "user.name", "Test")
	runCmd(t, dir, "git", "commit", "--allow-empty", "-m", "init")
	return dir
}

// runCmd runs name+args in dir, failing the test on error.
func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// readConfigRepos loads config.json from dir and returns the repos map.
func readConfigRepos(t *testing.T, dir string) map[string]config.RepoEntry {
	t.Helper()
	cfgPath := filepath.Join(dir, config.ConfigFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read config: %v", err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return cfg.Repos
}

// configBytes returns the raw bytes of config.json from dir, or nil if absent.
func configBytes(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, config.ConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read config: %v", err)
	}
	return data
}

func TestReposAddCmd_HappyPath(t *testing.T) {
	configDir := setupTempConfigDir(t)
	repoDir := initTempGitRepo(t)

	cmd := newReposAddCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, []string{repoDir}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	// Config should have a new entry.
	repos := readConfigRepos(t, configDir)
	if len(repos) == 0 {
		t.Fatal("expected at least one repo in config.json")
	}

	// Stdout should contain the derived id and abs path.
	out := buf.String()
	if !strings.Contains(out, repoDir) {
		t.Errorf("stdout missing repo path %q; got: %s", repoDir, out)
	}
}

func TestReposAddCmd_NonGitPath(t *testing.T) {
	configDir := setupTempConfigDir(t)

	// Pre-call: ensure config does not exist.
	beforeBytes := configBytes(t, configDir)

	cmd := newReposAddCmd()
	err := cmd.RunE(cmd, []string{"/tmp/cmux-board-not-a-repo-xyzzy"})
	if err == nil {
		t.Fatal("expected error for non-git path, got nil")
	}
	if !errors.Is(err, config.ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}

	// Config should be byte-equal to pre-call (nil or unchanged).
	afterBytes := configBytes(t, configDir)
	if string(beforeBytes) != string(afterBytes) {
		t.Errorf("config.json changed on error; before=%q after=%q", beforeBytes, afterBytes)
	}
}

func TestReposAddCmd_ExplicitIDFlag(t *testing.T) {
	configDir := setupTempConfigDir(t)
	repoDir := initTempGitRepo(t)

	cmd := newReposAddCmd()
	if err := cmd.Flags().Set("id", "custom-slug"); err != nil {
		t.Fatalf("set --id flag: %v", err)
	}
	if err := cmd.RunE(cmd, []string{repoDir}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	repos := readConfigRepos(t, configDir)
	if _, ok := repos["custom-slug"]; !ok {
		t.Errorf("expected 'custom-slug' key in repos, got keys: %v", repoKeys(repos))
	}
}

func TestReposAddCmd_ExplicitNameAndBranch(t *testing.T) {
	configDir := setupTempConfigDir(t)
	repoDir := initTempGitRepo(t)

	cmd := newReposAddCmd()
	if err := cmd.Flags().Set("name", "My Repo"); err != nil {
		t.Fatalf("set --name: %v", err)
	}
	if err := cmd.Flags().Set("default-branch", "HEAD"); err != nil {
		t.Fatalf("set --default-branch: %v", err)
	}
	if err := cmd.RunE(cmd, []string{repoDir}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	repos := readConfigRepos(t, configDir)
	for _, entry := range repos {
		if entry.Name != "My Repo" {
			t.Errorf("Name = %q, want 'My Repo'", entry.Name)
		}
		if entry.DefaultBranch != "HEAD" {
			t.Errorf("DefaultBranch = %q, want 'HEAD'", entry.DefaultBranch)
		}
	}
}

func TestReposAddCmd_DuplicateID(t *testing.T) {
	setupTempConfigDir(t)
	repoDir := initTempGitRepo(t)

	// Add once.
	cmd := newReposAddCmd()
	if err := cmd.Flags().Set("id", "dup-test"); err != nil {
		t.Fatalf("set --id: %v", err)
	}
	if err := cmd.RunE(cmd, []string{repoDir}); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Add again with same id — need fresh cobra cmd to reset flags.
	cmd2 := newReposAddCmd()
	if err := cmd2.Flags().Set("id", "dup-test"); err != nil {
		t.Fatalf("set --id: %v", err)
	}
	err := cmd2.RunE(cmd2, []string{repoDir})
	if err == nil {
		t.Fatal("expected error for duplicate repo_id, got nil")
	}
	if !errors.Is(err, config.ErrDuplicateRepoID) {
		t.Errorf("expected ErrDuplicateRepoID, got %v", err)
	}
}

func repoKeys(m map[string]config.RepoEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
