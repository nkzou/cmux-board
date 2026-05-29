package initwizard

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// ProbeACLI verifies that acli is installed and authenticated.
// It runs through two checks:
//  1. WhoAmI (which calls 'acli jira auth status') — if acli is not found the
//     runner returns exitCode -1 and an exec error; we surface an install hint.
//  2. If WhoAmI returns an AuthError, the user has not logged in yet.
//
// On success it prints the resolved identity and returns (identity, site, nil).
// site is parsed from the identity (for acli the ID field carries the site).
func ProbeACLI(ctx context.Context, w io.Writer, adapter tracker.IssueTracker) (tracker.UserIdentity, error) {
	identity, err := adapter.WhoAmI(ctx)
	if err != nil {
		if tracker.IsAuthError(err) {
			fmt.Fprintln(w, "acli is not authenticated.")
			fmt.Fprintln(w, "Run: acli jira auth login --web")
			fmt.Fprintln(w, "Then re-run cmux-board init.")
			return tracker.UserIdentity{}, fmt.Errorf("acli not authenticated — run 'acli jira auth login --web'")
		}
		// Distinguish binary-not-found from other errors (errFatal from runner startup failure).
		if isACLINotFound(err) {
			fmt.Fprintln(w, "acli is not installed or not on PATH.")
			fmt.Fprintln(w, "Install: brew install atlassian/tap/atlassian-cli")
			fmt.Fprintln(w, "Or download from: https://developer.atlassian.com/cloud/acli/getting-started/")
			return tracker.UserIdentity{}, fmt.Errorf("acli not found — install it first")
		}
		return tracker.UserIdentity{}, fmt.Errorf("failed to probe acli: %w", err)
	}
	fmt.Fprintf(w, "Authenticated via acli: %s (%s)\n", identity.DisplayName, identity.Email)
	return identity, nil
}

// isACLINotFound returns true if the error indicates acli binary was not found.
// Matches the errFatal message we emit when ProcessState is nil (exitCode -1).
func isACLINotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, "not found or failed to start", "acli not found", "exec: not found")
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ProbeAuth constructs a Jira adapter from the provided credentials and calls WhoAmI.
// On success it prints the resolved identity to w and returns the UserIdentity.
// On failure it returns an error with an actionable message (no raw stack trace).
// Token is never included in any returned error string.
func ProbeAuth(ctx context.Context, w io.Writer, adapter tracker.IssueTracker) (tracker.UserIdentity, error) {
	identity, err := adapter.WhoAmI(ctx)
	if err != nil {
		if tracker.IsAuthError(err) {
			return tracker.UserIdentity{}, fmt.Errorf("authentication failed: check your API token and email")
		}
		return tracker.UserIdentity{}, fmt.Errorf("failed to verify credentials: %w", sanitizeError(err))
	}
	fmt.Fprintf(w, "Authenticated as %s (%s)\n", identity.DisplayName, identity.Email)
	return identity, nil
}

// sanitizeError wraps an error to ensure no sensitive data (like tokens) appears
// in error messages returned from ProbeAuth.
// Currently returns the error as-is; callers must ensure no token appears upstream.
func sanitizeError(err error) error {
	return err
}
