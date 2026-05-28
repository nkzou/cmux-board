package initwizard

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// boardMockAdapter exposes a programmable GetBoard for prompt_board tests.
type boardMockAdapter struct {
	known map[string]tracker.Board // boardID → board
	err   error                    // when non-nil, GetBoard always returns this error
}

func (m *boardMockAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, errors.New("not implemented")
}
func (m *boardMockAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	return nil, errors.New("PickBoard must not call ListBoards")
}
func (m *boardMockAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	if m.err != nil {
		return tracker.Board{}, m.err
	}
	b, ok := m.known[boardID]
	if !ok {
		return tracker.Board{}, errors.New("board not found")
	}
	return b, nil
}
func (m *boardMockAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	return nil, errors.New("not implemented")
}
func (m *boardMockAdapter) GetTicket(_ context.Context, _ string) (tracker.Ticket, error) {
	return tracker.Ticket{}, errors.New("not implemented")
}
func (m *boardMockAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	return errors.New("not implemented")
}
func (m *boardMockAdapter) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

func twoBoards() *boardMockAdapter {
	return &boardMockAdapter{known: map[string]tracker.Board{
		"42": {ID: "42", Name: "Alpha"},
		"99": {ID: "99", Name: "Beta"},
	}}
}

func TestPickBoard_InteractiveValidID(t *testing.T) {
	a := twoBoards()
	r := strings.NewReader("42\n")
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, r, a, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "42" || board.Name != "Alpha" {
		t.Errorf("got %+v, want {ID:42, Name:Alpha}", board)
	}
}

func TestPickBoard_InteractiveEmptyThenValid(t *testing.T) {
	a := twoBoards()
	r := strings.NewReader("\n99\n")
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, r, a, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "99" {
		t.Errorf("expected board 99, got %q", board.ID)
	}
	if !strings.Contains(w.String(), "must not be empty") {
		t.Errorf("expected empty-ID warning in output, got: %q", w.String())
	}
}

func TestPickBoard_InteractiveUnknownThenValid(t *testing.T) {
	a := twoBoards()
	r := strings.NewReader("ghost\n42\n")
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, r, a, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "42" {
		t.Errorf("expected board 42, got %q", board.ID)
	}
	if !strings.Contains(w.String(), `board "ghost" not found`) {
		t.Errorf("expected not-found message in output, got: %q", w.String())
	}
}

func TestPickBoard_ExhaustRetries(t *testing.T) {
	a := twoBoards()
	r := strings.NewReader(strings.Repeat("ghost\n", maxBoardPickRetries))
	var w bytes.Buffer
	_, err := PickBoard(context.Background(), &w, r, a, "")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("expected '3 attempts' in error, got: %v", err)
	}
}

func TestPickBoard_FlagOverrideValid(t *testing.T) {
	a := twoBoards()
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, strings.NewReader(""), a, "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "42" || board.Name != "Alpha" {
		t.Errorf("got %+v, want {ID:42, Name:Alpha}", board)
	}
	if w.Len() > 0 {
		t.Errorf("expected no output in flag-override mode, got: %q", w.String())
	}
}

func TestPickBoard_FlagOverrideUnknownID(t *testing.T) {
	a := twoBoards()
	var w bytes.Buffer
	_, err := PickBoard(context.Background(), &w, strings.NewReader(""), a, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown board ID")
	}
	if !strings.Contains(err.Error(), `"unknown"`) {
		t.Errorf("expected board ID in error message, got: %v", err)
	}
}
