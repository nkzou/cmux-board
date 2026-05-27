package jira

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// acliBoardSearchResult is the JSON shape returned by 'acli jira board search --json'.
// Observed from acli 1.3.18 against datadoghq.atlassian.net.
type acliBoardSearchResult struct {
	IsLast     bool          `json:"isLast"`
	MaxResults int           `json:"maxResults"`
	StartAt    int           `json:"startAt"`
	Total      int           `json:"total"`
	Values     []acliBoardItem `json:"values"`
}

// acliBoardItem is one entry in the board search results.
// Note: location is a display string like "ProjectName (PROJKEY)", not a struct.
type acliBoardItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location"`
}

// acliBoardGetResult is the JSON shape returned by 'acli jira board get --id N --json'.
// Note: acli 1.3.18 does NOT return column config or filterId — only id/name/type/location/link.
type acliBoardGetResult struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location"`
	Link     string `json:"link"`
}

// ListBoards returns all boards accessible to the authenticated user.
// Calls 'acli jira board search --json --paginate'.
func (a *JiraAdapter) ListBoards(ctx context.Context) ([]tracker.BoardSummary, error) {
	stdout, stderr, exitCode, err := a.runner(ctx, "jira", "board", "search", "--json", "--paginate")
	if err != nil {
		return nil, fmt.Errorf("ListBoards: failed to run acli: %w", err)
	}

	if ferr := mapACLIError(stdout, stderr, exitCode, "ListBoards"); ferr != nil {
		return nil, ferr
	}

	var result acliBoardSearchResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("ListBoards: failed to parse acli output: %w", err)
	}

	boards := make([]tracker.BoardSummary, 0, len(result.Values))
	for _, b := range result.Values {
		boards = append(boards, tracker.BoardSummary{
			ID:   fmt.Sprintf("%d", b.ID),
			Name: b.Name,
			Type: b.Type,
		})
	}
	return boards, nil
}

// GetBoard fetches the board name from acli and merges it with the adapter's
// locally-stored column layout.
//
// Calls 'acli jira board get --id N --json' to get the board name.
// Columns come from a.cfg.Columns (set at construction from config.json
// adapter_config.columns). This avoids the acli limitation: acli 1.3.18
// 'board get --json' does not return column config or filterId.
//
// If a.cfg.Columns is empty, the returned Board has no columns and all
// tickets fall into the "Unmapped" column in the UI.
func (a *JiraAdapter) GetBoard(ctx context.Context, boardID string) (tracker.Board, error) {
	stdout, stderr, exitCode, err := a.runner(ctx, "jira", "board", "get", "--id", boardID, "--json")
	if err != nil {
		return tracker.Board{}, fmt.Errorf("GetBoard: failed to run acli: %w", err)
	}

	if ferr := mapACLIError(stdout, stderr, exitCode, "GetBoard"); ferr != nil {
		return tracker.Board{}, ferr
	}

	var result acliBoardGetResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return tracker.Board{}, fmt.Errorf("GetBoard: failed to parse acli output: %w", err)
	}

	if result.ID == 0 {
		return tracker.Board{}, &errFatal{msg: fmt.Sprintf("GetBoard: board %s not found", boardID)}
	}

	return tracker.Board{
		ID:      fmt.Sprintf("%d", result.ID),
		Name:    result.Name,
		Columns: a.cfg.Columns, // manually-configured; empty slice means "all in Unmapped"
	}, nil
}
