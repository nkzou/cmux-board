package jira

import (
	"context"
	"testing"
)

const boardGetJSON = `{
  "id": 42,
  "name": "My Project Board",
  "type": "scrum",
  "location": "My Project (MYPROJ)",
  "link": "https://example.atlassian.net/rest/agile/1.0/board/42"
}`

func TestGetBoardHappyPath(t *testing.T) {
	runner := fakeRunner([]byte(boardGetJSON), nil, 0, nil)
	a := &JiraAdapter{runner: runner}

	board, err := a.GetBoard(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "42" {
		t.Errorf("ID: got %q, want %q", board.ID, "42")
	}
	if board.Name != "My Project Board" {
		t.Errorf("Name: got %q, want %q", board.Name, "My Project Board")
	}
	// Columns are not available via acli board get.
	if board.Columns != nil {
		t.Errorf("Columns: expected nil (not available via acli), got %v", board.Columns)
	}
}

func TestGetBoardArgs(t *testing.T) {
	var capturedArgs []string
	runner := capturingRunner(&capturedArgs, []byte(boardGetJSON), nil, 0, nil)
	a := &JiraAdapter{runner: runner}
	_, _ = a.GetBoard(context.Background(), "42")

	wantArgs := []string{"jira", "board", "get", "--id", "42", "--json"}
	assertArgs(t, capturedArgs, wantArgs)
}

func TestGetBoardAuthError(t *testing.T) {
	runner := fakeRunner(nil,
		[]byte("✗ Error: unauthorized: use 'acli jira auth login' to authenticate"),
		0, nil,
	)
	a := &JiraAdapter{runner: runner}

	_, err := a.GetBoard(context.Background(), "42")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErrAuth(err) {
		t.Errorf("expected errAuth, got %T: %v", err, err)
	}
}

func TestGetBoardNotFound(t *testing.T) {
	// ID 0 in response means board not found.
	runner := fakeRunner([]byte(`{"id":0,"name":"","type":"","location":"","link":""}`), nil, 0, nil)
	a := &JiraAdapter{runner: runner}

	_, err := a.GetBoard(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error for board ID 0, got nil")
	}
}
