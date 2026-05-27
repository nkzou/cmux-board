package jira

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

const listTicketsPageSize = 50

// ListTickets fetches all tickets for a board, optionally filtered by update time.
// Uses go-atlassian Board.Issues (GET /rest/agile/1.0/board/{id}/issue).
func (a *JiraAdapter) ListTickets(ctx context.Context, boardID string, since *time.Time) ([]tracker.Ticket, error) {
	id, err := strconv.Atoi(boardID)
	if err != nil {
		return nil, fmt.Errorf("invalid boardID %q: %w", boardID, err)
	}

	var results []tracker.Ticket
	startAt := 0
	const fields = "summary,status,assignee,labels,priority,updated"

	opts := &model.IssueOptionScheme{
		Fields: []string{"summary", "status", "assignee", "labels", "priority", "updated"},
	}
	if since != nil {
		opts.JQL = fmt.Sprintf(`updated > "%s"`, since.UTC().Format("2006-01-02T15:04"))
	}

	for {
		slog.Debug("jira request", "method", "GET", "path", fmt.Sprintf("/rest/agile/1.0/board/%s/issue", boardID))

		page, resp, err := a.agile.Board.Issues(ctx, id, opts, startAt, listTicketsPageSize)
		if err != nil {
			if resp != nil {
				return nil, fmt.Errorf("failed to list tickets for board %s: %w", boardID, mapResponseError(resp))
			}
			return nil, fmt.Errorf("failed to list tickets for board %s: %w", boardID, err)
		}

		slog.Debug("jira response", "status", resp.Code)

		for _, iss := range page.Issues {
			results = append(results, issueV2ToTicket(iss, a.creds.Site))
		}

		// No isLast on this endpoint — use total guard.
		if startAt+len(page.Issues) >= page.Total {
			break
		}
		startAt += len(page.Issues)
	}

	_ = fields // satisfy compiler; opts.Fields drives field selection
	return results, nil
}

// issueV2ToTicket converts a go-atlassian IssueSchemeV2 to our tracker.Ticket.
func issueV2ToTicket(iss *model.IssueSchemeV2, site string) tracker.Ticket {
	var (
		summary    string
		statusName string
		assigneeID string
		labels     []string
		priority   string
		updatedAt  time.Time
	)

	if iss.Fields != nil {
		summary = iss.Fields.Summary
		if iss.Fields.Status != nil {
			statusName = iss.Fields.Status.Name
		}
		if iss.Fields.Assignee != nil {
			assigneeID = iss.Fields.Assignee.AccountID
		}
		labels = iss.Fields.Labels
		if iss.Fields.Priority != nil {
			priority = iss.Fields.Priority.Name
		}
		if iss.Fields.Updated != nil {
			updatedAt = time.Time(*iss.Fields.Updated)
		}
	}

	return tracker.Ticket{
		ID:         iss.ID,
		Key:        iss.Key,
		Summary:    summary,
		Status:     statusName,
		URL:        "https://" + site + "/browse/" + iss.Key,
		AssigneeID: assigneeID,
		Labels:     labels,
		Priority:   priority,
		UpdatedAt:  updatedAt,
		Raw:        map[string]any{},
	}
}
