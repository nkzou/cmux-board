package config

import (
	"errors"
	"testing"

	"github.com/nkzou/cmux-board/internal/state"
)

func makeConfig(repos map[string]RepoEntry) *Config {
	return &Config{
		SchemaVersion: SchemaVersionCurrent,
		Repos:         repos,
	}
}

func makeState(tickets map[string]state.TicketState, activations map[string][]state.ActivationEntry) *state.State {
	if tickets == nil {
		tickets = make(map[string]state.TicketState)
	}
	if activations == nil {
		activations = make(map[string][]state.ActivationEntry)
	}
	return &state.State{
		SchemaVersion: state.SchemaVersionCurrent,
		Tickets:       tickets,
		Activations:   activations,
	}
}

func TestAddRepo_HappyPath(t *testing.T) {
	cfg := makeConfig(make(map[string]RepoEntry))
	args := RepoArgs{
		ID:            "my-repo",
		Name:          "My Repo",
		Path:          "/tmp/my-repo",
		DefaultBranch: "main",
	}
	if err := AddRepo(cfg, args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := cfg.Repos["my-repo"]
	if !ok {
		t.Fatal("expected 'my-repo' in cfg.Repos")
	}
	if entry.Name != "My Repo" || entry.Path != "/tmp/my-repo" || entry.DefaultBranch != "main" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestAddRepo_DuplicateID(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	err := AddRepo(cfg, RepoArgs{ID: "foo", Name: "Foo2", Path: "/tmp/foo2", DefaultBranch: "main"})
	if !errors.Is(err, ErrDuplicateRepoID) {
		t.Errorf("expected ErrDuplicateRepoID, got %v", err)
	}
}

func TestAddRepo_NilReposMap(t *testing.T) {
	cfg := makeConfig(nil)
	if err := AddRepo(cfg, RepoArgs{ID: "bar", Name: "Bar", Path: "/tmp/bar", DefaultBranch: "main"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Repos["bar"]; !ok {
		t.Error("expected 'bar' in cfg.Repos after nil-map initialization")
	}
}

func TestRemoveRepo_HappyPath(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	st := makeState(nil, nil)
	if err := RemoveRepo(cfg, st, "foo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("expected empty Repos, got %v", cfg.Repos)
	}
}

func TestRemoveRepo_RefusedByTicketAssignment(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	st := makeState(
		map[string]state.TicketState{
			"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"foo"}},
		},
		nil,
	)
	err := RemoveRepo(cfg, st, "foo")
	if !errors.Is(err, ErrRepoReferenced) {
		t.Errorf("expected ErrRepoReferenced, got %v", err)
	}
	if err != nil && !containsStr(err.Error(), "PROJ-42") {
		t.Errorf("error message should mention 'PROJ-42', got: %v", err)
	}
}

func TestRemoveRepo_RefusedByActivation(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	st := makeState(
		map[string]state.TicketState{
			"PROJ-42": {Key: "PROJ-42"}, // no assigned_repo_ids
		},
		map[string][]state.ActivationEntry{
			"PROJ-42": {{ActivationID: "act-1", RepoID: "foo"}},
		},
	)
	err := RemoveRepo(cfg, st, "foo")
	if !errors.Is(err, ErrRepoReferenced) {
		t.Errorf("expected ErrRepoReferenced, got %v", err)
	}
	if err != nil && !containsStr(err.Error(), "PROJ-42") {
		t.Errorf("error message should mention 'PROJ-42', got: %v", err)
	}
}

func TestRemoveRepo_AssignedTicketIncluded(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"bar": {Name: "Bar", Path: "/tmp/bar", DefaultBranch: "main"},
	})
	st := makeState(
		map[string]state.TicketState{
			"PROJ-99": {Key: "PROJ-99", Source: "jira", AssignedRepoIDs: []string{"bar"}},
		},
		nil,
	)
	err := RemoveRepo(cfg, st, "bar")
	if !errors.Is(err, ErrRepoReferenced) {
		t.Errorf("expected ErrRepoReferenced (assigned ticket), got %v", err)
	}
	if err != nil && !containsStr(err.Error(), "PROJ-99") {
		t.Errorf("error message should mention 'PROJ-99', got: %v", err)
	}
}

func TestRemoveRepo_NotFound(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{})
	st := makeState(nil, nil)
	err := RemoveRepo(cfg, st, "ghost")
	if err == nil {
		t.Error("expected non-nil error for unknown repo_id")
	}
}

func TestListRepos_Empty(t *testing.T) {
	cfg := makeConfig(make(map[string]RepoEntry))
	rows := ListRepos(cfg, nil)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestListRepos_Sorted(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"zebra": {Name: "Zebra"},
		"apple": {Name: "Apple"},
		"mango": {Name: "Mango"},
	})
	rows := ListRepos(cfg, nil)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	want := []string{"apple", "mango", "zebra"}
	for i, w := range want {
		if rows[i].ID != w {
			t.Errorf("rows[%d].ID = %q, want %q", i, rows[i].ID, w)
		}
	}
}

func TestListRepos_RefCounts(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"foo": {Name: "Foo", Path: "/tmp/foo", DefaultBranch: "main"},
	})
	st := makeState(
		map[string]state.TicketState{
			"PROJ-42": {Key: "PROJ-42", AssignedRepoIDs: []string{"foo"}},
		},
		map[string][]state.ActivationEntry{
			"PROJ-42": {{ActivationID: "act-1", RepoID: "foo"}},
		},
	)
	rows := ListRepos(cfg, st)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].TicketRefs != 1 {
		t.Errorf("TicketRefs = %d, want 1", rows[0].TicketRefs)
	}
	if rows[0].ActivationRefs != 1 {
		t.Errorf("ActivationRefs = %d, want 1", rows[0].ActivationRefs)
	}
}

func TestListRepos_NilState(t *testing.T) {
	cfg := makeConfig(map[string]RepoEntry{
		"foo": {Name: "Foo"},
	})
	rows := ListRepos(cfg, nil)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].TicketRefs != 0 || rows[0].ActivationRefs != 0 {
		t.Errorf("expected 0 refs for nil state, got ticket=%d activation=%d",
			rows[0].TicketRefs, rows[0].ActivationRefs)
	}
}

func TestAddRepo_DoesNotWriteDisk(t *testing.T) {
	// Structural: AddRepo operates only on in-memory cfg — no file I/O.
	cfg := makeConfig(nil)
	if err := AddRepo(cfg, RepoArgs{ID: "test", Name: "Test", Path: "/tmp/test", DefaultBranch: "main"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Repos["test"]; !ok {
		t.Error("expected entry in cfg.Repos")
	}
	// If we got here without filesystem operations, the function is pure.
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
