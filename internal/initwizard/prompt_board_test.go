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

// boardMockAdapter wraps mockAdapter with a custom ListBoards.
type boardMockAdapter struct {
	boards    []tracker.BoardSummary
	listErr   error
}

func (m *boardMockAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	return tracker.UserIdentity{}, errors.New("not implemented")
}
func (m *boardMockAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	return m.boards, m.listErr
}
func (m *boardMockAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	return tracker.Board{}, errors.New("not implemented")
}
func (m *boardMockAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	return nil, errors.New("not implemented")
}
func (m *boardMockAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	return errors.New("not implemented")
}
func (m *boardMockAdapter) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

func threeBoards() []tracker.BoardSummary {
	return []tracker.BoardSummary{
		{ID: "b1", Name: "Alpha"},
		{ID: "b2", Name: "Beta"},
		{ID: "b3", Name: "Gamma"},
	}
}

func TestPickBoard_InteractiveSingleBoard(t *testing.T) {
	a := &boardMockAdapter{boards: []tracker.BoardSummary{{ID: "b1", Name: "Alpha"}}}
	r := strings.NewReader("1\n")
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, r, a, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "b1" {
		t.Errorf("expected board ID %q, got %q", "b1", board.ID)
	}
}

func TestPickBoard_InteractivePickSecond(t *testing.T) {
	a := &boardMockAdapter{boards: threeBoards()}
	r := strings.NewReader("2\n")
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, r, a, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "b2" {
		t.Errorf("expected board ID %q, got %q", "b2", board.ID)
	}
}

func TestPickBoard_InteractiveInvalidThenValid(t *testing.T) {
	a := &boardMockAdapter{boards: threeBoards()}
	r := strings.NewReader("abc\n1\n")
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, r, a, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "b1" {
		t.Errorf("expected board ID %q, got %q", "b1", board.ID)
	}
}

func TestPickBoard_InteractiveExhaustRetries(t *testing.T) {
	a := &boardMockAdapter{boards: threeBoards()}
	// Provide maxBoardPickRetries invalid inputs
	inputs := strings.Repeat("abc\n", maxBoardPickRetries)
	r := strings.NewReader(inputs)
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
	a := &boardMockAdapter{boards: threeBoards()}
	var w bytes.Buffer
	board, err := PickBoard(context.Background(), &w, strings.NewReader(""), a, "b2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "b2" {
		t.Errorf("expected board ID %q, got %q", "b2", board.ID)
	}
	// No output in flag-override mode
	if w.Len() > 0 {
		t.Errorf("expected no output in flag-override mode, got: %q", w.String())
	}
}

func TestPickBoard_FlagOverrideUnknownID(t *testing.T) {
	a := &boardMockAdapter{boards: threeBoards()}
	var w bytes.Buffer
	_, err := PickBoard(context.Background(), &w, strings.NewReader(""), a, "unknown-id")
	if err == nil {
		t.Fatal("expected error for unknown board ID")
	}
	if !strings.Contains(err.Error(), "unknown-id") {
		t.Errorf("expected board ID in error message, got: %v", err)
	}
}

func TestPickBoard_EmptyBoardList(t *testing.T) {
	a := &boardMockAdapter{boards: nil}
	var w bytes.Buffer
	_, err := PickBoard(context.Background(), &w, strings.NewReader(""), a, "")
	if err == nil {
		t.Fatal("expected error for empty board list")
	}
	if !strings.Contains(err.Error(), "no boards found") {
		t.Errorf("expected 'no boards found' in error, got: %v", err)
	}
}
