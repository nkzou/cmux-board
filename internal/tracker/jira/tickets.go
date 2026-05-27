package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// acliIssue is the JSON shape of a single issue in 'acli jira workitem search --json'.
// Observed from acli 1.3.18: returns an array of these, NOT {issues: [...]}.
// Available fields in JSON mode: assignee, issuetype, priority, status, summary.
// Note: labels and updated are NOT returned by acli's JSON output.
type acliIssue struct {
	ID     string          `json:"id"`
	Key    string          `json:"key"`
	Fields acliIssueFields `json:"fields"`
}

type acliIssueFields struct {
	Summary  string       `json:"summary"`
	Status   *acliStatus  `json:"status"`
	Assignee *acliUser    `json:"assignee"`
	Priority *acliPriority `json:"priority"`
}

type acliStatus struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type acliUser struct {
	AccountID string `json:"accountId"`
	Email     string `json:"emailAddress"`
}

type acliPriority struct {
	Name string `json:"name"`
}

// locationProjectKeyRe extracts the project key from the location string "Name (KEY)".
var locationProjectKeyRe = regexp.MustCompile(`\(([A-Z][A-Z0-9_]+)\)\s*$`)

// ListTickets fetches all tickets for a board using JQL.
// Calls 'acli jira workitem search --json --paginate --jql "..."'.
//
// DEVIATION from brief: acli 'board get' does not expose a filterId, so we cannot
// use '--filter N'. Instead, the board's project key is resolved via 'board search'
// (location field contains "ProjectName (KEY)"), and JQL filters by project.
// If since is non-nil, adds 'AND updated >= "..."' to the JQL.
//
// DEVIATION: acli 1.3.18 workitem search JSON returns only 5 fields (assignee,
// issuetype, priority, status, summary). Labels and UpdatedAt are always empty.
func (a *JiraAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	// Resolve project key for this board.
	projectKey, err := a.resolveProjectKey(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("ListTickets: %w", err)
	}

	jql := fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey)
	if since != nil {
		jql = fmt.Sprintf("project = %s AND updated >= %q ORDER BY updated DESC",
			projectKey, since.UTC().Format("2006-01-02 15:04"))
	}

	stdout, stderr, exitCode, err := a.runner(ctx,
		"jira", "workitem", "search",
		"--json", "--paginate",
		"--jql", jql,
	)
	if err != nil {
		return nil, fmt.Errorf("ListTickets: failed to run acli: %w", err)
	}

	if ferr := mapACLIError(stdout, stderr, exitCode, "ListTickets"); ferr != nil {
		return nil, ferr
	}

	var issues []acliIssue
	if err := json.Unmarshal(stdout, &issues); err != nil {
		return nil, fmt.Errorf("ListTickets: failed to parse acli output: %w", err)
	}

	tickets := make([]tracker.Ticket, 0, len(issues))
	for _, iss := range issues {
		tickets = append(tickets, acliIssueToTicket(iss, a.creds.Site))
	}
	return tickets, nil
}

// resolveProjectKey finds the project key for the given boardID by calling board search
// and parsing the location string. The location format is "ProjectName (PROJKEY)".
func (a *JiraAdapter) resolveProjectKey(ctx context.Context, boardID string) (string, error) {
	stdout, stderr, exitCode, err := a.runner(ctx,
		"jira", "board", "search",
		"--json", "--paginate",
	)
	if err != nil {
		return "", fmt.Errorf("resolveProjectKey: failed to run acli: %w", err)
	}
	if ferr := mapACLIError(stdout, stderr, exitCode, "resolveProjectKey"); ferr != nil {
		return "", ferr
	}

	var result acliBoardSearchResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", fmt.Errorf("resolveProjectKey: failed to parse board search: %w", err)
	}

	for _, b := range result.Values {
		if fmt.Sprintf("%d", b.ID) == boardID {
			key := extractProjectKey(b.Location)
			if key != "" {
				return key, nil
			}
			// Fallback: if location doesn't match pattern, try board name.
			break
		}
	}
	return "", fmt.Errorf("resolveProjectKey: board %s not found in board search results", boardID)
}

// extractProjectKey parses the project key from the location string "ProjectName (KEY)".
func extractProjectKey(location string) string {
	m := locationProjectKeyRe.FindStringSubmatch(strings.TrimSpace(location))
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// acliIssueToTicket converts an acliIssue to our tracker.Ticket.
func acliIssueToTicket(iss acliIssue, site string) tracker.Ticket {
	var (
		statusName string
		assigneeID string
		priority   string
	)

	if iss.Fields.Status != nil {
		statusName = iss.Fields.Status.Name
	}
	if iss.Fields.Assignee != nil {
		assigneeID = iss.Fields.Assignee.AccountID
	}
	if iss.Fields.Priority != nil {
		priority = iss.Fields.Priority.Name
	}

	url := "https://" + site + "/browse/" + iss.Key

	return tracker.Ticket{
		ID:         iss.ID,
		Key:        iss.Key,
		Summary:    iss.Fields.Summary,
		Status:     statusName,
		URL:        url,
		AssigneeID: assigneeID,
		Labels:     nil,   // not available via acli workitem search JSON
		Priority:   priority,
		UpdatedAt:  time.Time{}, // not available via acli workitem search JSON
		Raw:        map[string]any{},
	}
}
