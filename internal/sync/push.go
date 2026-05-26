package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-zou/cmux-board/internal/state"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// Result is the outcome of a Push call.
type Result struct {
	// Conflict is true when the tracker rejected the transition due to OCC mismatch
	// or an invalid workflow transition.
	Conflict bool

	// ConflictKind distinguishes ErrConflict (OCC) from ErrInvalidTransition (workflow).
	ConflictKind ConflictKind

	// ServerStatus is the status reported by the tracker at the time of conflict.
	// Populated only when Conflict == true. The UI uses this to snap the card back
	// to the correct column without an extra round-trip (T-031).
	// INVARIANT: always non-empty when Conflict == true.
	ServerStatus string
}

// Push executes a column-transition intent for the given ticket.
//
// It reads last_known_status from store.Snapshot() to build expected_from_status and
// calls tracker.TransitionStatus. On success it writes the new status into state via
// store.Mutate. On ErrConflict or ErrInvalidTransition it returns Result{Conflict:true}
// without mutating state (the UI snaps back using Result.ServerStatus).
//
// The targetStatus is a tracker-native status name resolved upstream by the UI.
// On any conflict variant, the returned error is nil — conflict is a normal application
// outcome; only unexpected tracker errors (5xx, network) are returned as non-nil errors.
func Push(
	ctx          context.Context,
	store        *state.Store,
	tr           tracker.IssueTracker,
	ticketID     string,
	targetStatus string, // tracker-native status name resolved upstream by the UI
) (Result, error) {
	snap, _ := store.Snapshot()

	ticket, ok := snap.Tickets[ticketID]
	if !ok {
		return Result{}, fmt.Errorf("push: ticket %q not found in state", ticketID)
	}
	expectedFrom := ticket.LastKnownStatus

	err := tr.TransitionStatus(ctx, ticketID, expectedFrom, targetStatus)
	if err != nil {
		// Check for ConflictError first (richer error with ServerStatus from T-031).
		var ce tracker.ConflictError
		if errors.As(err, &ce) {
			return Result{
				Conflict:     true,
				ConflictKind: ConflictOCC,
				ServerStatus: ce.ServerStatus,
			}, nil
		}

		// Fallback: bare ErrConflict sentinel (no server status available).
		if errors.Is(err, tracker.ErrConflict) {
			return Result{
				Conflict:     true,
				ConflictKind: ConflictOCC,
				ServerStatus: expectedFrom, // snap back to pre-push status
			}, nil
		}

		if errors.Is(err, tracker.ErrInvalidTransition) {
			// Tracker refused the move — card snaps back to where it came from.
			return Result{
				Conflict:     true,
				ConflictKind: ConflictInvalidTransition,
				ServerStatus: expectedFrom, // pre-push status is still the server status
			}, nil
		}

		return Result{}, fmt.Errorf("push: TransitionStatus: %w", err)
	}

	// Success: commit new status through Mutate.
	mutErr := store.Mutate(func(s *state.State) error {
		if t, ok := s.Tickets[ticketID]; ok {
			t.Status = targetStatus
			t.LastKnownStatus = targetStatus
			s.Tickets[ticketID] = t
		}
		return nil
	})
	if mutErr != nil {
		return Result{}, fmt.Errorf("push: store.Mutate: %w", mutErr)
	}

	return Result{}, nil
}
