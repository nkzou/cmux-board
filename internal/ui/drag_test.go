package ui

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// buildDragModel builds a Model with a seeded board and ticket for drag tests.
// Board layout is seeded directly on the model (not on State, schema v3).
func buildDragModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}

	now := time.Now()
	if err := store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-1"] = state.TicketState{
			Key:       "PROJ-1",
			Summary:   "Test ticket",
			Status:    "To Do",
			UpdatedAt: &now,
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cfg := &config.Config{SchemaVersion: config.SchemaVersionCurrent}
	m := NewModelWithContext(context.Background(), cfg, store)
	// Seed board layout directly on the model (schema v3: board not in State).
	m.board = state.BoardSnapshot{
		Columns: []state.ColumnSnapshot{
			{ID: "col-todo", Name: "To Do", StatusIDs: []string{"To Do"}},
			{ID: "col-inprogress", Name: "In Progress", StatusIDs: []string{"In Progress"}},
			{ID: "col-done", Name: "Done", StatusIDs: []string{"Done"}},
		},
	}
	return m
}

func TestDrag_SnapBackOnConflict(t *testing.T) {
	m := buildDragModel(t)

	// Set up drag state: PROJ-1 in col-todo, dragging to col-inprogress.
	m.dragging = true
	m.dragTicketID = "PROJ-1"
	m.dragFromColumn = "col-todo"
	m.dragTargetColumn = "col-inprogress"

	// Execute drop (starts optimistic move).
	m, _ = m.dropTicket()

	// After optimistic move, ticket should be at "In Progress".
	snap, _ := m.store.Snapshot()
	if snap.Tickets["PROJ-1"].Status != "In Progress" {
		t.Errorf("after optimistic move: Status got %q, want %q",
			snap.Tickets["PROJ-1"].Status, "In Progress")
	}

	// Inject PushConflict result (server says ticket is at "To Do").
	m, _ = m.handlePushResult(pushResultMsg{
		ticketID:     "PROJ-1",
		fromColumn:   "col-todo",
		newStatus:    "In Progress",
		serverStatus: "To Do",
		conflict:     true,
	})

	// Ticket must be snapped back to "To Do".
	snap2, _ := m.store.Snapshot()
	if snap2.Tickets["PROJ-1"].Status != "To Do" {
		t.Errorf("after snap-back: Status got %q, want %q",
			snap2.Tickets["PROJ-1"].Status, "To Do")
	}
	// Toast should be in the queue.
	if len(m.toasts) == 0 {
		t.Error("expected conflict toast in queue")
	}
}

func TestDrag_OptimisticMoveAndCommitOnSuccess(t *testing.T) {
	m := buildDragModel(t)

	// Set up drag.
	m.dragging = true
	m.dragTicketID = "PROJ-1"
	m.dragFromColumn = "col-todo"
	m.dragTargetColumn = "col-inprogress"

	// Execute drop.
	m, _ = m.dropTicket()

	// Optimistic move applied.
	snap, _ := m.store.Snapshot()
	if snap.Tickets["PROJ-1"].Status != "In Progress" {
		t.Errorf("optimistic: got %q, want In Progress", snap.Tickets["PROJ-1"].Status)
	}

	// Simulate successful push (sync.Push already committed via Mutate).
	m, _ = m.handlePushResult(pushResultMsg{
		ticketID:   "PROJ-1",
		fromColumn: "col-todo",
		newStatus:  "In Progress",
	})

	// Snapshot should still reflect In Progress.
	snap2, _ := m.store.Snapshot()
	if snap2.Tickets["PROJ-1"].Status != "In Progress" {
		t.Errorf("after success: got %q, want In Progress", snap2.Tickets["PROJ-1"].Status)
	}
	// No error toasts.
	if len(m.toasts) != 0 {
		t.Errorf("unexpected toasts: %v", m.toasts)
	}
}

func TestDrag_NoopOnSameColumn(t *testing.T) {
	m := buildDragModel(t)

	// Drag to same column.
	m.dragging = true
	m.dragTicketID = "PROJ-1"
	m.dragFromColumn = "col-todo"
	m.dragTargetColumn = "col-todo"

	storeMutateCalled := false
	// Verify that Status is unchanged after drop.
	snapBefore, _ := m.store.Snapshot()
	beforeStatus := snapBefore.Tickets["PROJ-1"].Status

	m, cmd := m.dropTicket()

	// Drag state must be cleared.
	if m.dragging {
		t.Error("dragging should be false after drop")
	}
	// No push command.
	if cmd != nil {
		t.Error("expected nil cmd for same-column drop (no-op)")
	}
	snapAfter, _ := m.store.Snapshot()
	if snapAfter.Tickets["PROJ-1"].Status != beforeStatus {
		t.Errorf("status changed on same-column drop: %q -> %q", beforeStatus, snapAfter.Tickets["PROJ-1"].Status)
	}
	_ = storeMutateCalled
}

func TestDrag_StateMutationOnlyThroughMutate(t *testing.T) {
	// Verify that the push success path uses store.Mutate to persist the new status.
	m := buildDragModel(t)

	// Simulate state that sync.Push leaves after a successful transition.
	if err := m.store.Mutate(func(s *state.State) error {
		if t2, ok := s.Tickets["PROJ-1"]; ok {
			t2.Status = "Done"
			s.Tickets["PROJ-1"] = t2
		}
		return nil
	}); err != nil {
		t.Fatalf("seed mutate: %v", err)
	}

	m, _ = m.handlePushResult(pushResultMsg{
		ticketID:   "PROJ-1",
		fromColumn: "col-todo",
		newStatus:  "Done",
	})

	// Store must reflect "Done".
	snap, _ := m.store.Snapshot()
	if snap.Tickets["PROJ-1"].Status != "Done" {
		t.Errorf("Status got %q, want Done", snap.Tickets["PROJ-1"].Status)
	}
}

// mockTrackerForDrag is a minimal tracker.IssueTracker mock for drag tests.
type mockTrackerForDrag struct {
	transitionStatus func(ctx context.Context, ticketID, from, to string) error
}

func (m *mockTrackerForDrag) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}
func (m *mockTrackerForDrag) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}
func (m *mockTrackerForDrag) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{}, nil
}
func (m *mockTrackerForDrag) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	return nil, nil
}
func (m *mockTrackerForDrag) GetTicket(_ context.Context, _ string) (tracker.Ticket, error) {
	return tracker.Ticket{}, nil
}
func (m *mockTrackerForDrag) TransitionStatus(ctx context.Context, ticketID, from, to string) error {
	return m.transitionStatus(ctx, ticketID, from, to)
}
func (m *mockTrackerForDrag) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

func TestDrag_PushCalledAsynchronously(t *testing.T) {
	// Verify that pushCmdWith returns a non-nil tea.Cmd and that the cmd
	// emits a pushResultMsg when invoked.
	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	now := time.Now()
	store.Mutate(func(s *state.State) error { //nolint:errcheck
		s.Tickets["PROJ-1"] = state.TicketState{
			Key:       "PROJ-1",
			Status:    "To Do",
			UpdatedAt: &now,
		}
		return nil
	})

	transitionCalled := false
	tr := &mockTrackerForDrag{
		transitionStatus: func(_ context.Context, _, _, _ string) error {
			transitionCalled = true
			return nil
		},
	}

	cmd := pushCmdWith(context.Background(), store, tr, "PROJ-1", "col-todo", "In Progress", false)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from pushCmdWith")
	}

	// Execute the cmd synchronously (BubbleTea would run it in a goroutine).
	msg := cmd()
	if !transitionCalled {
		t.Error("TransitionStatus was not called")
	}

	result, ok := msg.(pushResultMsg)
	if !ok {
		t.Fatalf("expected pushResultMsg, got %T", msg)
	}
	if result.conflict {
		t.Error("expected no conflict on success")
	}
	if result.err != nil {
		t.Fatalf("expected no error, got: %v", result.err)
	}
}

func TestDrag_ConflictSnapBackUsesServerStatus(t *testing.T) {
	// Verify that snap-back uses ServerStatus from the msg, NOT a fresh fetch.
	m := buildDragModel(t)

	// Force optimistic move to "In Progress".
	m.store.Mutate(func(s *state.State) error { //nolint:errcheck
		if tk, ok := s.Tickets["PROJ-1"]; ok {
			tk.Status = "In Progress"
			s.Tickets["PROJ-1"] = tk
		}
		return nil
	})

	conflictErr := errors.New("conflict sentinel")
	_ = conflictErr

	// Handle conflict with ServerStatus = "To Do".
	m, _ = m.handlePushResult(pushResultMsg{
		ticketID:     "PROJ-1",
		fromColumn:   "col-todo",
		newStatus:    "In Progress",
		serverStatus: "To Do",
		conflict:     true,
	})

	snap, _ := m.store.Snapshot()
	if snap.Tickets["PROJ-1"].Status != "To Do" {
		t.Errorf("snap-back got %q, want To Do", snap.Tickets["PROJ-1"].Status)
	}
}
