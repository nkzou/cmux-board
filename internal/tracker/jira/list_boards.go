package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// boardsResponse is the Jira /rest/agile/1.0/board paginated response shape.
type boardsResponse struct {
	StartAt    int          `json:"startAt"`
	MaxResults int          `json:"maxResults"`
	Total      int          `json:"total"`
	IsLast     bool         `json:"isLast"`
	Values     []boardEntry `json:"values"`
}

// boardEntry is a single board in the ListBoards response.
type boardEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

const listBoardsPageSize = 50

// ListBoards returns all boards accessible to the authenticated user.
// Paginates using startAt until isLast == true or all boards are fetched.
// Endpoint: GET /rest/agile/1.0/board?startAt=<n>&maxResults=50
func (a *JiraAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	var results []tracker.BoardSummary
	startAt := 0

	for {
		path := fmt.Sprintf("/rest/agile/1.0/board?startAt=%d&maxResults=%d", startAt, listBoardsPageSize)
		resp, err := a.client.Do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list boards: %w", err)
		}

		body, err := classifyResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to list boards: %w", err)
		}

		var page boardsResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("failed to decode ListBoards response: %w", err)
		}

		for _, e := range page.Values {
			results = append(results, tracker.BoardSummary{
				ID:   strconv.Itoa(e.ID),
				Name: e.Name,
				Type: e.Type,
			})
		}

		// Stop when isLast is set OR when we've seen all boards (belt-and-suspenders).
		if page.IsLast || startAt+len(page.Values) >= page.Total {
			break
		}
		startAt += len(page.Values)
	}

	return results, nil
}
