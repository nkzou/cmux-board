package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBoardHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 42,
			"name": "My Project Board",
			"type": "scrum",
			"columnConfig": {
				"columns": [
					{"name": "To Do",       "statuses": [{"id": "1", "self": "..."}]},
					{"name": "In Progress", "statuses": [{"id": "3", "self": "..."}, {"id": "5", "self": "..."}]},
					{"name": "Done",        "statuses": [{"id": "6", "self": "..."}]}
				]
			}
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	c := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}
	adapter := newJiraAdapterWithClient(c)

	board, err := adapter.GetBoard(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.ID != "42" {
		t.Errorf("board.ID: got %q, want %q", board.ID, "42")
	}
	if board.Name != "My Project Board" {
		t.Errorf("board.Name: got %q, want %q", board.Name, "My Project Board")
	}
	if len(board.Columns) != 3 {
		t.Fatalf("len(columns): got %d, want 3", len(board.Columns))
	}

	// Verify slugified IDs.
	if board.Columns[0].ID != "to-do" {
		t.Errorf("column[0].ID: got %q, want %q", board.Columns[0].ID, "to-do")
	}
	if board.Columns[1].ID != "in-progress" {
		t.Errorf("column[1].ID: got %q, want %q", board.Columns[1].ID, "in-progress")
	}
	if board.Columns[2].ID != "done" {
		t.Errorf("column[2].ID: got %q, want %q", board.Columns[2].ID, "done")
	}

	// Verify StatusIDs.
	if len(board.Columns[1].StatusIDs) != 2 {
		t.Errorf("column[1].StatusIDs: got %v, want 2 entries", board.Columns[1].StatusIDs)
	}
	if board.Columns[1].StatusIDs[0] != "3" || board.Columns[1].StatusIDs[1] != "5" {
		t.Errorf("column[1].StatusIDs: got %v, want [3 5]", board.Columns[1].StatusIDs)
	}
}

func TestGetBoardNoStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 1,
			"name": "Empty Board",
			"columnConfig": {
				"columns": [
					{"name": "No Status Column", "statuses": []}
				]
			}
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	c := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}
	adapter := newJiraAdapterWithClient(c)

	board, err := adapter.GetBoard(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(board.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(board.Columns))
	}
	// StatusIDs should be nil or empty — no panic.
	if len(board.Columns[0].StatusIDs) != 0 {
		t.Errorf("expected empty StatusIDs, got %v", board.Columns[0].StatusIDs)
	}
}

func TestGetBoardSlugCollision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 1,
			"name": "Collision Board",
			"columnConfig": {
				"columns": [
					{"name": "In Progress",  "statuses": [{"id": "3", "self": "..."}]},
					{"name": "In-Progress",  "statuses": [{"id": "5", "self": "..."}]}
				]
			}
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	c := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}
	adapter := newJiraAdapterWithClient(c)

	board, err := adapter.GetBoard(context.Background(), "1")
	if err != nil {
		t.Fatalf("no error expected for slug collision: %v", err)
	}
	// Both columns must be present — adapter does not deduplicate.
	if len(board.Columns) != 2 {
		t.Errorf("expected 2 columns even with slug collision, got %d", len(board.Columns))
	}
}

func TestGetBoard401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "bad"}
	c := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}
	adapter := newJiraAdapterWithClient(c)

	_, err := adapter.GetBoard(context.Background(), "1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *errAuth
	if !errors.As(err, &authErr) {
		t.Errorf("expected *errAuth, got %T: %v", err, err)
	}
}

func TestGetBoard404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	c := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}
	adapter := newJiraAdapterWithClient(c)

	_, err := adapter.GetBoard(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var fatalErr *errFatal
	if !errors.As(err, &fatalErr) {
		t.Errorf("expected *errFatal, got %T: %v", err, err)
	}
}
