package jira

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kevin-zou/cmux-board/internal/tracker"
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
type errAuth struct {
	statusCode int
}

func (e *errAuth) Error() string {
	return fmt.Sprintf("tracker: auth error %d", e.statusCode)
}

// errCAPTCHA is returned when the Jira CAPTCHA lockout header is present.
// Resolution: the user must log in via browser to clear the CAPTCHA.
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
	invalidTransitionMsg    = "It is not possible to perform this transition"
)

// classifyResponse reads the HTTP response status and body snapshot, then returns
// the appropriate typed error (nil if 2xx). The body is read and buffered here;
// the returned bodyBytes slice is safe for the caller to decode after classifyResponse returns.
// The response body is consumed; callers MUST NOT read resp.Body after calling this.
//
// Decision table (precedence order):
//  1. CAPTCHA header present → errCAPTCHA (regardless of status)
//  2. 2xx → nil, body returned
//  3. 409 → tracker.ErrConflict
//  4. 401 → errAuth
//  5. 429 or 5xx → errRetryable with Retry-After delay
//  6. 400 + workflow-forbidden body → tracker.ErrInvalidTransition
//  7. default → errFatal
func classifyResponse(resp *http.Response) ([]byte, error) {
	// 1. CAPTCHA check — takes precedence over everything including 2xx.
	if strings.Contains(resp.Header.Get(seraphLoginReasonHeader), captchaDeniedValue) {
		// Drain body to allow connection reuse.
		if resp.Body != nil {
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			resp.Body.Close()
		}
		return nil, &errCAPTCHA{}
	}

	// 2. 2xx — success; read and return body.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if resp.Body == nil {
			return nil, nil
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return body, nil
	}

	// For all error cases, drain and close the body.
	var bodyBytes []byte
	if resp.Body != nil {
		defer resp.Body.Close()
		bodyBytes, _ = io.ReadAll(resp.Body)
	}

	// 3. 409 — OCC conflict.
	if resp.StatusCode == http.StatusConflict {
		return nil, tracker.ErrConflict
	}

	// 4. 401 — auth error (CAPTCHA already handled above).
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &errAuth{statusCode: resp.StatusCode}
	}

	// 5. 429 or 5xx — retryable.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		delay := retryAfterFromHeader(resp)
		return nil, &errRetryable{statusCode: resp.StatusCode, retryAfter: delay}
	}

	// 6. 400 — check for workflow-forbidden message.
	if resp.StatusCode == http.StatusBadRequest {
		if strings.Contains(string(bodyBytes), invalidTransitionMsg) {
			return nil, tracker.ErrInvalidTransition
		}
		return nil, &errFatal{statusCode: resp.StatusCode, body: string(bodyBytes)}
	}

	// 7. Default — all other non-2xx.
	return nil, &errFatal{statusCode: resp.StatusCode, body: string(bodyBytes)}
}

// retryAfterFromHeader parses the Retry-After header (integer seconds).
// Returns defaultRetryAfter if absent or unparseable; caps at maxRetryAfter.
func retryAfterFromHeader(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
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
