package jira

import (
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// JiraAdapter implements tracker.IssueTracker for Jira Cloud.
// Construct via NewJiraAdapter; do not create directly.
type JiraAdapter struct {
	client *jiraClient
}

// NewJiraAdapter creates a new JiraAdapter with the given credentials.
func NewJiraAdapter(creds Credentials) *JiraAdapter {
	return &JiraAdapter{client: newClient(creds)}
}

// newJiraAdapterWithClient constructs an adapter with an injected client (for testing).
func newJiraAdapterWithClient(client *jiraClient) *JiraAdapter {
	return &JiraAdapter{client: client}
}

// Capabilities returns the feature flags for the Jira adapter.
// All six flags are true for v1 Jira Cloud.
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

// GetBoard is implemented in get_board.go (T-021).

// ListTickets is implemented in list_tickets.go (T-022).

// TransitionStatus is implemented in transition.go (T-023).

// compile-time assertion: JiraAdapter satisfies tracker.IssueTracker.
var _ tracker.IssueTracker = (*JiraAdapter)(nil)

