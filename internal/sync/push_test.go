package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// mockTracker is a minimal IssueTracker mock for push tests.
type mockTracker struct {
	transitionErr    error
	transitionCalled bool // dry-run guard: tests assert this stays false
}

func (m *mockTracker) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}
func (m *mockTracker) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}
func (m *mockTracker) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{}, nil
}
func (m *mockTracker) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	return nil, nil
}
func (m *mockTracker) TransitionStatus(_ context.Context, _, _, _ string) error {
	m.transitionCalled = true
	return m.transitionErr
}
func (m *mockTracker) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

// seedStore creates a store with PROJ-42 at the given lastKnownStatus.
func seedStore(t *testing.T, status string) *state.Store {
	t.Helper()
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("seedStore: %v", err)
	}
	_ = store.Mutate(func(s *state.State) error {
		s.Tickets["PROJ-42"] = state.TicketState{
			Key:    "PROJ-42",
			Status: status,
		}
		return nil
	})
	return store
}

// TC-1: happy path — success writes new status into state.
func TestPush_HappyPath(t *testing.T) {
	store := seedStore(t, "In Progress")
	tr := &mockTracker{transitionErr: nil}

	res, err := Push(context.Background(), store, tr, "PROJ-42", "Done", false)
	if err != nil {
		t.Fatalf("TC-1: unexpected error: %v", err)
	}
	if res.Conflict {
		t.Error("TC-1: Conflict should be false on success")
	}
	if res.DryRun {
		t.Error("TC-1: DryRun should be false")
	}

	snap, _ := store.Snapshot()
	got := snap.Tickets["PROJ-42"]
	if got.Status != "Done" {
		t.Errorf("TC-1: Status: want %q, got %q", "Done", got.Status)
	}
}

// TC-2: ErrConflict with ConflictError wrapper → Result.ServerStatus == wrapper's value.
func TestPush_ErrConflictWithWrapper(t *testing.T) {
	store := seedStore(t, "In Progress")
	tr := &mockTracker{transitionErr: tracker.ConflictError{ServerStatus: "Review"}}

	res, err := Push(context.Background(), store, tr, "PROJ-42", "Done", false)
	if err != nil {
		t.Fatalf("TC-2: unexpected error: %v", err)
	}
	if !res.Conflict {
		t.Error("TC-2: Conflict should be true")
	}
	if res.ConflictKind != ConflictOCC {
		t.Errorf("TC-2: ConflictKind: want ConflictOCC, got %v", res.ConflictKind)
	}
	if res.ServerStatus != "Review" {
		t.Errorf("TC-2: ServerStatus: want %q, got %q", "Review", res.ServerStatus)
	}

	// State must be unchanged.
	snap, _ := store.Snapshot()
	if snap.Tickets["PROJ-42"].Status != "In Progress" {
		t.Error("TC-2: state must not change on conflict")
	}
}

// TC-2b: bare ErrConflict (no wrapper) → ServerStatus == expectedFrom (lastKnownStatus).
func TestPush_ErrConflictBare(t *testing.T) {
	store := seedStore(t, "In Progress")
	tr := &mockTracker{transitionErr: tracker.ErrConflict}

	res, err := Push(context.Background(), store, tr, "PROJ-42", "Done", false)
	if err != nil {
		t.Fatalf("TC-2b: unexpected error: %v", err)
	}
	if !res.Conflict {
		t.Error("TC-2b: Conflict should be true")
	}
	if res.ServerStatus != "In Progress" {
		t.Errorf("TC-2b: ServerStatus: want %q (expectedFrom), got %q", "In Progress", res.ServerStatus)
	}
}

// TC-3: ErrInvalidTransition → no state mutation, ConflictInvalidTransition (RT-3).
func TestPush_ErrInvalidTransition(t *testing.T) {
	store := seedStore(t, "In Progress")
	tr := &mockTracker{transitionErr: tracker.ErrInvalidTransition}

	res, err := Push(context.Background(), store, tr, "PROJ-42", "Done", false)
	if err != nil {
		t.Fatalf("TC-3: unexpected error: %v", err)
	}
	if !res.Conflict {
		t.Error("TC-3: Conflict should be true")
	}
	if res.ConflictKind != ConflictInvalidTransition {
		t.Errorf("TC-3: ConflictKind: want ConflictInvalidTransition, got %v", res.ConflictKind)
	}
	// ServerStatus for invalid transition must equal expectedFrom (the pre-push status).
	if res.ServerStatus != "In Progress" {
		t.Errorf("TC-3: ServerStatus: want %q (expectedFrom), got %q", "In Progress", res.ServerStatus)
	}
	// State unchanged.
	snap, _ := store.Snapshot()
	if snap.Tickets["PROJ-42"].Status != "In Progress" {
		t.Error("TC-3: state must not change on ErrInvalidTransition")
	}
}

// TC-4: unknown ticket → non-nil error, no state mutation.
func TestPush_UnknownTicket(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-4: %v", err)
	}
	tr := &mockTracker{}

	res, pushErr := Push(context.Background(), store, tr, "PROJ-GHOST", "Done", false)
	if pushErr == nil {
		t.Error("TC-4: want non-nil error for unknown ticket")
	}
	if res.Conflict {
		t.Error("TC-4: Conflict should be false for unknown ticket")
	}
}

// TC-5 (T-031): ConflictError unwraps to ErrConflict.
func TestConflictError_Unwraps(t *testing.T) {
	err := tracker.ConflictError{ServerStatus: "Done"}
	if !errors.Is(err, tracker.ErrConflict) {
		t.Error("TC-5: ConflictError must unwrap to ErrConflict via errors.Is")
	}
}

// TC-DR1 (dry-run): Push with dryRun=true MUST NOT call TransitionStatus.
func TestPush_DryRun_NoTrackerCall(t *testing.T) {
	store := seedStore(t, "In Progress")
	// Set transitionErr to a sentinel so any accidental call would surface clearly.
	tr := &mockTracker{transitionErr: errors.New("DRY-RUN VIOLATION: tracker called")}

	res, err := Push(context.Background(), store, tr, "PROJ-42", "Done", true)
	if err != nil {
		t.Fatalf("TC-DR1: unexpected error: %v", err)
	}
	if tr.transitionCalled {
		t.Error("TC-DR1: TransitionStatus MUST NOT be called in dry-run mode")
	}
	if !res.DryRun {
		t.Error("TC-DR1: Result.DryRun must be true")
	}
	if res.Conflict {
		t.Error("TC-DR1: Result.Conflict must be false in dry-run")
	}
}

// TC-DR2 (dry-run): Push with dryRun=true MUST NOT mutate state.
func TestPush_DryRun_NoStateMutation(t *testing.T) {
	store := seedStore(t, "In Progress")
	tr := &mockTracker{}

	_, err := Push(context.Background(), store, tr, "PROJ-42", "Done", true)
	if err != nil {
		t.Fatalf("TC-DR2: unexpected error: %v", err)
	}

	snap, _ := store.Snapshot()
	got := snap.Tickets["PROJ-42"]
	if got.Status != "In Progress" {
		t.Errorf("TC-DR2: Status: want %q (unchanged), got %q", "In Progress", got.Status)
	}
}

// TC-DR3 (dry-run): Result.ServerStatus equals expectedFrom (lastKnownStatus).
// This is the snap-back invariant: UI must always have a non-empty ServerStatus
// when the result is non-success.
func TestPush_DryRun_ServerStatusEqualsExpectedFrom(t *testing.T) {
	store := seedStore(t, "In Progress")
	tr := &mockTracker{}

	res, err := Push(context.Background(), store, tr, "PROJ-42", "Done", true)
	if err != nil {
		t.Fatalf("TC-DR3: unexpected error: %v", err)
	}
	if res.ServerStatus != "In Progress" {
		t.Errorf("TC-DR3: ServerStatus: want %q (lastKnownStatus), got %q", "In Progress", res.ServerStatus)
	}
}

// TC-DR4 (dry-run): unknown ticket in dry-run still returns an error (not silently swallowed).
// Dry-run mode does not change the contract that an unknown ticket is a caller bug.
func TestPush_DryRun_UnknownTicketStillErrors(t *testing.T) {
	store, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatalf("TC-DR4: %v", err)
	}
	tr := &mockTracker{}

	res, pushErr := Push(context.Background(), store, tr, "PROJ-GHOST", "Done", true)
	if pushErr == nil {
		t.Error("TC-DR4: want non-nil error for unknown ticket even in dry-run")
	}
	if tr.transitionCalled {
		t.Error("TC-DR4: TransitionStatus must not be called for unknown ticket in dry-run")
	}
	if res.DryRun {
		t.Error("TC-DR4: DryRun should not be set when ticket lookup fails")
	}
}
