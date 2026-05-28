package initwizard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

func testWizardInput() WizardInput {
	return WizardInput{
		Adapter:         "jira",
		Site:            "testorg.atlassian.net",
		Email:           "user@example.com",
		BoardID:         "board-1",
		BoardName:       "My Board",
		WorktreeBaseDir: "/tmp/worktrees",
		DefaultApproach: "main",
		Repos:           nil,
	}
}

func TestFinalize_HappyPath(t *testing.T) {
	dir := t.TempDir()
	input := testWizardInput()
	r := strings.NewReader("y\n")
	var w bytes.Buffer

	if err := Finalize(context.Background(), &w, r, input, dir); err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	// config.json
	cfgPath := filepath.Join(dir, config.ConfigFileName)
	var cfg config.Config
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.json missing: %v", err)
	}
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		t.Fatalf("config.json parse error: %v", err)
	}
	if cfg.SchemaVersion != 2 {
		t.Errorf("config schema_version: want 2, got %d", cfg.SchemaVersion)
	}
	cfgFi, _ := os.Stat(cfgPath)
	if cfgFi.Mode().Perm() != 0644 {
		t.Errorf("config.json mode: want 0644, got %04o", cfgFi.Mode().Perm())
	}

	// state.json
	statePath := filepath.Join(dir, config.StateFileName)
	var st state.State
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state.json missing: %v", err)
	}
	if err := json.Unmarshal(stateData, &st); err != nil {
		t.Fatalf("state.json parse error: %v", err)
	}
	if st.SchemaVersion != 2 {
		t.Errorf("state schema_version: want 2, got %d", st.SchemaVersion)
	}
	if len(st.Tickets) != 0 {
		t.Errorf("expected empty tickets, got %d", len(st.Tickets))
	}
	stateFi, _ := os.Stat(statePath)
	if stateFi.Mode().Perm() != 0644 {
		t.Errorf("state.json mode: want 0644, got %04o", stateFi.Mode().Perm())
	}

	// credentials.json
	credsPath := filepath.Join(dir, config.CredentialsFileName)
	credsData, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("credentials.json missing: %v", err)
	}
	var credFile credentialsFile
	if err := json.Unmarshal(credsData, &credFile); err != nil {
		t.Fatalf("credentials.json parse error: %v", err)
	}
	if credFile.Adapters["jira"].SiteURL != input.Site {
		t.Errorf("credentials.json: site mismatch: got %q, want %q",
			credFile.Adapters["jira"].SiteURL, input.Site)
	}
	if credFile.Adapters["jira"].AuthMethod != "acli" {
		t.Errorf("credentials.json: auth_method: got %q, want %q",
			credFile.Adapters["jira"].AuthMethod, "acli")
	}
	credsFi, _ := os.Stat(credsPath)
	if credsFi.Mode().Perm() != 0600 {
		t.Errorf("credentials.json mode: want 0600, got %04o", credsFi.Mode().Perm())
	}

	// Dock snippet and template help in output
	if !strings.Contains(w.String(), "dock.json") {
		t.Errorf("expected dock.json hint in output, got: %q", w.String())
	}
	if !strings.Contains(w.String(), "{{.Ticket.Key}}") {
		t.Errorf("expected template help in output, got: %q", w.String())
	}
}

func TestFinalize_UserAborts(t *testing.T) {
	dir := t.TempDir()
	input := testWizardInput()
	r := strings.NewReader("n\n")
	var w bytes.Buffer

	err := Finalize(context.Background(), &w, r, input, dir)
	if err != nil {
		t.Fatalf("expected nil on abort, got: %v", err)
	}

	// No files written
	for _, name := range []string{config.ConfigFileName, config.StateFileName, config.CredentialsFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be absent after abort", name)
		}
	}
	if !strings.Contains(w.String(), "Aborted") {
		t.Errorf("expected 'Aborted' in output, got: %q", w.String())
	}
}

func TestFinalize_TwoReposPersisted(t *testing.T) {
	dir := t.TempDir()
	input := testWizardInput()
	input.Repos = []config.RepoEntry{
		{ID: "repo-a", Name: "Repo A", Path: "/a", DefaultBranch: "main"},
		{ID: "repo-b", Name: "Repo B", Path: "/b", DefaultBranch: "main"},
	}
	r := strings.NewReader("y\n")
	var w bytes.Buffer

	if err := Finalize(context.Background(), &w, r, input, dir); err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	cfgData, _ := os.ReadFile(filepath.Join(dir, config.ConfigFileName))
	var cfg config.Config
	json.Unmarshal(cfgData, &cfg)
	if len(cfg.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(cfg.Repos))
	}
	if _, ok := cfg.Repos["repo-a"]; !ok {
		t.Errorf("repo-a not in config.json repos")
	}
	if _, ok := cfg.Repos["repo-b"]; !ok {
		t.Errorf("repo-b not in config.json repos")
	}
}

func TestFinalize_ZeroRepos(t *testing.T) {
	dir := t.TempDir()
	input := testWizardInput()
	input.Repos = nil
	r := strings.NewReader("y\n")
	var w bytes.Buffer

	if err := Finalize(context.Background(), &w, r, input, dir); err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	cfgData, _ := os.ReadFile(filepath.Join(dir, config.ConfigFileName))
	var cfg config.Config
	json.Unmarshal(cfgData, &cfg)
	if len(cfg.Repos) != 0 {
		t.Errorf("expected empty repos map, got %d entries", len(cfg.Repos))
	}
}
