package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestListBoardsSinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"startAt": 0, "maxResults": 50, "total": 2, "isLast": true,
			"values": [
				{"id": 1, "name": "Board Alpha", "type": "scrum"},
				{"id": 2, "name": "Board Beta",  "type": "kanban"}
			]
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	boards, err := adapter.ListBoards(context.Background())
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

func TestListBoardsPagination(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if n == 1 {
			w.Write([]byte(`{
				"startAt": 0, "maxResults": 50, "total": 2, "isLast": false,
				"values": [{"id": 1, "name": "Board A", "type": "scrum"}]
			}`))
		} else {
			w.Write([]byte(`{
				"startAt": 1, "maxResults": 50, "total": 2, "isLast": true,
				"values": [{"id": 2, "name": "Board B", "type": "kanban"}]
			}`))
		}
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	boards, err := adapter.ListBoards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("got %d boards, want 2", len(boards))
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected exactly 2 HTTP requests, got %d", callCount)
	}
}

func TestListBoardsTotalGuard(t *testing.T) {
	// isLast is false but startAt + len(values) >= total — should stop after one request.
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"startAt": 0, "maxResults": 50, "total": 1, "isLast": false,
			"values": [{"id": 1, "name": "Only Board", "type": "scrum"}]
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	boards, err := adapter.ListBoards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 1 {
		t.Fatalf("got %d boards, want 1", len(boards))
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected exactly 1 HTTP request (total guard), got %d", callCount)
	}
}

func TestListBoards401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "bad"}
	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.ListBoards(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *errAuth
	if !errors.As(err, &authErr) {
		t.Errorf("expected *errAuth, got %T: %v", err, err)
	}
}
