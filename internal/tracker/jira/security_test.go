package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kevin-zou/cmux-board/internal/secretsink"
)

const (
	jirasecKnownToken = "ATATT3xFfGF0ABCDEFGHIJKLMNOPQRSTUVWXYZabcd1234"
	jirasecEmail      = "user@example.com"
	jirasecTestTimeout = 5 * time.Second
)

// jirasecLogger builds a slog logger that writes through secretsink.Writer(buf),
// replaces the global slog default, and returns a restore function.
func jirasecLogger(t *testing.T, buf *bytes.Buffer) func() {
	t.Helper()
	secretsink.Register(jirasecKnownToken)
	h := slog.NewJSONHandler(secretsink.Writer(buf), &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	return func() { slog.SetDefault(prev) }
}

// TestF20d_NoHeaderLoggingDuringRequest verifies that HTTP headers (Authorization, Cookie,
// Proxy-Authorization, and the full Basic credential) never appear in slog output during
// a real client.Do call.
// Ties to: F20, F20.d
func TestF20d_NoHeaderLoggingDuringRequest(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":    "557058:test",
			"displayName":  "Test User",
			"emailAddress": jirasecEmail,
		})
	}))
	defer srv.Close()

	buf := &bytes.Buffer{}
	restore := jirasecLogger(t, buf)
	defer restore()

	creds := Credentials{
		Site:     "test.atlassian.net",
		Email:    jirasecEmail,
		APIToken: jirasecKnownToken,
	}
	client := newClientWithHTTP(creds, srv.Client())
	client.baseURL = srv.URL

	resp, err := client.Do(context.Background(), "GET", "/rest/api/3/myself", nil)
	if err != nil {
		t.Fatalf("F20.d: client.Do unexpected error: %v", err)
	}
	resp.Body.Close()

	captured := buf.String()
	t.Logf("F20.d captured slog output:\n%s", captured)

	forbiddenStrings := []string{
		"Authorization",
		"Cookie",
		"Proxy-Authorization",
		jirasecEmail,
		"Headers",
		"Header:",
		jirasecKnownToken,
		base64.StdEncoding.EncodeToString([]byte(jirasecEmail + ":" + jirasecKnownToken)),
	}
	for _, forbidden := range forbiddenStrings {
		if strings.Contains(captured, forbidden) {
			t.Errorf("F20.d: captured slog output contains forbidden string %q", forbidden)
		}
	}

	// Correctness check: the Authorization header reached the server correctly.
	expectedBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte(jirasecEmail+":"+jirasecKnownToken))
	if receivedAuth != expectedBasic {
		t.Errorf("F20.d correctness: server received Authorization=%q, want %q", receivedAuth, expectedBasic)
	}
}

// TestF20e_Meta_HeaderLeakIsDetectable proves that F20.d is non-vacuous:
// a deliberately-bad fixture that logs req.Header DOES produce "Authorization"
// in the output, confirming F20.d would fail if header logging were introduced.
// Ties to: F20.e
func TestF20e_Meta_HeaderLeakIsDetectable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	buf := &bytes.Buffer{}
	restore := jirasecLogger(t, buf)
	defer restore()

	creds := Credentials{
		Site:     "test.atlassian.net",
		Email:    jirasecEmail,
		APIToken: jirasecKnownToken,
	}
	client := newClientWithHTTP(creds, srv.Client())
	client.baseURL = srv.URL

	t.Run("F20e_meta", func(t *testing.T) {
		// Deliberately-bad fixture: logs req.Header before sending.
		// This is the FORBIDDEN pattern — this sub-test proves it IS detectable.
		ctx, cancel := context.WithTimeout(context.Background(), jirasecTestTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", client.baseURL+"/rest/api/3/myself", nil)
		if err != nil {
			t.Fatalf("F20e meta: build request: %v", err)
		}
		req.Header.Set("Authorization", "Basic "+buildBasicAuth(creds.Email, creds.APIToken))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Deliberately log the header (this is the bad fixture that SHOULD leak).
		slog.Debug("bad-fixture", "headers", req.Header)

		resp, err := client.httpClient.Do(req)
		if err != nil {
			t.Fatalf("F20e meta: do request: %v", err)
		}
		resp.Body.Close()

		leaked := buf.String()
		// The Authorization header NAME is not a registered secret, so it WILL appear.
		if !strings.Contains(leaked, "Authorization") {
			t.Errorf("F20.e meta: expected bad fixture to leak 'Authorization' header name, got:\n%s", leaked)
		}
	})
}
