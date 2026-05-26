package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// boardConfigResponse is the Jira /rest/agile/1.0/board/{id}/configuration response shape.
type boardConfigResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	ColumnConfig struct {
		Columns []boardConfigColumn `json:"columns"`
	} `json:"columnConfig"`
}

// boardConfigColumn is a single column entry from the board configuration.
type boardConfigColumn struct {
	Name     string              `json:"name"`
	Statuses []boardConfigStatus `json:"statuses"`
}

// boardConfigStatus is a status entry within a column.
type boardConfigStatus struct {
	ID   string `json:"id"`
	Self string `json:"self"`
}

// GetBoard fetches the board configuration including column layout and status mappings.
// Each column's ID is derived by slugifying the column name.
// Endpoint: GET /rest/agile/1.0/board/{boardID}/configuration
func (a *JiraAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%s/configuration", boardID)
	resp, err := a.client.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return tracker.Board{}, fmt.Errorf("failed to get board %s: %w", boardID, err)
	}

	body, err := classifyResponse(resp)
	if err != nil {
		return tracker.Board{}, fmt.Errorf("failed to get board %s: %w", boardID, err)
	}

	var config boardConfigResponse
	if err := json.Unmarshal(body, &config); err != nil {
		return tracker.Board{}, fmt.Errorf("failed to decode GetBoard response for %s: %w", boardID, err)
	}

	columns := make([]tracker.Column, 0, len(config.ColumnConfig.Columns))
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

	return tracker.Board{
		ID:      strconv.Itoa(config.ID),
		Name:    config.Name,
		Columns: columns,
	}, nil
}
