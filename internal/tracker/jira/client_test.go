package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v3 "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/ctreminiom/go-atlassian/v2/jira/agile"
	"github.com/kevin-zou/cmux-board/internal/secretsink"
)

func testCredentials() Credentials {
	return Credentials{
		Site:     "test.atlassian.net",
		Email:    "user@example.com",
		APIToken: "test-api-token-abc123",
	}
}

// newTestV3Client creates a v3 client pointed at srv with basic auth.
func newTestV3Client(t *testing.T, srv *httptest.Server, creds Credentials) *v3.Client {
	t.Helper()
	c, err := v3.New(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("v3.New: %v", err)
	}
	c.Auth.SetBasicAuth(creds.Email, creds.APIToken)
	return c
}

// newTestAgileClient creates an agile client pointed at srv with basic auth.
func newTestAgileClient(t *testing.T, srv *httptest.Server, creds Credentials) *agile.Client {
	t.Helper()
	c, err := agile.New(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("agile.New: %v", err)
	}
	c.Auth.SetBasicAuth(creds.Email, creds.APIToken)
	return c
}

// newTestAdapter creates a JiraAdapter pointed at the given test server.
func newTestAdapter(t *testing.T, srv *httptest.Server, creds Credentials) *JiraAdapter {
	t.Helper()
	return &JiraAdapter{
		v3:    newTestV3Client(t, srv, creds),
		agile: newTestAgileClient(t, srv, creds),
		creds: creds,
	}
}

// TestAuthHeaderInjected verifies that go-atlassian sends a correct Basic auth header.
func TestAuthHeaderInjected(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"id","displayName":"Test","emailAddress":"user@example.com"}`))
	}))
	defer srv.Close()

	creds := testCredentials()
	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

// TestHeaderLoggingBan verifies that go-atlassian + our logging code never emit
// header-related strings. This is the local analog of F20.d (which is in security_test.go).
func TestHeaderLoggingBan(t *testing.T) {
	creds := testCredentials()
	basicValue := base64.StdEncoding.EncodeToString([]byte(creds.Email + ":" + creds.APIToken))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"id","displayName":"Test","emailAddress":"user@example.com"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(secretsink.Writer(&buf), &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

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
