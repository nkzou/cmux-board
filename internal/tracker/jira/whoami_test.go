package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhoAmIHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"accountId": "557058:abc123",
			"displayName": "Alice Tester",
			"emailAddress": "alice@example.com"
		}`))
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "alice@example.com", APIToken: "token"}
	adapter := newTestAdapter(t, srv, creds)

	identity, err := adapter.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.ID != "557058:abc123" {
		t.Errorf("ID: got %q, want %q", identity.ID, "557058:abc123")
	}
	if identity.DisplayName != "Alice Tester" {
		t.Errorf("DisplayName: got %q, want %q", identity.DisplayName, "Alice Tester")
	}
	if identity.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", identity.Email, "alice@example.com")
	}
}

func TestWhoAmI401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "alice@example.com", APIToken: "bad"}
	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *errAuth
	if !errors.As(err, &authErr) {
		t.Errorf("expected *errAuth, got %T: %v", err, err)
	}
}

func TestWhoAmICAPTCHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seraph-LoginReason", "AUTHENTICATION_DENIED")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := Credentials{Site: "test.atlassian.net", Email: "alice@example.com", APIToken: "token"}
	adapter := newTestAdapter(t, srv, creds)

	_, err := adapter.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var captchaErr *errCAPTCHA
	if !errors.As(err, &captchaErr) {
		t.Errorf("expected *errCAPTCHA, got %T: %v", err, err)
	}
}
