package jira

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

func makeResponse(statusCode int, body string, headers map[string]string) *http.Response {
	rec := httptest.NewRecorder()
	for k, v := range headers {
		rec.Header().Set(k, v)
	}
	rec.WriteHeader(statusCode)
	if body != "" {
		rec.Body.WriteString(body)
	}
	return rec.Result()
}

func TestClassifyResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		headers    map[string]string
		wantNil    bool // true if error should be nil
		wantIs     error
		wantAsType interface{}
		wantDelay  time.Duration
	}{
		{
			name:       "HTTP 200 nil error body returned",
			statusCode: 200,
			body:       `{"key":"value"}`,
			wantNil:    true,
		},
		{
			name:       "HTTP 204 nil error empty body",
			statusCode: 204,
			body:       "",
			wantNil:    true,
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
			name:    "HTTP 409 ErrConflict",
			statusCode: 409,
			wantIs:  tracker.ErrConflict,
		},
		{
			name:       "HTTP 401 errAuth",
			statusCode: 401,
			wantAsType: &errAuth{},
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
			name:       "CAPTCHA header on 200 returns errCAPTCHA",
			statusCode: 200,
			body:       "ok",
			headers:    map[string]string{"X-Seraph-LoginReason": "AUTHENTICATION_DENIED"},
			wantAsType: &errCAPTCHA{},
		},
		{
			name:       "CAPTCHA header on 401 returns errCAPTCHA not errAuth",
			statusCode: 401,
			headers:    map[string]string{"X-Seraph-LoginReason": "AUTHENTICATION_DENIED"},
			wantAsType: &errCAPTCHA{},
		},
		{
			name:    "ErrConflict and ErrInvalidTransition are distinct",
			// special case: tested outside the main loop below
		},
	}

	// Verify the two sentinels are distinct.
	if errors.Is(tracker.ErrConflict, tracker.ErrInvalidTransition) {
		t.Errorf("ErrConflict and ErrInvalidTransition must not be equal")
	}

	for _, tt := range tests {
		if tt.name == "ErrConflict and ErrInvalidTransition are distinct" {
			continue // handled above
		}
		t.Run(tt.name, func(t *testing.T) {
			resp := makeResponse(tt.statusCode, tt.body, tt.headers)
			body, err := classifyResponse(resp)

			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got nil (body: %q)", body)
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
					// Verify the canonical CAPTCHA error message.
					want := "tracker: locked — login via browser to clear CAPTCHA"
					if ce.Error() != want {
						t.Errorf("errCAPTCHA.Error(): got %q, want %q", ce.Error(), want)
					}
				case *errFatal:
					var fe *errFatal
					if !errors.As(err, &fe) {
						t.Fatalf("expected *errFatal, got %T: %v", err, err)
					}
					// Verify it's NOT ErrInvalidTransition.
					if errors.Is(err, tracker.ErrInvalidTransition) {
						t.Errorf("errFatal should not be ErrInvalidTransition")
					}
					// Verify the body is preserved in the error.
					if tt.body != "" && !strings.Contains(fe.body, tt.body) {
						t.Errorf("errFatal.body: got %q, want it to contain %q", fe.body, tt.body)
					}
				}
			}
		})
	}
}
