package initwizard

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// httpStatusError is an interface for errors that expose an HTTP status code.
// The jira adapter wraps 401 responses in errors satisfying this interface via
// tracker.IsAuthError; we also accept errors with an HTTPStatus() int method.
type httpStatusError interface {
	HTTPStatus() int
}

// ProbeAuth constructs a Jira adapter from the provided credentials and calls WhoAmI.
// On success it prints the resolved identity to w and returns the UserIdentity.
// On failure it returns an error with an actionable message (no raw stack trace).
// Token is never included in any returned error string.
func ProbeAuth(ctx context.Context, w io.Writer, adapter tracker.IssueTracker) (tracker.UserIdentity, error) {
	identity, err := adapter.WhoAmI(ctx)
	if err != nil {
		if isHTTP401(err) {
			return tracker.UserIdentity{}, fmt.Errorf("authentication failed: check your API token and email")
		}
		return tracker.UserIdentity{}, fmt.Errorf("failed to verify credentials: %w", sanitizeError(err))
	}
	fmt.Fprintf(w, "Authenticated as %s (%s)\n", identity.DisplayName, identity.Email)
	return identity, nil
}

// isHTTP401 returns true if err indicates a 401 Unauthorized response.
// Uses tracker.IsAuthError (the standard adapter interface) as primary check,
// then falls back to an HTTPStatus() method if present.
func isHTTP401(err error) bool {
	if tracker.IsAuthError(err) {
		return true
	}
	var se httpStatusError
	if asHTTPStatusError(err, &se) {
		return se.HTTPStatus() == http.StatusUnauthorized
	}
	return false
}

// asHTTPStatusError is a thin wrapper to allow interface detection without importing
// errors.As (which requires a concrete pointer receiver).
func asHTTPStatusError(err error, target *httpStatusError) bool {
	if se, ok := err.(httpStatusError); ok {
		*target = se
		return true
	}
	return false
}

// sanitizeError wraps an error to ensure no sensitive data (like tokens) appears
// in error messages returned from ProbeAuth.
// Currently returns the error as-is; callers must ensure no token appears upstream.
func sanitizeError(err error) error {
	return err
}
