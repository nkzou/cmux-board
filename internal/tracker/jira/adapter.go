package jira

import (
	v3 "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/ctreminiom/go-atlassian/v2/jira/agile"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// JiraAdapter implements tracker.IssueTracker for Jira Cloud via go-atlassian.
// Construct via NewJiraAdapter; do not create directly.
type JiraAdapter struct {
	v3    *v3.Client
	agile *agile.Client
	creds Credentials
}

// NewJiraAdapter creates a new JiraAdapter with the given credentials.
func NewJiraAdapter(creds Credentials) *JiraAdapter {
	siteURL := "https://" + creds.Site

	v3c, _ := v3.New(nil, siteURL)
	v3c.Auth.SetBasicAuth(creds.Email, creds.APIToken)

	agileC, _ := agile.New(nil, siteURL)
	agileC.Auth.SetBasicAuth(creds.Email, creds.APIToken)

	return &JiraAdapter{v3: v3c, agile: agileC, creds: creds}
}

// newJiraAdapterWithClients constructs an adapter with injected clients (for testing).
func newJiraAdapterWithClients(v3c *v3.Client, agileC *agile.Client, creds Credentials) *JiraAdapter {
	return &JiraAdapter{v3: v3c, agile: agileC, creds: creds}
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
