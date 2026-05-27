package jira

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

const (
	defaultRetryAfter     = 60 * time.Second
	maxRetryAfter         = 5 * time.Minute
	invalidTransitionMsg  = "It is not possible to perform this transition"
)

// errRetryable is returned when the server signals the request should be retried
// after a delay (HTTP 429 or 5xx). Callers use errors.As to retrieve the delay.
type errRetryable struct {
	statusCode int
	retryAfter time.Duration
}

func (e *errRetryable) Error() string {
	return fmt.Sprintf("tracker: retryable error %d (retry after %s)", e.statusCode, e.retryAfter)
}

// RetryAfter returns the recommended delay before retrying.
func (e *errRetryable) RetryAfter() time.Duration { return e.retryAfter }

// errAuth is returned on HTTP 401 (unauthorized, no CAPTCHA).
// It satisfies the tracker.AuthError interface so the sync package can classify
// poll failures without importing jira-specific types.
type errAuth struct {
	statusCode int
}

func (e *errAuth) Error() string {
	return fmt.Sprintf("tracker: auth error %d", e.statusCode)
}

// IsAuthError implements tracker.AuthError.
func (e *errAuth) IsAuthError() bool { return true }

// errCAPTCHA is returned when the Jira CAPTCHA lockout header is present.
type errCAPTCHA struct{}

func (e *errCAPTCHA) Error() string {
	return "tracker: locked — login via browser to clear CAPTCHA"
}

// errFatal is returned for non-retryable, non-auth HTTP errors (400 non-workflow, 403, 404, etc.).
type errFatal struct {
	statusCode int
	body       string
}

func (e *errFatal) Error() string {
	return fmt.Sprintf("tracker: fatal error %d: %s", e.statusCode, e.body)
}

const (
	seraphLoginReasonHeader = "X-Seraph-LoginReason"
	captchaDeniedValue      = "AUTHENTICATION_DENIED"
)

// mapResponseError converts a go-atlassian ResponseScheme into our typed errors.
// Call when the library returns a non-nil error or when you need to check for
// CAPTCHA / non-2xx status from the embedded *http.Response.
//
// Decision table:
//  1. CAPTCHA header present → errCAPTCHA
//  2. 401 → errAuth
//  3. 409 → tracker.ErrConflict
//  4. 429 or 5xx → errRetryable
//  5. 400 + workflow-forbidden body → tracker.ErrInvalidTransition
//  6. default → errFatal
func mapResponseError(resp *model.ResponseScheme) error {
	if resp == nil {
		return fmt.Errorf("tracker: nil response scheme")
	}

	// 1. CAPTCHA check on the embedded *http.Response header.
	if resp.Response != nil {
		if strings.Contains(resp.Response.Header.Get(seraphLoginReasonHeader), captchaDeniedValue) {
			return &errCAPTCHA{}
		}
	}

	body := resp.Bytes.String()
	code := resp.Code

	// 2. 401
	if code == http.StatusUnauthorized {
		return &errAuth{statusCode: code}
	}

	// 3. 409 — OCC conflict.
	if code == http.StatusConflict {
		return tracker.ErrConflict
	}

	// 4. 429 or 5xx — retryable.
	if code == http.StatusTooManyRequests || code >= 500 {
		delay := retryAfterFromResp(resp)
		return &errRetryable{statusCode: code, retryAfter: delay}
	}

	// 5. 400 — check for workflow-forbidden message.
	if code == http.StatusBadRequest {
		if strings.Contains(body, invalidTransitionMsg) {
			return tracker.ErrInvalidTransition
		}
		return &errFatal{statusCode: code, body: body}
	}

	// 6. Default.
	return &errFatal{statusCode: code, body: body}
}

// retryAfterFromResp parses the Retry-After header from the embedded *http.Response.
func retryAfterFromResp(resp *model.ResponseScheme) time.Duration {
	if resp.Response == nil {
		return defaultRetryAfter
	}
	val := resp.Response.Header.Get("Retry-After")
	if val == "" {
		return defaultRetryAfter
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs <= 0 {
		return defaultRetryAfter
	}
	d := time.Duration(secs) * time.Second
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}
