package jira

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// makeResponseScheme builds a *model.ResponseScheme from a canned HTTP response.
func makeResponseScheme(statusCode int, body string, headers map[string]string) *model.ResponseScheme {
	rec := httptest.NewRecorder()
	for k, v := range headers {
		rec.Header().Set(k, v)
	}
	rec.WriteHeader(statusCode)
	if body != "" {
		rec.Body.WriteString(body)
	}
	httpResp := rec.Result()
	rs := &model.ResponseScheme{
		Response: httpResp,
		Code:     statusCode,
	}
	rs.Bytes.WriteString(body)
	return rs
}

func TestMapResponseError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		headers    map[string]string
		wantNil    bool
		wantIs     error
		wantAsType interface{}
		wantDelay  time.Duration
	}{
		{
			name:       "HTTP 401 errAuth",
			statusCode: 401,
			wantAsType: &errAuth{},
		},
		{
			name:    "HTTP 409 ErrConflict",
			statusCode: 409,
			wantIs:  tracker.ErrConflict,
		},
		{
			name:       "HTTP 429 with Retry-After",
			statusCode: 429,
			headers:    map[string]string{"Retry-After": "42"},
			wantAsType: &errRetryable{},
			wantDelay:  42 * time.Second,
		},
		{
			name:       "HTTP 429 without Retry-After uses default 60s",
			statusCode: 429,
			wantAsType: &errRetryable{},
			wantDelay:  60 * time.Second,
		},
		{
			name:       "HTTP 429 with Retry-After above cap uses 300s",
			statusCode: 429,
			headers:    map[string]string{"Retry-After": "9999"},
			wantAsType: &errRetryable{},
			wantDelay:  300 * time.Second,
		},
		{
			name:       "HTTP 503 with Retry-After 10",
			statusCode: 503,
			headers:    map[string]string{"Retry-After": "10"},
			wantAsType: &errRetryable{},
			wantDelay:  10 * time.Second,
		},
		{
			name:       "HTTP 500 no Retry-After uses default 60s",
			statusCode: 500,
			wantAsType: &errRetryable{},
			wantDelay:  60 * time.Second,
		},
		{
			name:       "HTTP 400 workflow-forbidden body",
			statusCode: 400,
			body:       "It is not possible to perform this transition",
			wantIs:     tracker.ErrInvalidTransition,
		},
		{
			name:       "HTTP 400 other body is errFatal not ErrInvalidTransition",
			statusCode: 400,
			body:       "customfield_10000 is required",
			wantAsType: &errFatal{},
		},
		{
			name:       "CAPTCHA header on 401 returns errCAPTCHA not errAuth",
			statusCode: 401,
			headers:    map[string]string{"X-Seraph-LoginReason": "AUTHENTICATION_DENIED"},
			wantAsType: &errCAPTCHA{},
		},
	}

	// Verify the two sentinels are distinct.
	if errors.Is(tracker.ErrConflict, tracker.ErrInvalidTransition) {
		t.Errorf("ErrConflict and ErrInvalidTransition must not be equal")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := makeResponseScheme(tt.statusCode, tt.body, tt.headers)
			err := mapResponseError(rs)

			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Errorf("errors.Is(%v, %v) = false, want true", err, tt.wantIs)
				}
			}

			if tt.wantAsType != nil {
				switch target := tt.wantAsType.(type) {
				case *errRetryable:
					var re *errRetryable
					if !errors.As(err, &re) {
						t.Fatalf("expected *errRetryable, got %T: %v", err, err)
					}
					if tt.wantDelay != 0 && re.RetryAfter() != tt.wantDelay {
						t.Errorf("RetryAfter: got %v, want %v", re.RetryAfter(), tt.wantDelay)
					}
					_ = target
				case *errAuth:
					var ae *errAuth
					if !errors.As(err, &ae) {
						t.Fatalf("expected *errAuth, got %T: %v", err, err)
					}
				case *errCAPTCHA:
					var ce *errCAPTCHA
					if !errors.As(err, &ce) {
						t.Fatalf("expected *errCAPTCHA, got %T: %v", err, err)
					}
					want := "tracker: locked — login via browser to clear CAPTCHA"
					if ce.Error() != want {
						t.Errorf("errCAPTCHA.Error(): got %q, want %q", ce.Error(), want)
					}
				case *errFatal:
					var fe *errFatal
					if !errors.As(err, &fe) {
						t.Fatalf("expected *errFatal, got %T: %v", err, err)
					}
					if errors.Is(err, tracker.ErrInvalidTransition) {
						t.Errorf("errFatal should not be ErrInvalidTransition")
					}
					if tt.body != "" && !strings.Contains(fe.body, tt.body) {
						t.Errorf("errFatal.body: got %q, want it to contain %q", fe.body, tt.body)
					}
				}
			}
		})
	}
}

// TestAuthErrorInterface verifies errAuth satisfies tracker.AuthError.
func TestAuthErrorInterface(t *testing.T) {
	var ae tracker.AuthError = &errAuth{statusCode: http.StatusUnauthorized}
	if !ae.IsAuthError() {
		t.Error("errAuth.IsAuthError() should return true")
	}
	if tracker.IsAuthError(ae) == false {
		t.Error("tracker.IsAuthError should return true for errAuth")
	}
}

// TestIsAuthErrorWrapped verifies wrapped errAuth still satisfies tracker.IsAuthError.
func TestIsAuthErrorWrapped(t *testing.T) {
	inner := &errAuth{statusCode: 401}
	wrapped := errors.Join(errors.New("outer"), inner)
	if !tracker.IsAuthError(wrapped) {
		t.Error("tracker.IsAuthError should return true for wrapped errAuth")
	}
}

func TestErrCAPTCHAMessage(t *testing.T) {
	e := &errCAPTCHA{}
	want := "tracker: locked — login via browser to clear CAPTCHA"
	if e.Error() != want {
		t.Errorf("got %q, want %q", e.Error(), want)
	}
}

func TestRetryAfterCapping(t *testing.T) {
	rs := makeResponseScheme(429, "", map[string]string{"Retry-After": "99999"})
	err := mapResponseError(rs)
	var re *errRetryable
	if !errors.As(err, &re) {
		t.Fatalf("expected *errRetryable, got %T", err)
	}
	if re.RetryAfter() > maxRetryAfter {
		t.Errorf("RetryAfter %v exceeds cap %v", re.RetryAfter(), maxRetryAfter)
	}
}

func TestMapResponseErrorNilResponse(t *testing.T) {
	err := mapResponseError(&model.ResponseScheme{Code: 500, Response: nil})
	if err == nil {
		t.Fatal("expected error for 500 with nil Response")
	}
	var re *errRetryable
	if !errors.As(err, &re) {
		t.Errorf("expected *errRetryable for 500, got %T: %v", err, err)
	}
}

func TestMapResponseErrorNilScheme(t *testing.T) {
	// Should not panic on nil scheme.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("mapResponseError panicked: %v", r)
		}
	}()
	// Nil scheme case - handled by nil check.
	rs := (*model.ResponseScheme)(nil)
	err := mapResponseError(rs)
	if err == nil {
		t.Error("expected error for nil ResponseScheme")
	}
}

func TestBufContainsBodyInFatal(t *testing.T) {
	rs := makeResponseScheme(404, "not found body", nil)
	err := mapResponseError(rs)
	var fe *errFatal
	if !errors.As(err, &fe) {
		t.Fatalf("expected *errFatal, got %T", err)
	}
	if !strings.Contains(fe.body, "not found body") {
		t.Errorf("errFatal.body missing expected content, got %q", fe.body)
	}
}
