package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestListTicketsHappyPath(t *testing.T) {
	site := "test.atlassian.net"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"startAt": 0, "maxResults": 50, "total": 1,
			"issues": [{
				"id": "10042",
				"key": "PROJ-42",
				"self": "https://test.atlassian.net/rest/api/3/issue/10042",
				"fields": {
					"summary": "Fix the bug",
					"status": {"id": "3", "name": "In Progress"},
					"assignee": {"accountId": "acc-1"},
					"labels": ["backend", "urgent"],
					"priority": {"name": "High"},
					"updated": "2026-05-26T12:00:00-0000"
				}
			}]
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: site, Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	tickets, err := adapter.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(tickets))
	}
	tk := tickets[0]
	if tk.ID != "10042" {
		t.Errorf("ID: got %q, want %q", tk.ID, "10042")
	}
	if tk.Key != "PROJ-42" {
		t.Errorf("Key: got %q, want %q", tk.Key, "PROJ-42")
	}
	if tk.Summary != "Fix the bug" {
		t.Errorf("Summary: got %q, want %q", tk.Summary, "Fix the bug")
	}
	if tk.Status != "In Progress" {
		t.Errorf("Status: got %q, want %q", tk.Status, "In Progress")
	}
	// URL must be synthesized from site + key, NOT issue.self.
	wantURL := "https://" + site + "/browse/PROJ-42"
	if tk.URL != wantURL {
		t.Errorf("URL: got %q, want %q", tk.URL, wantURL)
	}
	if tk.AssigneeID != "acc-1" {
		t.Errorf("AssigneeID: got %q, want %q", tk.AssigneeID, "acc-1")
	}
	if len(tk.Labels) != 2 || tk.Labels[0] != "backend" {
		t.Errorf("Labels: got %v", tk.Labels)
	}
	if tk.Priority != "High" {
		t.Errorf("Priority: got %q, want %q", tk.Priority, "High")
	}
}

func TestListTicketsNullableFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"startAt": 0, "maxResults": 50, "total": 1,
			"issues": [{
				"id": "1", "key": "PROJ-1", "self": "...",
				"fields": {
					"summary": "No assignee or priority",
					"status": {"id": "1", "name": "To Do"},
					"assignee": null,
					"labels": [],
					"priority": null,
					"updated": "2026-05-01T00:00:00-0000"
				}
			}]
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	tickets, err := adapter.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(tickets))
	}
	if tickets[0].AssigneeID != "" {
		t.Errorf("AssigneeID: got %q, want empty string for null assignee", tickets[0].AssigneeID)
	}
	if tickets[0].Priority != "" {
		t.Errorf("Priority: got %q, want empty string for null priority", tickets[0].Priority)
	}
}

func TestListTicketsPagination(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if n == 1 {
			w.Write([]byte(`{
				"startAt": 0, "maxResults": 50, "total": 2,
				"issues": [{"id":"1","key":"A-1","self":"","fields":{"summary":"A","status":{"id":"1","name":"To Do"},"labels":[],"updated":"2026-01-01T00:00:00-0000"}}]
			}`))
		} else {
			w.Write([]byte(`{
				"startAt": 1, "maxResults": 50, "total": 2,
				"issues": [{"id":"2","key":"A-2","self":"","fields":{"summary":"B","status":{"id":"1","name":"To Do"},"labels":[],"updated":"2026-01-01T00:00:00-0000"}}]
			}`))
		}
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	tickets, err := adapter.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("got %d tickets, want 2", len(tickets))
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected exactly 2 HTTP requests, got %d", callCount)
	}
}

func TestListTicketsSinceFilter(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"startAt":0,"maxResults":50,"total":0,"issues":[]}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	since := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	_, err := adapter.ListTickets(context.Background(), "1", &since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedURL, "jql=") {
		t.Errorf("expected jql= in URL, got: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "updated") {
		t.Errorf("expected 'updated' in jql param, got: %s", capturedURL)
	}
}

func TestListTicketsRawIsNonNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"startAt": 0, "maxResults": 50, "total": 1,
			"issues": [{"id":"1","key":"X-1","self":"","fields":{"summary":"x","status":{"id":"1","name":"To Do"},"labels":[],"updated":"2026-01-01T00:00:00-0000"}}]
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	tickets, err := adapter.ListTickets(context.Background(), "1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) == 0 {
		t.Fatal("expected at least one ticket")
	}
	for i, tk := range tickets {
		if tk.Raw == nil {
			t.Errorf("ticket[%d].Raw is nil, want non-nil empty map", i)
		}
	}
}

func TestListTickets401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "bad"}
	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.ListTickets(context.Background(), "1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *errAuth
	if !errors.As(err, &authErr) {
		t.Errorf("expected *errAuth, got %T: %v", err, err)
	}
}

func TestListTickets5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.ListTickets(context.Background(), "1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var retryErr *errRetryable
	if !errors.As(err, &retryErr) {
		t.Errorf("expected *errRetryable, got %T: %v", err, err)
	}
}
