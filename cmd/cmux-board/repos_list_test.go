package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

func TestReposListCmd_EmptyRepos(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{})

	cmd := newReposListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("output missing 'ID' header; got: %s", out)
	}
	// No data rows — should just have two header lines.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 2 {
		t.Errorf("expected only header rows for empty repos, got %d lines:\n%s", len(lines), out)
	}
}

func TestReposListCmd_OneRepo(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"my-repo": {Name: "My Repo", Path: "/tmp/x", DefaultBranch: "main"},
	})

	cmd := newReposListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"my-repo", "My Repo", "/tmp/x", "main", "0 tickets / 0 activations"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

func TestReposListCmd_SortedOrder(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"zebra": {Name: "Zebra"},
		"apple": {Name: "Apple"},
		"mango": {Name: "Mango"},
	})

	cmd := newReposListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := buf.String()
	applePos := strings.Index(out, "apple")
	mangoPos := strings.Index(out, "mango")
	zebraPos := strings.Index(out, "zebra")

	if applePos < 0 || mangoPos < 0 || zebraPos < 0 {
		t.Fatalf("one or more repo IDs missing from output: %s", out)
	}
	if !(applePos < mangoPos && mangoPos < zebraPos) {
		t.Errorf("repos not in sorted order; apple=%d mango=%d zebra=%d\n%s",
			applePos, mangoPos, zebraPos, out)
	}
}

func TestReposListCmd_JSONOutput(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"my-repo": {Name: "My Repo", Path: "/tmp/x", DefaultBranch: "main"},
	})

	cmd := newReposListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set --json: %v", err)
	}

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var rows []repoJSONRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.ID != "my-repo" || r.Name != "My Repo" || r.Path != "/tmp/x" || r.DefaultBranch != "main" {
		t.Errorf("unexpected JSON row: %+v", r)
	}
	if r.TicketRefs != 0 || r.ActivationRefs != 0 {
		t.Errorf("expected 0 refs, got ticket=%d activation=%d", r.TicketRefs, r.ActivationRefs)
	}
}

func TestReposListCmd_JSONWithRefs(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	writeStateWithTickets(t, configDir,
		map[string]state.TicketState{
			"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"foo"}},
		},
		map[string][]state.ActivationEntry{
			"PROJ-42": {{ActivationID: "act-1", RepoID: "foo"}},
		},
	)

	cmd := newReposListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set --json: %v", err)
	}

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var rows []repoJSONRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].TicketRefs != 1 || rows[0].ActivationRefs != 1 {
		t.Errorf("expected ticket=1 activation=1, got ticket=%d activation=%d",
			rows[0].TicketRefs, rows[0].ActivationRefs)
	}
}

func TestReposListCmd_CorruptStateNonFatal(t *testing.T) {
	configDir := setupTempConfigDir(t)
	writeConfigWithRepos(t, configDir, map[string]config.RepoEntry{
		"my-repo": {Name: "My Repo", Path: "/tmp/x", DefaultBranch: "main"},
	})
	// Write garbage to state.json.
	statePath := filepath.Join(configDir, config.StateFileName)
	if err := os.WriteFile(statePath, []byte("{garbage not json"), 0644); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}

	cmd := newReposListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Should exit 0 even with corrupt state.json.
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("expected success with corrupt state.json, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "my-repo") {
		t.Errorf("expected 'my-repo' in output, got: %s", out)
	}
	if !strings.Contains(out, "0 tickets / 0 activations") {
		t.Errorf("expected 0 refs for corrupt state, got: %s", out)
	}
}
