package ui

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/cmuxcli"
	"github.com/nkzou/cmux-board/internal/state"
)

// mockCmuxCli implements focusCmuxCli for testing.
type mockCmuxCli struct {
	listWorkspaces    func(ctx context.Context) ([]cmuxcli.Workspace, error)
	listPanes         func(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error)
	newWorkspace      func(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error)
	focusPane         func(ctx context.Context, wsRef, paneRef string) error
	focusPaneArgvs    [][]string // records (wsRef, paneRef) pairs for argv assertion
}

func (m *mockCmuxCli) ListWorkspaces(ctx context.Context) ([]cmuxcli.Workspace, error) {
	return m.listWorkspaces(ctx)
}
func (m *mockCmuxCli) ListPanes(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error) {
	return m.listPanes(ctx, wsRef)
}
func (m *mockCmuxCli) NewWorkspaceWithLayout(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error) {
	return m.newWorkspace(ctx, args)
}
func (m *mockCmuxCli) FocusPane(ctx context.Context, wsRef, paneRef string) error {
	m.focusPaneArgvs = append(m.focusPaneArgvs, []string{wsRef, paneRef})
	return m.focusPane(ctx, wsRef, paneRef)
}

// mockClaudeCli implements focusClaudeCli for testing.
type mockClaudeCli struct {
	isOrphan func(ctx context.Context, worktree, shortID string) (bool, error)
}

func (m *mockClaudeCli) IsOrphan(ctx context.Context, worktree, shortID string) (bool, error) {
	return m.isOrphan(ctx, worktree, shortID)
}

// buildFocusStore builds a store seeded with one complete activation.
func buildFocusStore(t *testing.T, activationID string) *state.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	now := time.Now()
	if err := store.Mutate(func(s *state.State) error {
		s.Activations["PROJ-1"] = []state.ActivationEntry{{
			ActivationID:    activationID,
			ActIDShort:      "aabb1122",
			RepoID:          "repo1",
			TicketID:        "PROJ-1",
			ApproachName:    "test",
			WorktreePath:    "/tmp/wt/",
			BranchName:      "test-branch",
			ClaudeName:      "cmux-board:PROJ-1:aabb1122",
			CmuxName:        "PROJ-1 [aabb1122]",
			ClaudeShortID:   "deadbeef",
			CmuxWorkspaceID: "workspace:4",
			AgentPaneRef:    "pane:7",
			Step:            state.StepCmuxCreated,
			Complete:        true,
			CreatedAt:       now,
		}}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return store
}

func TestFocus_HealthyActivation(t *testing.T) {
	activationID := "01JTEST0000000000000000010"
	store := buildFocusStore(t, activationID)
	snap, _ := store.Snapshot()
	entry, _ := state.FindActivationByID(snap, activationID)

	var focusedWsRef, focusedPaneRef string
	cmuxMock := &mockCmuxCli{
		listWorkspaces: func(_ context.Context) ([]cmuxcli.Workspace, error) {
			return []cmuxcli.Workspace{{Ref: "workspace:4", Title: "PROJ-1 [aabb1122]"}}, nil
		},
		listPanes: func(_ context.Context, _ string) ([]cmuxcli.Pane, error) {
			return []cmuxcli.Pane{{Ref: "pane:7", Index: 0}}, nil
		},
		newWorkspace: func(_ context.Context, _ cmuxcli.NewWorkspaceArgs) (string, error) {
			t.Error("NewWorkspaceWithLayout should not be called for healthy activation")
			return "", nil
		},
		focusPane: func(_ context.Context, wsRef, paneRef string) error {
			focusedWsRef = wsRef
			focusedPaneRef = paneRef
			return nil
		},
	}
	claudeMock := &mockClaudeCli{
		isOrphan: func(_ context.Context, _, _ string) (bool, error) { return false, nil },
	}

	result := Focus(context.Background(), store, cmuxMock, claudeMock, "PROJ-1", entry)

	if result.Err != nil {
		t.Fatalf("Focus: %v", result.Err)
	}
	if result.NeedsRespawn {
		t.Error("NeedsRespawn should be false")
	}
	if result.NeedsNewTab {
		t.Error("NeedsNewTab should be false")
	}
	if focusedWsRef != "workspace:4" {
		t.Errorf("FocusPane called with ws=%q, want workspace:4", focusedWsRef)
	}
	if focusedPaneRef != "pane:7" {
		t.Errorf("FocusPane called with pane=%q, want pane:7", focusedPaneRef)
	}

	// LastFocusedAt must be updated in state.
	finalSnap, _ := store.Snapshot()
	finalEntry, _ := state.FindActivationByID(finalSnap, activationID)
	if finalEntry.LastFocusedAt == nil {
		t.Error("LastFocusedAt should be set after focus")
	}
	if finalEntry.ClaudeOrphan {
		t.Error("ClaudeOrphan should be false")
	}
	if finalEntry.CmuxOrphan {
		t.Error("CmuxOrphan should be false")
	}
}

func TestFocus_ClaudeOrphan_ReturnNeedsRespawn(t *testing.T) {
	activationID := "01JTEST0000000000000000011"
	store := buildFocusStore(t, activationID)
	snap, _ := store.Snapshot()
	entry, _ := state.FindActivationByID(snap, activationID)

	focusPaneCalled := false
	cmuxMock := &mockCmuxCli{
		listWorkspaces: func(_ context.Context) ([]cmuxcli.Workspace, error) {
			return []cmuxcli.Workspace{{Ref: "workspace:4"}}, nil
		},
		listPanes: func(_ context.Context, _ string) ([]cmuxcli.Pane, error) {
			return []cmuxcli.Pane{{Ref: "pane:7", Index: 0}}, nil
		},
		newWorkspace: func(_ context.Context, _ cmuxcli.NewWorkspaceArgs) (string, error) {
			return "", nil
		},
		focusPane: func(_ context.Context, _, _ string) error {
			focusPaneCalled = true
			return nil
		},
	}
	claudeMock := &mockClaudeCli{
		isOrphan: func(_ context.Context, _, _ string) (bool, error) {
			return true, nil // session is gone
		},
	}

	result := Focus(context.Background(), store, cmuxMock, claudeMock, "PROJ-1", entry)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if !result.NeedsRespawn {
		t.Error("NeedsRespawn should be true when claude is orphan")
	}
	if focusPaneCalled {
		t.Error("FocusPane should NOT be called when claude_orphan")
	}

	// ClaudeOrphan must be persisted.
	finalSnap, _ := store.Snapshot()
	finalEntry, _ := state.FindActivationByID(finalSnap, activationID)
	if !finalEntry.ClaudeOrphan {
		t.Error("claude_orphan should be true in state")
	}
}

func TestFocus_CmuxOrphan_CreatesNewWorkspace_NoForeignAdoption(t *testing.T) {
	activationID := "01JTEST0000000000000000012"
	store := buildFocusStore(t, activationID)
	snap, _ := store.Snapshot()
	entry, _ := state.FindActivationByID(snap, activationID)

	// A foreign workspace whose current_directory matches the worktree path,
	// but its ref does NOT match the cached CmuxWorkspaceID.
	foreignRef := "workspace:99"
	newWorkspaceCalled := false
	var focusedWsRef string

	cmuxMock := &mockCmuxCli{
		listWorkspaces: func(_ context.Context) ([]cmuxcli.Workspace, error) {
			return []cmuxcli.Workspace{
				{
					Ref:              foreignRef,
					Title:            "some other workspace",
					CurrentDirectory: entry.WorktreePath, // same path, but foreign
				},
				// The cached workspace:4 is absent — triggers cmux_orphan.
			}, nil
		},
		listPanes: func(_ context.Context, wsRef string) ([]cmuxcli.Pane, error) {
			return []cmuxcli.Pane{{Ref: "pane:10", Index: 0}}, nil
		},
		newWorkspace: func(_ context.Context, args cmuxcli.NewWorkspaceArgs) (string, error) {
			newWorkspaceCalled = true
			return "workspace:10", nil
		},
		focusPane: func(_ context.Context, wsRef, _ string) error {
			focusedWsRef = wsRef
			return nil
		},
	}
	claudeMock := &mockClaudeCli{
		isOrphan: func(_ context.Context, _, _ string) (bool, error) { return false, nil },
	}

	result := Focus(context.Background(), store, cmuxMock, claudeMock, "PROJ-1", entry)

	if result.Err != nil {
		t.Fatalf("Focus: %v", result.Err)
	}
	if !result.NeedsNewTab {
		t.Error("NeedsNewTab should be true when cmux_orphan")
	}
	if !newWorkspaceCalled {
		t.Error("NewWorkspaceWithLayout should be called when cmux_orphan")
	}

	// The foreign workspace ref must NEVER be stored in state.
	finalSnap, _ := store.Snapshot()
	finalEntry, _ := state.FindActivationByID(finalSnap, activationID)
	if finalEntry.CmuxWorkspaceID == foreignRef {
		t.Errorf("foreign workspace %q was adopted — forbidden", foreignRef)
	}
	if finalEntry.CmuxWorkspaceID != "workspace:10" {
		t.Errorf("new workspace ref not stored; got %q", finalEntry.CmuxWorkspaceID)
	}
	// FocusPane must be called on the new workspace.
	if focusedWsRef != "workspace:10" {
		t.Errorf("FocusPane called with %q, want workspace:10", focusedWsRef)
	}
}

func TestFocus_FocusPaneFails_ReturnsErr(t *testing.T) {
	activationID := "01JTEST0000000000000000013"
	store := buildFocusStore(t, activationID)
	snap, _ := store.Snapshot()
	entry, _ := state.FindActivationByID(snap, activationID)

	focusErr := errors.New("cmux unreachable")
	cmuxMock := &mockCmuxCli{
		listWorkspaces: func(_ context.Context) ([]cmuxcli.Workspace, error) {
			return []cmuxcli.Workspace{{Ref: "workspace:4"}}, nil
		},
		listPanes: func(_ context.Context, _ string) ([]cmuxcli.Pane, error) {
			return []cmuxcli.Pane{{Ref: "pane:7", Index: 0}}, nil
		},
		newWorkspace: func(_ context.Context, _ cmuxcli.NewWorkspaceArgs) (string, error) {
			return "", nil
		},
		focusPane: func(_ context.Context, _, _ string) error { return focusErr },
	}
	claudeMock := &mockClaudeCli{
		isOrphan: func(_ context.Context, _, _ string) (bool, error) { return false, nil },
	}

	result := Focus(context.Background(), store, cmuxMock, claudeMock, "PROJ-1", entry)

	if result.Err == nil {
		t.Fatal("expected error from FocusPane failure, got nil")
	}
	// Verify error is wrapped and propagated.
	if !errors.Is(result.Err, focusErr) && !focusContainsStr(result.Err.Error(), "focus pane") {
		t.Errorf("error %v does not wrap focusErr or contain 'focus pane'", result.Err)
	}
}

func TestFocus_NegativeArgv_FocusPaneNotBareFocus(t *testing.T) {
	// Verify that the cmuxcli.FocusPane wrapper (called by realCmuxCli) builds argv
	// with "focus-pane" subcommand, not bare "focus". This test exercises the mock
	// interface to assert that Focus() passes the correct wsRef and paneRef to FocusPane.
	activationID := "01JTEST0000000000000000014"
	store := buildFocusStore(t, activationID)
	snap, _ := store.Snapshot()
	entry, _ := state.FindActivationByID(snap, activationID)

	var capturedPaneCall []string
	cmuxMock := &mockCmuxCli{
		listWorkspaces: func(_ context.Context) ([]cmuxcli.Workspace, error) {
			return []cmuxcli.Workspace{{Ref: entry.CmuxWorkspaceID}}, nil
		},
		listPanes: func(_ context.Context, _ string) ([]cmuxcli.Pane, error) {
			return []cmuxcli.Pane{{Ref: entry.AgentPaneRef, Index: 0}}, nil
		},
		newWorkspace: func(_ context.Context, _ cmuxcli.NewWorkspaceArgs) (string, error) {
			return "", nil
		},
		focusPane: func(_ context.Context, wsRef, paneRef string) error {
			capturedPaneCall = []string{wsRef, paneRef}
			return nil
		},
	}
	claudeMock := &mockClaudeCli{
		isOrphan: func(_ context.Context, _, _ string) (bool, error) { return false, nil },
	}

	result := Focus(context.Background(), store, cmuxMock, claudeMock, "PROJ-1", entry)
	if result.Err != nil {
		t.Fatalf("Focus: %v", result.Err)
	}

	// FocusPane must have been called with cached wsRef and paneRef.
	if len(capturedPaneCall) != 2 {
		t.Fatal("FocusPane was not called")
	}
	if capturedPaneCall[0] != entry.CmuxWorkspaceID {
		t.Errorf("FocusPane wsRef got %q, want %q", capturedPaneCall[0], entry.CmuxWorkspaceID)
	}
	if capturedPaneCall[1] != entry.AgentPaneRef {
		t.Errorf("FocusPane paneRef got %q, want %q", capturedPaneCall[1], entry.AgentPaneRef)
	}
	// The real FocusPane uses "focus-pane" subcommand (asserted in cmuxcli/focus_pane_test.go).
	// Here we verify that Focus() passes the correct refs so the real function would build
	// the right argv.
}

func focusContainsStr(s, sub string) bool {
	if len(sub) == 0 || len(s) < len(sub) {
		return len(sub) == 0
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
