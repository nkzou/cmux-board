package jira

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// sequentialHandler serves canned responses in order, tracking call count.
type sequentialHandler struct {
	responses []func(w http.ResponseWriter, r *http.Request)
	callIdx   int32
}

func (h *sequentialHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idx := int(atomic.AddInt32(&h.callIdx, 1)) - 1
	if idx >= len(h.responses) {
		http.Error(w, "unexpected call", http.StatusInternalServerError)
		return
	}
	h.responses[idx](w, r)
}

func (h *sequentialHandler) CallCount() int {
	return int(atomic.LoadInt32(&h.callIdx))
}

func jsonOK(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}
}

func statusResponse(status string) func(http.ResponseWriter, *http.Request) {
	return jsonOK(`{"fields":{"status":{"name":"` + status + `"}}}`)
}

func transitionsJSON(transitions string) func(http.ResponseWriter, *http.Request) {
	return jsonOK(`{"transitions":[` + transitions + `]}`)
}

func statusCode(code int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}
}

func statusCodeWithBody(code int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write([]byte(body))
	}
}

func newAdapterWithHandler(t *testing.T, h http.Handler) (*JiraAdapter, *sequentialHandler) {
	t.Helper()
	seq, ok := h.(*sequentialHandler)
	if !ok {
		panic("expected *sequentialHandler")
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	creds := Credentials{Site: "test.atlassian.net", Email: "u@e.com", APIToken: "t"}
	c := &jiraClient{httpClient: srv.Client(), baseURL: srv.URL, creds: creds}
	return newJiraAdapterWithClient(c), seq
}

func TestTransitionStatusHappyPath(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			// Step 1: GET current status → "To Do"
			statusResponse("To Do"),
			// Step 3: GET transitions → one matching transition
			transitionsJSON(`{"id":"31","name":"Start Progress","to":{"name":"In Progress"}}`),
			// Step 4: POST transition → 204 No Content
			func(w http.ResponseWriter, r *http.Request) {
				// Verify POST body contains the transition ID.
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), `"31"`) {
					t.Errorf("POST body missing transition ID 31: %s", body)
				}
				w.WriteHeader(http.StatusNoContent)
			},
		},
	}
	adapter, seq := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if seq.CallCount() != 3 {
		t.Errorf("expected 3 HTTP requests, got %d", seq.CallCount())
	}
}

func TestTransitionStatusOCCMismatch_Step2(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			// Step 1: GET current status → "In Progress" (already moved)
			statusResponse("In Progress"),
		},
	}
	adapter, seq := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "Done")
	if err == nil {
		t.Fatal("expected ErrConflict, got nil")
	}
	if !errors.Is(err, tracker.ErrConflict) {
		t.Errorf("expected tracker.ErrConflict, got %T: %v", err, err)
	}
	// CRITICAL: only ONE HTTP request must be made (no transitions GET, no POST).
	if seq.CallCount() != 1 {
		t.Errorf("expected exactly 1 HTTP request on OCC mismatch, got %d", seq.CallCount())
	}
}

func TestTransitionStatusNoMatchingTransition(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			// Step 1: GET current status → "To Do" (matches expectedFromStatus)
			statusResponse("To Do"),
			// Step 3: GET transitions → no matching transition for "In Progress"
			transitionsJSON(
				`{"id":"51","name":"Close","to":{"name":"Done"}},` +
					`{"id":"61","name":"Block","to":{"name":"Blocked"}}`,
			),
		},
	}
	adapter, seq := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err == nil {
		t.Fatal("expected ErrInvalidTransition, got nil")
	}
	if !errors.Is(err, tracker.ErrInvalidTransition) {
		t.Errorf("expected tracker.ErrInvalidTransition, got %T: %v", err, err)
	}
	// CRITICAL: exactly TWO HTTP requests (GET status + GET transitions; no POST).
	if seq.CallCount() != 2 {
		t.Errorf("expected exactly 2 HTTP requests on no-match, got %d", seq.CallCount())
	}
}

func TestTransitionStatusPOST409(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			statusResponse("To Do"),
			transitionsJSON(`{"id":"31","name":"Start","to":{"name":"In Progress"}}`),
			statusCode(http.StatusConflict), // POST → 409
		},
	}
	adapter, _ := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err == nil {
		t.Fatal("expected ErrConflict from POST 409, got nil")
	}
	if !errors.Is(err, tracker.ErrConflict) {
		t.Errorf("expected tracker.ErrConflict, got %T: %v", err, err)
	}
}

func TestTransitionStatusPOST400WorkflowForbidden(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			statusResponse("To Do"),
			transitionsJSON(`{"id":"31","name":"Start","to":{"name":"In Progress"}}`),
			statusCodeWithBody(http.StatusBadRequest, "It is not possible to perform this transition"),
		},
	}
	adapter, _ := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err == nil {
		t.Fatal("expected ErrInvalidTransition from POST 400 forbidden, got nil")
	}
	if !errors.Is(err, tracker.ErrInvalidTransition) {
		t.Errorf("expected tracker.ErrInvalidTransition, got %T: %v", err, err)
	}
}

func TestTransitionStatusPOST400OtherBody(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			statusResponse("To Do"),
			transitionsJSON(`{"id":"31","name":"Start","to":{"name":"In Progress"}}`),
			statusCodeWithBody(http.StatusBadRequest, "customfield_10000 is required"),
		},
	}
	adapter, _ := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err == nil {
		t.Fatal("expected errFatal from POST 400 other body, got nil")
	}
	var fatalErr *errFatal
	if !errors.As(err, &fatalErr) {
		t.Errorf("expected *errFatal, got %T: %v", err, err)
	}
	if errors.Is(err, tracker.ErrInvalidTransition) {
		t.Error("other-body 400 must NOT be ErrInvalidTransition")
	}
}

func TestTransitionStatusPOST5xx(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			statusResponse("To Do"),
			transitionsJSON(`{"id":"31","name":"Start","to":{"name":"In Progress"}}`),
			statusCode(http.StatusInternalServerError),
		},
	}
	adapter, _ := newAdapterWithHandler(t, h)

	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err == nil {
		t.Fatal("expected errRetryable from POST 5xx, got nil")
	}
	var retryErr *errRetryable
	if !errors.As(err, &retryErr) {
		t.Errorf("expected *errRetryable, got %T: %v", err, err)
	}
}

func TestTransitionStatusCaseInsensitiveMatch(t *testing.T) {
	h := &sequentialHandler{
		responses: []func(http.ResponseWriter, *http.Request){
			statusResponse("To Do"),
			// Transition uses all-lowercase target name.
			transitionsJSON(`{"id":"31","name":"start","to":{"name":"in progress"}}`),
			statusCode(http.StatusNoContent), // POST → 204
		},
	}
	adapter, _ := newAdapterWithHandler(t, h)

	// toStatus uses mixed case — should match case-insensitively.
	err := adapter.TransitionStatus(context.Background(), "PROJ-1", "To Do", "In Progress")
	if err != nil {
		t.Fatalf("expected case-insensitive match to succeed, got: %v", err)
	}
}
