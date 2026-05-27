package jira

import (
	"github.com/nkzou/cmux-board/internal/tracker"
)

// Credentials holds configuration for the Jira adapter.
// Auth is owned by acli; Email and APIToken are not used.
type Credentials struct {
	Site     string // e.g. "datadog.atlassian.net" (no scheme, no trailing slash)
	ACLIPath string // optional path override; defaults to "acli" on PATH
}

// JiraAdapter implements tracker.IssueTracker for Jira Cloud via acli shellout.
// Construct via NewJiraAdapter; do not create directly.
type JiraAdapter struct {
	creds  Credentials
	runner Runner
}

// NewJiraAdapter creates a new JiraAdapter with the given credentials.
func NewJiraAdapter(creds Credentials) *JiraAdapter {
	path := creds.ACLIPath
	if path == "" {
		path = "acli"
	}
	return &JiraAdapter{
		creds:  creds,
		runner: newDefaultRunner(path),
	}
}

// Capabilities returns the feature flags for the Jira adapter.
func (a *JiraAdapter) Capabilities() tracker.Capabilities {
	return tracker.Capabilities{
		HasAssignees:                 true,
		HasLabels:                    true,
		HasPriority:                  true,
		RequiresWorkflowID:           true,
		SupportsBoardSummary:         true,
		TransitionFromStatusRequired: true,
	}
}

// compile-time assertion: JiraAdapter satisfies tracker.IssueTracker.
var _ tracker.IssueTracker = (*JiraAdapter)(nil)
