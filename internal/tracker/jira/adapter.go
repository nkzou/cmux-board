package jira

import (
	"github.com/nkzou/cmux-board/internal/tracker"
)

// Config holds all configuration for the Jira adapter.
// Auth is owned by acli; no secrets are stored here.
type Config struct {
	// Site is used for URL construction (e.g. "datadog.atlassian.net").
	// No scheme, no trailing slash.
	Site string
	// ACLIPath is an optional override for the acli binary path.
	// Defaults to "acli" on PATH when empty.
	ACLIPath string
	// Columns is the manually-configured board column layout.
	// Each Column.StatusIDs holds status NAMES (not numeric IDs) because
	// acli works in names. The Resolve function compares case-insensitively.
	// Set once at adapter construction; not mutated after that.
	Columns []tracker.Column
}

// Credentials is a backward-compatible alias for Config.
// Deprecated: use Config directly. Callers in tests that reference Credentials
// directly should migrate to Config.
type Credentials = Config

// JiraAdapter implements tracker.IssueTracker for Jira Cloud via acli shellout.
// Construct via NewJiraAdapter; do not create directly.
type JiraAdapter struct {
	cfg    Config
	runner Runner
}

// NewJiraAdapter creates a new JiraAdapter with the given config.
func NewJiraAdapter(cfg Config) *JiraAdapter {
	path := cfg.ACLIPath
	if path == "" {
		path = "acli"
	}
	return &JiraAdapter{
		cfg:    cfg,
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
