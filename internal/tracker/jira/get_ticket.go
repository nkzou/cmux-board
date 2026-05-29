package jira

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// GetTicket fetches a single ticket by key using 'acli jira workitem view KEY --json'.
// It maps the result through acliIssueToTicket, which handles all optional fields.
func (a *JiraAdapter) GetTicket(ctx context.Context, key string) (tracker.Ticket, error) {
	stdout, stderr, exitCode, err := a.runner(ctx,
		"jira", "workitem", "view", key, "--json",
	)
	if err != nil {
		return tracker.Ticket{}, fmt.Errorf("GetTicket: failed to run acli: %w", err)
	}

	if ferr := mapACLIError(stdout, stderr, exitCode, "GetTicket"); ferr != nil {
		return tracker.Ticket{}, ferr
	}

	// acliWorkitemViewResult only has status; use acliIssue which has the full field set.
	var iss acliIssue
	if err := json.Unmarshal(stdout, &iss); err != nil {
		return tracker.Ticket{}, fmt.Errorf("GetTicket: failed to parse acli output: %w", err)
	}

	return acliIssueToTicket(iss, a.cfg.Site), nil
}
