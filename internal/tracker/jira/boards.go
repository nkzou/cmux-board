package jira

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/nkzou/cmux-board/internal/tracker"
)

const listBoardsPageSize = 50

// ListBoards returns all boards accessible to the authenticated user.
// Paginates using startAt until isLast == true or all boards fetched.
func (a *JiraAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	var results []tracker.BoardSummary
	startAt := 0

	for {
		slog.Debug("jira request", "method", "GET", "path", "/rest/agile/1.0/board")

		page, resp, err := a.agile.Board.Gets(ctx, &model.GetBoardsOptions{}, startAt, listBoardsPageSize)
		if err != nil {
			if resp != nil {
				return nil, fmt.Errorf("failed to list boards: %w", mapResponseError(resp))
			}
			return nil, fmt.Errorf("failed to list boards: %w", err)
		}

		slog.Debug("jira response", "status", resp.Code)

		for _, b := range page.Values {
			results = append(results, tracker.BoardSummary{
				ID:   strconv.Itoa(b.ID),
				Name: b.Name,
				Type: b.Type,
			})
		}

		// Stop when isLast is set OR when we've seen all boards.
		if page.IsLast || startAt+len(page.Values) >= page.Total {
			break
		}
		startAt += len(page.Values)
	}

	return results, nil
}

// GetBoard fetches the board configuration including column layout and status mappings.
// Uses go-atlassian Board.Configuration (GET /rest/agile/1.0/board/{id}/configuration).
func (a *JiraAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	id, err := strconv.Atoi(boardID)
	if err != nil {
		return tracker.Board{}, fmt.Errorf("invalid boardID %q: %w", boardID, err)
	}

	slog.Debug("jira request", "method", "GET", "path", "/rest/agile/1.0/board/"+boardID+"/configuration")

	config, resp, err := a.agile.Board.Configuration(ctx, id)
	if err != nil {
		if resp != nil {
			return tracker.Board{}, fmt.Errorf("failed to get board %s: %w", boardID, mapResponseError(resp))
		}
		return tracker.Board{}, fmt.Errorf("failed to get board %s: %w", boardID, err)
	}

	slog.Debug("jira response", "status", resp.Code)

	var columns []tracker.Column
	if config.ColumnConfig != nil {
		columns = make([]tracker.Column, 0, len(config.ColumnConfig.Columns))
		for _, col := range config.ColumnConfig.Columns {
			var statusIDs []string
			for _, s := range col.Statuses {
				statusIDs = append(statusIDs, s.ID)
			}
			columns = append(columns, tracker.Column{
				ID:        Slug(col.Name),
				Name:      col.Name,
				StatusIDs: statusIDs,
			})
		}
	}

	return tracker.Board{
		ID:      strconv.Itoa(config.ID),
		Name:    config.Name,
		Columns: columns,
	}, nil
}
