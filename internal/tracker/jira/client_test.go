package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kevin-zou/cmux-board/internal/secretsink"
)

func testCredentials() Credentials {
	return Credentials{
		Site:     "test.atlassian.net",
		Email:    "user@example.com",
		APIToken: "test-api-token-abc123",
	}
}

func TestDoInjectsAuthHeader(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	creds := testCredentials()
	c := &jiraClient{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		creds:      creds,
	}

	resp, err := c.Do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if !strings.HasPrefix(capturedAuth, "Basic ") {
		t.Fatalf("Authorization header missing Basic prefix: %q", capturedAuth)
	}
	encoded := strings.TrimPrefix(capturedAuth, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to base64-decode auth: %v", err)
	}
	want := creds.Email + ":" + creds.APIToken
	if string(decoded) != want {
		t.Errorf("got credential %q, want %q", string(decoded), want)
	}
}

func TestDoSetsContentTypeAndAccept(t *testing.T) {
	var capturedContentType, capturedAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContentType = r.Header.Get("Content-Type")
		capturedAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &jiraClient{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		creds:      testCredentials(),
	}

	resp, err := c.Do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if capturedContentType != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", capturedContentType, "application/json")
	}
	if capturedAccept != "application/json" {
		t.Errorf("Accept: got %q, want %q", capturedAccept, "application/json")
	}
}

func TestDoRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never be called with a cancelled context.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &jiraClient{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		creds:      testCredentials(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := c.Do(ctx, http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func TestDoRetryAfterParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &jiraClient{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		creds:      testCredentials(),
	}

	_, err := c.Do(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("expected retryable error, got nil")
	}

	var retryErr *errRetryable
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected *errRetryable, got %T: %v", err, err)
	}
	if retryErr.RetryAfter() != 42*1e9 {
		t.Errorf("RetryAfter: got %v, want 42s", retryErr.RetryAfter())
	}
}

// TestDoHeaderLoggingBan is the F20.d test: verifies that a real HTTP cycle through
// the client emits zero occurrences of header-related strings in slog output.
func TestDoHeaderLoggingBan(t *testing.T) {
	creds := testCredentials()
	basicValue := base64.StdEncoding.EncodeToString([]byte(creds.Email + ":" + creds.APIToken))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Capture slog output via secretsink.Writer wrapping a buffer.
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(secretsink.Writer(&buf), &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	// Temporarily replace the default logger.
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	c := &jiraClient{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		creds:      creds,
	}

	resp, err := c.Do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	output := buf.String()

	// Sanity: at least one log line was emitted.
	if output == "" {
		t.Fatal("slog output is empty — client did not emit any log lines")
	}

	forbidden := []string{
		"Authorization",
		"Cookie",
		"Proxy-Authorization",
		creds.Email,
		basicValue,
		"Header:",
		"Headers",
	}
	for _, f := range forbidden {
		if strings.Contains(output, f) {
			t.Errorf("slog output contains forbidden string %q\noutput: %s", f, output)
		}
	}
}
