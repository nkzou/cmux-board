package initwizard

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// mockAdapter is a test double for tracker.IssueTracker.
type mockAdapter struct {
	whoAmIFn func(ctx context.Context) (tracker.UserIdentity, error)
}

func (m *mockAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	return m.whoAmIFn(ctx)
}
func (m *mockAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	return tracker.Board{}, errors.New("not implemented")
}
func (m *mockAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	return errors.New("not implemented")
}
func (m *mockAdapter) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{}
}

// authErrMock satisfies tracker.AuthError to signal 401.
type authErrMock struct{ msg string }

func (e *authErrMock) Error() string    { return e.msg }
func (e *authErrMock) IsAuthError() bool { return true }

func TestProbeAuth_HappyPath(t *testing.T) {
	const testToken = "test-token-value"
	adapter := &mockAdapter{
		whoAmIFn: func(ctx context.Context) (tracker.UserIdentity, error) {
			return tracker.UserIdentity{
				ID:          "u1",
				DisplayName: "Alice Doe",
				Email:       "alice@example.com",
			}, nil
		},
	}
	var w bytes.Buffer
	identity, err := ProbeAuth(context.Background(), &w, adapter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.DisplayName != "Alice Doe" {
		t.Errorf("expected DisplayName %q, got %q", "Alice Doe", identity.DisplayName)
	}
	if !strings.Contains(w.String(), "Alice Doe") {
		t.Errorf("expected display name in output, got: %q", w.String())
	}
	if !strings.Contains(w.String(), "alice@example.com") {
		t.Errorf("expected email in output, got: %q", w.String())
	}
	// Token must not appear in output
	if strings.Contains(w.String(), testToken) {
		t.Errorf("token leaked into output: %q", w.String())
	}
}

func TestProbeAuth_401Error(t *testing.T) {
	const testToken = "test-token-value"
	adapter := &mockAdapter{
		whoAmIFn: func(ctx context.Context) (tracker.UserIdentity, error) {
			return tracker.UserIdentity{}, &authErrMock{msg: "401 unauthorized with token " + testToken}
		},
	}
	var w bytes.Buffer
	_, err := ProbeAuth(context.Background(), &w, adapter)
	if err == nil {
		t.Fatal("expected error from 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' in error, got: %v", err)
	}
	// Token must not appear in error
	if strings.Contains(err.Error(), testToken) {
		t.Errorf("token leaked into error message: %v", err)
	}
	// Output must not contain the token
	if strings.Contains(w.String(), testToken) {
		t.Errorf("token leaked into output: %q", w.String())
	}
}

func TestProbeAuth_NetworkError(t *testing.T) {
	const testToken = "test-token-value"
	adapter := &mockAdapter{
		whoAmIFn: func(ctx context.Context) (tracker.UserIdentity, error) {
			return tracker.UserIdentity{}, errors.New("connection refused")
		},
	}
	var w bytes.Buffer
	_, err := ProbeAuth(context.Background(), &w, adapter)
	if err == nil {
		t.Fatal("expected error from network failure")
	}
	if !strings.Contains(err.Error(), "failed to verify credentials") {
		t.Errorf("expected 'failed to verify credentials' in error, got: %v", err)
	}
	// Token must not appear in error or output
	if strings.Contains(err.Error(), testToken) {
		t.Errorf("token leaked into error message: %v", err)
	}
	if strings.Contains(w.String(), testToken) {
		t.Errorf("token leaked into output: %q", w.String())
	}
}
