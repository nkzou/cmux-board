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
	Summary   string         `json:"summary"`
	Status    *acliStatus    `json:"status"`
	IssueType *acliIssueType `json:"issuetype"`
	Assignee  *acliUser      `json:"assignee"`
	Priority  *acliPriority  `json:"priority"`
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

type acliIssueType struct {
	Name string `json:"name"`
}

// locationProjectKeyRe extracts the project key from the location string "Name (KEY)".
var locationProjectKeyRe = regexp.MustCompile(`\(([A-Z][A-Z0-9_]+)\)\s*$`)

// listTicketsWindow is the maximum lookback for ticket search. Tickets older than
// this window are never fetched, regardless of poll cadence — they're effectively
// archived from the board's perspective.
const listTicketsWindow = 3 * 30 * 24 * time.Hour // ~3 calendar months

// ListTickets fetches all tickets for a board using JQL.
// Calls 'acli jira workitem search --json --paginate --jql "..."'.
//
// JQL always includes 'assignee = currentUser()' so the board only shows
// tickets owned by the authenticated user — cmux-board is a personal worktree
// dashboard, not a team-wide view.
//
// JQL always includes 'updated >= <cutoff>' where cutoff is the more recent of
// (since) and (now - 3 months). The 3-month floor keeps the result set bounded
// against long-lived projects with thousands of historical tickets.
//
// DEVIATION from brief: acli 'board get' does not expose a filterId, so we cannot
// use '--filter N'. Instead, the board's project key is resolved via 'board search'
// (location field contains "ProjectName (KEY)"), and JQL filters by project.
//
// DEVIATION: acli 1.3.18 workitem search JSON returns only 5 fields (assignee,
// issuetype, priority, status, summary). Labels and UpdatedAt are always empty.
func (a *JiraAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	// Resolve project key for this board.
	projectKey, err := a.resolveProjectKey(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("ListTickets: %w", err)
	}

	cutoff := time.Now().Add(-listTicketsWindow)
	if since != nil && since.After(cutoff) {
		cutoff = *since
	}
	jql := fmt.Sprintf("project = %s AND assignee = currentUser() AND updated >= %q ORDER BY updated DESC",
		projectKey, cutoff.UTC().Format("2006-01-02 15:04"))

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
		tickets = append(tickets, acliIssueToTicket(iss, a.cfg.Site))
	}
	return tickets, nil
}

// resolveProjectKey finds the project key for the given boardID by calling
// 'acli jira board get --id N --json' and parsing the location string
// ("ProjectName (PROJKEY)"). A direct lookup keeps us from paginating through
// every board on the account — the search call can take many seconds against
// instances with thousands of boards.
func (a *JiraAdapter) resolveProjectKey(ctx context.Context, boardID string) (string, error) {
	stdout, stderr, exitCode, err := a.runner(ctx,
		"jira", "board", "get",
		"--id", boardID,
		"--json",
	)
	if err != nil {
		return "", fmt.Errorf("resolveProjectKey: failed to run acli: %w", err)
	}
	if ferr := mapACLIError(stdout, stderr, exitCode, "resolveProjectKey"); ferr != nil {
		return "", ferr
	}

	var result acliBoardGetResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", fmt.Errorf("resolveProjectKey: failed to parse board get: %w", err)
	}
	if result.ID == 0 {
		return "", fmt.Errorf("resolveProjectKey: board %s not found", boardID)
	}

	key := extractProjectKey(result.Location)
	if key == "" {
		return "", fmt.Errorf("resolveProjectKey: board %s has no parseable project key in location %q",
			boardID, result.Location)
	}
	return key, nil
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
		statusName    string
		issueType     string
		assigneeID    string
		assigneeEmail string
		priority      string
	)

	if iss.Fields.Status != nil {
		statusName = iss.Fields.Status.Name
	}
	if iss.Fields.IssueType != nil {
		issueType = iss.Fields.IssueType.Name
	}
	if iss.Fields.Assignee != nil {
		assigneeID = iss.Fields.Assignee.AccountID
		assigneeEmail = iss.Fields.Assignee.Email
	}
	if iss.Fields.Priority != nil {
		priority = iss.Fields.Priority.Name
	}

	url := "https://" + site + "/browse/" + iss.Key

	return tracker.Ticket{
		ID:            iss.ID,
		Key:           iss.Key,
		Summary:       iss.Fields.Summary,
		Status:        statusName,
		URL:           url,
		IssueType:     issueType,
		AssigneeID:    assigneeID,
		AssigneeEmail: assigneeEmail,
		Labels:        nil, // not available via acli workitem search JSON
		Priority:      priority,
		UpdatedAt:     time.Time{}, // not available via acli workitem search JSON
		Raw:           map[string]any{},
	}
}
