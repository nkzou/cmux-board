package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

func newGuardTestModel(t *testing.T) Model {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	cfg := &config.Config{
		Repos: map[string]config.RepoEntry{
			"my-repo": {ID: "my-repo", Name: "My Repo", Path: "/tmp/my-repo"},
		},
	}
	m := NewModelWithContext(context.Background(), cfg, store)
	return m
}

func TestTryActivate_FirstCallMarksTicketActivating(t *testing.T) {
	m := newGuardTestModel(t)

	m, cmd := m.tryActivateWithRepo("PROJ-1", "my-repo", "")
	if cmd == nil {
		t.Fatal("first tryActivate returned nil cmd; expected ActivateCmd batch")
	}
	if !m.activatingTickets["PROJ-1"] {
		t.Errorf("activatingTickets[PROJ-1] not set after first tryActivate")
	}
}

func TestTryActivate_SecondCallBlockedWithToast(t *testing.T) {
	m := newGuardTestModel(t)
	m, _ = m.tryActivateWithRepo("PROJ-1", "my-repo", "")

	// Second activation attempt for the same ticket while the first is in flight.
	mAfter, cmd := m.tryActivateWithRepo("PROJ-1", "my-repo", "approach-2")
	if cmd == nil {
		t.Fatal("blocked tryActivate should return a toast tea.Cmd, got nil")
	}
	if len(mAfter.toasts) == 0 {
		t.Fatal("blocked tryActivate should push a toast")
	}
	if !strings.Contains(mAfter.toasts[0].msg, "PROJ-1") {
		t.Errorf("toast should name the ticket; got %q", mAfter.toasts[0].msg)
	}
	if !strings.Contains(mAfter.toasts[0].msg, "already") {
		t.Errorf("toast should say already activating; got %q", mAfter.toasts[0].msg)
	}
}

func TestHandleActivationDone_ClearsTicketAndStopsSpinner(t *testing.T) {
	m := newGuardTestModel(t)
	m, _ = m.tryActivateWithRepo("PROJ-1", "my-repo", "")
	if !m.activatingTickets["PROJ-1"] {
		t.Fatal("precondition failed: PROJ-1 not marked activating")
	}

	mAfter, _ := m.handleActivationDone(activationDoneMsg{
		TicketID:     "PROJ-1",
		ActivationID: "anything",
		Err:          nil,
	})

	if mAfter.activatingTickets["PROJ-1"] {
		t.Errorf("handleActivationDone did not clear PROJ-1 from activatingTickets")
	}
	if mAfter.spinnerFrame != 0 {
		t.Errorf("spinnerFrame should reset to 0 when no activations remain; got %d", mAfter.spinnerFrame)
	}
}

func TestHandleActivationDone_ClearsOnErrorToo(t *testing.T) {
	// A failed activation must still release the ticket — otherwise a single
	// transient error would permanently lock the ticket out of activation.
	m := newGuardTestModel(t)
	m, _ = m.tryActivateWithRepo("PROJ-1", "my-repo", "")

	mAfter, _ := m.handleActivationDone(activationDoneMsg{
		TicketID: "PROJ-1",
		Err:      errSentinel("boom"),
	})

	if mAfter.activatingTickets["PROJ-1"] {
		t.Errorf("activation error must still clear in-flight marker")
	}
}

func TestSpinnerTick_StopsWhenSetEmpty(t *testing.T) {
	m := newGuardTestModel(t)
	// No activations in flight — tick must not re-arm itself.
	mAfter, cmd := m.handleSpinnerTick(spinnerTickMsg{})
	if cmd != nil {
		t.Errorf("handleSpinnerTick should not re-arm when no activations are in flight")
	}
	if mAfter.spinnerFrame != 0 {
		t.Errorf("spinnerFrame should not advance when set is empty")
	}
}

func TestSpinnerTick_AdvancesAndReArmsWhileActivating(t *testing.T) {
	m := newGuardTestModel(t)
	m, _ = m.tryActivateWithRepo("PROJ-1", "my-repo", "")

	mAfter, cmd := m.handleSpinnerTick(spinnerTickMsg{})
	if cmd == nil {
		t.Errorf("handleSpinnerTick should re-arm tick while activation is in flight")
	}
	if mAfter.spinnerFrame == 0 {
		t.Errorf("spinnerFrame should advance during in-flight activation")
	}
}

func TestSpinnerGlyph_NonEmpty(t *testing.T) {
	m := newGuardTestModel(t)
	if m.spinnerGlyph() == "" {
		t.Error("spinnerGlyph must always return a renderable frame")
	}
}

// TestHandleActivationDone_RefreshesSnapshot pins the badge-update fix.
// The activation goroutine writes the new ActivationEntry directly to the
// store via Mutate; without an explicit refreshSnapshot in
// handleActivationDone the Model's cached snapshot would not see the new
// entry and the worktree badge would be missing until the next poll.
func TestHandleActivationDone_RefreshesSnapshot(t *testing.T) {
	m := newGuardTestModel(t)
	m, _ = m.tryActivateWithRepo("PROJ-1", "my-repo", "")

	// Simulate the Activate goroutine journaling a new entry into the store
	// while the Model's snapshot still points at the pre-activation state.
	if err := m.store.Mutate(func(s *state.State) error {
		if s.Activations == nil {
			s.Activations = make(map[string][]state.ActivationEntry)
		}
		s.Activations["PROJ-1"] = append(s.Activations["PROJ-1"], state.ActivationEntry{
			ActivationID: "act-1",
			TicketID:     "PROJ-1",
			RepoID:       "my-repo",
		})
		return nil
	}); err != nil {
		t.Fatalf("seed activation: %v", err)
	}

	// Sanity check: stale snapshot has no activation yet.
	if state.ActivationCount(m.snapshot, "PROJ-1") != 0 {
		t.Fatalf("precondition: model snapshot should be stale (0), got %d",
			state.ActivationCount(m.snapshot, "PROJ-1"))
	}

	mAfter, _ := m.handleActivationDone(activationDoneMsg{
		TicketID:     "PROJ-1",
		ActivationID: "act-1",
	})

	if state.ActivationCount(mAfter.snapshot, "PROJ-1") != 1 {
		t.Errorf("handleActivationDone did not refresh snapshot; ActivationCount(PROJ-1) = %d, want 1",
			state.ActivationCount(mAfter.snapshot, "PROJ-1"))
	}
}

// errSentinel is a minimal stand-in error for activation-failure tests so the
// helper does not depend on production error sentinels.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
