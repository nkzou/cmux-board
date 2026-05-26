package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// issuesResponse is the Jira /rest/agile/1.0/board/{id}/issue paginated response shape.
type issuesResponse struct {
	StartAt    int          `json:"startAt"`
	MaxResults int          `json:"maxResults"`
	Total      int          `json:"total"`
	Issues     []issueEntry `json:"issues"`
}

// issueEntry is a single issue in the ListTickets response.
type issueEntry struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields issueFields `json:"fields"`
}

// issueFields contains the normalized fields returned by the restricted field set.
type issueFields struct {
	Summary  string          `json:"summary"`
	Status   issueStatus     `json:"status"`
	Assignee *issueAssignee  `json:"assignee"`
	Labels   []string        `json:"labels"`
	Priority *issuePriority  `json:"priority"`
	Updated  time.Time       `json:"updated"`
}

type issueStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type issueAssignee struct {
	AccountID string `json:"accountId"`
}

type issuePriority struct {
	Name string `json:"name"`
}

const listTicketsPageSize = 50

// ListTickets fetches all tickets for a board, optionally filtered by an update time.
// The `since` parameter adds a JQL `updated > "..."` filter when non-nil.
// Endpoint: GET /rest/agile/1.0/board/{boardID}/issue?startAt=<n>&maxResults=50&fields=...
func (a *JiraAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	var results []tracker.Ticket
	startAt := 0
	const fields = "summary,status,assignee,labels,priority,updated"

	for {
		path := fmt.Sprintf(
			"/rest/agile/1.0/board/%s/issue?startAt=%d&maxResults=%d&fields=%s",
			boardID, startAt, listTicketsPageSize, fields,
		)
		if since != nil {
			// Format truncated to minute per task spec; URL-encode for safe embedding.
			jql := fmt.Sprintf(`updated > "%s"`, since.UTC().Format("2006-01-02T15:04"))
			path += "&jql=" + url.QueryEscape(jql)
		}

		resp, err := a.client.Do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list tickets for board %s: %w", boardID, err)
		}

		body, err := classifyResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to list tickets for board %s: %w", boardID, err)
		}

		var page issuesResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("failed to decode ListTickets response for board %s: %w", boardID, err)
		}

		for _, iss := range page.Issues {
			results = append(results, tracker.Ticket{
				ID:         iss.ID,
				Key:        iss.Key,
				Summary:    iss.Fields.Summary,
				Status:     iss.Fields.Status.Name, // human name, used as last_known_status
				URL:        "https://" + a.client.creds.Site + "/browse/" + iss.Key,
				AssigneeID: assigneeID(iss.Fields.Assignee),
				Labels:     iss.Fields.Labels,
				Priority:   priorityName(iss.Fields.Priority),
				UpdatedAt:  iss.Fields.Updated,
				Raw:        map[string]any{},
			})
		}

		// Pagination: no isLast field on this endpoint — use total guard only.
		if startAt+len(page.Issues) >= page.Total {
			break
		}
		startAt += len(page.Issues)
	}

	return results, nil
}

// assigneeID safely dereferences a nullable Assignee pointer.
func assigneeID(a *issueAssignee) string {
	if a == nil {
		return ""
	}
	return a.AccountID
}

// priorityName safely dereferences a nullable Priority pointer.
func priorityName(p *issuePriority) string {
	if p == nil {
		return ""
	}
	return p.Name
}
