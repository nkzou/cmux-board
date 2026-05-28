package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/claudecli"
	"github.com/nkzou/cmux-board/internal/cmuxcli"
	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

type mockRespawnClaudeCli struct {
	respawn func(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error)
}

func (m *mockRespawnClaudeCli) Respawn(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error) {
	return m.respawn(ctx, args)
}

func TestRespawnActivation_ReplacesClaudeAndCmuxRefs(t *testing.T) {
	activationID := "01JTEST0000000000000000020"
	store := buildFocusStore(t, activationID)
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-1"] = state.TicketState{
			Key:           "PROJ-1",
			ID:            "10001",
			Source:        "jira",
			Summary:       "Fix respawn",
			Status:        "In Progress",
			IssueType:     "Bug",
			Priority:      "High",
			AssigneeEmail: "dev@example.com",
		}
		act := findActivationMutable(s, activationID)
		act.ClaudeOrphan = true
		act.CmuxOrphan = true
		return nil
	}); err != nil {
		t.Fatalf("seed ticket/orphans: %v", err)
	}

	cfg := &config.Config{
		SchemaVersion: config.SchemaVersionCurrent,
		Repos: map[string]config.RepoEntry{
			"repo1": {ID: "repo1", Name: "Repo One", Path: "/repo", DefaultBranch: "main"},
		},
		Claude: config.ClaudeConfig{StarterPrompt: "work on {{.Ticket.Key}} {{.Ticket.IssueType}} {{.Ticket.Priority}} {{.Ticket.AssigneeEmail}} in {{.Repo.ID}}"},
	}

	var capturedBG claudecli.BGArgs
	claudeMock := &mockRespawnClaudeCli{
		respawn: func(_ context.Context, args claudecli.BGArgs) (claudecli.BGResult, error) {
			capturedBG = args
			return claudecli.BGResult{ShortID: "feedbeef", Name: args.Name}, nil
		},
	}

	var capturedWorkspace cmuxcli.NewWorkspaceArgs
	var focusedWsRef, focusedPaneRef string
	cmuxMock := &mockCmuxCli{
		listPanes: func(_ context.Context, wsRef string) ([]cmuxcli.Pane, error) {
			if wsRef != "workspace:9" {
				t.Errorf("ListPanes wsRef = %q, want workspace:9", wsRef)
			}
			return []cmuxcli.Pane{{Ref: "pane:11", Index: 0}}, nil
		},
		newWorkspace: func(_ context.Context, args cmuxcli.NewWorkspaceArgs) (string, error) {
			capturedWorkspace = args
			return "workspace:9", nil
		},
		focusPane: func(_ context.Context, wsRef, paneRef string) error {
			focusedWsRef = wsRef
			focusedPaneRef = paneRef
			return nil
		},
	}

	result := RespawnActivation(context.Background(), store, cfg, cmuxMock, claudeMock, activationID)
	if result.Err != nil {
		t.Fatalf("RespawnActivation: %v", result.Err)
	}

	if capturedBG.Worktree != "/tmp/wt" {
		t.Errorf("BGArgs.Worktree = %q, want /tmp/wt", capturedBG.Worktree)
	}
	if capturedBG.Name != "cmux-board:PROJ-1:aabb1122" {
		t.Errorf("BGArgs.Name = %q, want existing claude name", capturedBG.Name)
	}
	if capturedBG.Prompt != "work on PROJ-1 Bug High dev@example.com in repo1" {
		t.Errorf("BGArgs.Prompt = %q, want rendered prompt", capturedBG.Prompt)
	}
	if capturedWorkspace.CWD != "/tmp/wt" {
		t.Errorf("workspace CWD = %q, want /tmp/wt", capturedWorkspace.CWD)
	}
	if !strings.Contains(capturedWorkspace.AgentAttachCommand, "feedbeef") {
		t.Errorf("workspace attach command = %q, want new short id", capturedWorkspace.AgentAttachCommand)
	}
	if focusedWsRef != "workspace:9" || focusedPaneRef != "pane:11" {
		t.Errorf("FocusPane called with (%q, %q), want (workspace:9, pane:11)", focusedWsRef, focusedPaneRef)
	}

	snap, _ := store.Snapshot()
	entry, _ := state.FindActivationByID(snap, activationID)
	if entry.ClaudeShortID != "feedbeef" {
		t.Errorf("ClaudeShortID = %q, want feedbeef", entry.ClaudeShortID)
	}
	if entry.ClaudeOrphan {
		t.Error("ClaudeOrphan should be cleared")
	}
	if entry.CmuxOrphan {
		t.Error("CmuxOrphan should be cleared")
	}
	if entry.CmuxWorkspaceID != "workspace:9" {
		t.Errorf("CmuxWorkspaceID = %q, want workspace:9", entry.CmuxWorkspaceID)
	}
	if entry.AgentPaneRef != "pane:11" {
		t.Errorf("AgentPaneRef = %q, want pane:11", entry.AgentPaneRef)
	}
	if entry.Step != state.StepCmuxCreated || !entry.Complete {
		t.Errorf("step/complete = (%q, %v), want (%q, true)", entry.Step, entry.Complete, state.StepCmuxCreated)
	}
	if entry.LastFocusedAt == nil {
		t.Error("LastFocusedAt should be set after focusing respawned pane")
	}
}

func TestHandlePickerMode_RespawnDispatchesCommand(t *testing.T) {
	t.Parallel()
	entry := makeEntry("PROJ-1", "repo-a", "01AAAAAAAAAAAAAAAAAAAAAA", state.StepCmuxCreated, true)
	entry.ClaudeOrphan = true
	m := makePickerTestModel(t,
		[]state.ActivationEntry{entry},
		map[string]config.RepoEntry{"repo-a": {ID: "repo-a", Name: "Repo A"}},
	)
	snap, _ := m.store.Snapshot()
	m.snapshot = snap
	m.mode = ModePicker
	m.pickerState = forcePickerState(snap, "PROJ-1", "repo-a")

	next, cmd := m.handlePickerMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyPickerRespawn)})
	if cmd == nil {
		t.Fatal("respawn key should dispatch a command")
	}
	if !next.activatingTickets["PROJ-1"] {
		t.Error("respawn should mark the ticket in-flight")
	}
	if next.mode != ModePicker {
		t.Errorf("mode = %v, want ModePicker", next.mode)
	}
}
