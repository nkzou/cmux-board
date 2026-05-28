package jira

import (
	"context"
	"fmt"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// WhoAmI validates acli authentication and returns the authenticated user's identity.
// Uses 'acli jira auth status' — output is text, not JSON.
//
// Authenticated output format (observed from acli 1.3.18):
//
//	✓ Authenticated
//	  Site: datadoghq.atlassian.net
//	  Email: user@example.com
//	  Authentication Type: oauth_global
//
// Unauthenticated output (stderr):
//
//	✗ Error: unauthorized: use 'acli jira auth login' to authenticate
//
// Note: acli exits 0 in both cases. We detect auth state by parsing the text.
func (a *JiraAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	stdout, stderr, exitCode, err := a.runner(ctx, "jira", "auth", "status")
	if err != nil {
		return tracker.UserIdentity{}, fmt.Errorf("WhoAmI: failed to run acli: %w", err)
	}

	stderrStr := strings.TrimSpace(string(stderr))
	stdoutStr := strings.TrimSpace(string(stdout))

	// Check stderr for auth errors (acli writes error to stderr even with exit 0).
	if isAuthStderr(stderrStr) {
		return tracker.UserIdentity{}, &errAuth{
			msg: "not authenticated — run 'acli jira auth login --web'",
		}
	}

	// Also check stdout for error text (some versions write to stdout).
	if isAuthStderr(stdoutStr) {
		return tracker.UserIdentity{}, &errAuth{
			msg: "not authenticated — run 'acli jira auth login --web'",
		}
	}

	_ = exitCode

	// Parse the text output.
	// Expected lines: "✓ Authenticated", "  Site: <site>", "  Email: <email>", ...
	if !strings.Contains(stdoutStr, "Authenticated") {
		return tracker.UserIdentity{}, &errAuth{
			msg: "not authenticated — run 'acli jira auth login --web'",
		}
	}

	identity := tracker.UserIdentity{}
	for _, line := range strings.Split(stdoutStr, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := cutPrefix(line, "Email:"); ok {
			identity.Email = strings.TrimSpace(after)
		}
		if after, ok := cutPrefix(line, "Site:"); ok {
			identity.ID = strings.TrimSpace(after) // use site as ID stand-in
		}
	}

	// If no email found, treat as unauthenticated.
	if identity.Email == "" && identity.ID == "" {
		return tracker.UserIdentity{}, &errAuth{
			msg: "not authenticated — run 'acli jira auth login --web'",
		}
	}

	identity.DisplayName = identity.Email
	return identity, nil
}

// cutPrefix returns s with the leading prefix removed and true if s starts with prefix.
// Replaces strings.CutPrefix for Go versions that don't have it.
func cutPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return s, false
}
