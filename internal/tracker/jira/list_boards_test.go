package jira

import (
	"context"
	"errors"
	"testing"
)

const boardSearchJSON = `{
  "isLast": true,
  "maxResults": 50,
  "startAt": 0,
  "total": 2,
  "values": [
    {"id": 1, "name": "Board Alpha", "type": "scrum", "location": "Alpha Project (ALPHA)"},
    {"id": 2, "name": "Board Beta",  "type": "kanban", "location": "Beta Project (BETA)"}
  ]
}`

func TestListBoardsHappyPath(t *testing.T) {
	runner := fakeRunner([]byte(boardSearchJSON), nil, 0, nil)
	a := &JiraAdapter{runner: runner}

	boards, err := a.ListBoards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("got %d boards, want 2", len(boards))
	}
	if boards[0].ID != "1" || boards[0].Name != "Board Alpha" || boards[0].Type != "scrum" {
		t.Errorf("board[0]: got %+v", boards[0])
	}
	if boards[1].ID != "2" || boards[1].Name != "Board Beta" || boards[1].Type != "kanban" {
		t.Errorf("board[1]: got %+v", boards[1])
	}
}

func TestListBoardsAuthError(t *testing.T) {
	runner := fakeRunner(nil,
		[]byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate"),
		0, nil,
	)
	a := &JiraAdapter{runner: runner}

	_, err := a.ListBoards(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ae *errAuth
	if !isErrAuth(err) {
		t.Errorf("expected errAuth, got %T: %v", err, err)
	}
	_ = ae
}

func TestListBoardsArgs(t *testing.T) {
	var capturedArgs []string
	runner := capturingRunner(&capturedArgs, []byte(boardSearchJSON), nil, 0, nil)
	a := &JiraAdapter{runner: runner}
	_, _ = a.ListBoards(context.Background())

	wantArgs := []string{"jira", "board", "search", "--json", "--paginate"}
	assertArgs(t, capturedArgs, wantArgs)
}

func TestListBoardsEmptyResult(t *testing.T) {
	runner := fakeRunner([]byte(`{"isLast":true,"maxResults":50,"startAt":0,"total":0,"values":[]}`), nil, 0, nil)
	a := &JiraAdapter{runner: runner}

	boards, err := a.ListBoards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 0 {
		t.Errorf("expected empty boards, got %d", len(boards))
	}
}

// isErrAuth is a helper for test assertions.
func isErrAuth(err error) bool {
	var ae *errAuth
	return errors.As(err, &ae)
}

// assertArgs compares captured args to wanted args.
func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("args len: got %d (%v), want %d (%v)", len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
