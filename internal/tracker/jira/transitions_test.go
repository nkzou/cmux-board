package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

func TestLoadTransitionCachePopulatesRaw(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"transitions":[
			{"id":"31","name":"In Progress","to":{"name":"In Progress"}},
			{"id":"41","name":"Done","to":{"name":"Done"}}
		]}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	client := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}

	ticket := &tracker.Ticket{
		ID:  "10001",
		Key: "PROJ-1",
		Raw: map[string]any{},
	}

	err := LoadTransitionCache(context.Background(), client, ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cache, ok := ticket.Raw[rawKeyWorkflowTransitions].(map[string]string)
	if !ok {
		t.Fatalf("Raw[%q] is not map[string]string, got %T", rawKeyWorkflowTransitions, ticket.Raw[rawKeyWorkflowTransitions])
	}
	if cache["In Progress"] != "31" {
		t.Errorf("cache[In Progress]: got %q, want %q", cache["In Progress"], "31")
	}
	if cache["Done"] != "41" {
		t.Errorf("cache[Done]: got %q, want %q", cache["Done"], "41")
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 HTTP request, got %d", callCount)
	}
}

func TestLoadTransitionCacheCacheHit(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	client := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}

	// Pre-populated cache.
	ticket := &tracker.Ticket{
		ID:  "10001",
		Key: "PROJ-1",
		Raw: map[string]any{
			rawKeyWorkflowTransitions: map[string]string{"In Progress": "31"},
		},
	}

	err := LoadTransitionCache(context.Background(), client, ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 0 {
		t.Errorf("expected 0 HTTP requests on cache hit, got %d", callCount)
	}
}

func TestTransitionIDFromRawCaseInsensitive(t *testing.T) {
	ticket := &tracker.Ticket{
		Raw: map[string]any{
			rawKeyWorkflowTransitions: map[string]string{"in progress": "31"},
		},
	}

	id, ok := TransitionIDFromRaw(ticket, "In Progress")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if id != "31" {
		t.Errorf("got id %q, want %q", id, "31")
	}
}

func TestTransitionIDFromRawMiss(t *testing.T) {
	ticket := &tracker.Ticket{
		Raw: map[string]any{
			rawKeyWorkflowTransitions: map[string]string{"Done": "41"},
		},
	}

	id, ok := TransitionIDFromRaw(ticket, "Blocked")
	if ok {
		t.Errorf("expected cache miss, got hit with id %q", id)
	}
	if id != "" {
		t.Errorf("expected empty id on miss, got %q", id)
	}
}

func TestTransitionIDFromRawNilMap(t *testing.T) {
	tests := []struct {
		name   string
		ticket *tracker.Ticket
	}{
		{
			name:   "nil Raw",
			ticket: &tracker.Ticket{Raw: nil},
		},
		{
			name:   "Raw without workflow_transitions key",
			ticket: &tracker.Ticket{Raw: map[string]any{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := TransitionIDFromRaw(tt.ticket, "In Progress")
			if ok || id != "" {
				t.Errorf("expected miss (false, \"\"), got (%v, %q)", ok, id)
			}
		})
	}
}

func TestWipeTransitionCache(t *testing.T) {
	ticket := &tracker.Ticket{
		Raw: map[string]any{
			rawKeyWorkflowTransitions: map[string]string{"In Progress": "31"},
		},
	}

	WipeTransitionCache(ticket)

	_, ok := TransitionIDFromRaw(ticket, "In Progress")
	if ok {
		t.Error("expected cache miss after wipe, got hit")
	}
}

func TestLoadTransitionCacheNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	client := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}

	ticket := &tracker.Ticket{
		ID:  "10001",
		Key: "PROJ-1",
		Raw: map[string]any{},
	}

	err := LoadTransitionCache(context.Background(), client, ticket)
	if err == nil {
		t.Fatal("expected error on network error, got nil")
	}

	var retryErr *errRetryable
	if !errors.As(err, &retryErr) {
		t.Errorf("expected *errRetryable, got %T: %v", err, err)
	}

	// Raw must NOT have been partially written on error.
	if _, ok := ticket.Raw[rawKeyWorkflowTransitions]; ok {
		t.Error("Raw should not contain workflow_transitions key after failed load")
	}
}
