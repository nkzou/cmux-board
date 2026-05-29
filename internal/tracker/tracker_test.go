package tracker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// stubAdapter is a compile-time check that the IssueTracker interface can be satisfied.
type stubAdapter struct{}

func (s *stubAdapter) WhoAmI(_ context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, nil
}

func (s *stubAdapter) ListBoards(_ context.Context) ([]tracker.BoardSummary, error) {
	return nil, nil
}

func (s *stubAdapter) GetBoard(_ context.Context, _ string) (tracker.Board, error) {
	return tracker.Board{}, nil
}

func (s *stubAdapter) ListTickets(_ context.Context, _ string, _ *time.Time) ([]tracker.Ticket, error) {
	return nil, nil
}

func (s *stubAdapter) GetTicket(_ context.Context, _ string) (tracker.Ticket, error) {
	return tracker.Ticket{}, nil
}

func (s *stubAdapter) TransitionStatus(_ context.Context, _, _, _ string) error {
	return nil
}

func (s *stubAdapter) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

// compile-time assertion: stubAdapter satisfies IssueTracker.
var _ tracker.IssueTracker = (*stubAdapter)(nil)

func TestSentinelsAreExported(t *testing.T) {
	if !errors.Is(tracker.ErrConflict, tracker.ErrConflict) {
		t.Fatal("ErrConflict not Is-matchable")
	}
	if !errors.Is(tracker.ErrInvalidTransition, tracker.ErrInvalidTransition) {
		t.Fatal("ErrInvalidTransition not Is-matchable")
	}
	if errors.Is(tracker.ErrConflict, tracker.ErrInvalidTransition) {
		t.Fatal("sentinels must be distinct")
	}
}

func TestSentinelErrorMessages(t *testing.T) {
	if tracker.ErrConflict.Error() == "" {
		t.Fatal("ErrConflict must have a non-empty error message")
	}
	if tracker.ErrInvalidTransition.Error() == "" {
		t.Fatal("ErrInvalidTransition must have a non-empty error message")
	}
}

func TestTicketRawIsMapStringAny(t *testing.T) {
	ticket := tracker.Ticket{
		Raw: map[string]any{"key": "value"},
	}
	if ticket.Raw == nil {
		t.Fatal("Ticket.Raw should be assignable as map[string]any")
	}
	if ticket.Raw["key"] != "value" {
		t.Fatalf("got %v, want %v", ticket.Raw["key"], "value")
	}
}
