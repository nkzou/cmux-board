package cmuxcli

import (
	"context"
	"encoding/json"
	"fmt"
)

// listWorkspacesResponse is the top-level JSON envelope from `cmux --json list-workspaces`.
// JSON shape (RESEARCH §7):
//
//	{
//	  "window_ref": "window:1",
//	  "workspaces": [
//	    { "ref": "workspace:1", "title": "...", "current_directory": "...", ... }
//	  ]
//	}
type listWorkspacesResponse struct {
	WindowRef  string      `json:"window_ref"`
	Workspaces []Workspace `json:"workspaces"`
}

// ListWorkspaces calls `cmux --json list-workspaces` and returns the workspace list.
//
// Exact argv: cmux --json list-workspaces
//
// Usage rules (callers MUST follow):
//   - Use Workspace.Ref for orphan detection: if a cached cmux_workspace_id is absent from
//     the returned slice, mark the activation cmux_orphan=true.
//   - Workspace.CurrentDirectory is available for diagnostic logging ONLY.
//     NEVER adopt a workspace based on CurrentDirectory matching a cached worktree path.
//     Foreign-workspace adoption by current_directory heuristic is forbidden (E8,
//     Codex Finding 1, CONVENTIONS.md "Foreign cmux workspaces are never adopted").
//   - Workspace.Title may be used to match act_id_short substrings for incomplete-activation
//     reconciliation (F-NEW3) — this is name-based, not path-based.
func ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	stdout, _, err := runCmux(ctx, "--json", "list-workspaces")
	if err != nil {
		return nil, fmt.Errorf("cmux list-workspaces failed: %w", err)
	}
	var resp listWorkspacesResponse
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse cmux list-workspaces output: %w", err)
	}
	return resp.Workspaces, nil
}

// IsOrphan returns true when wsRef is not present in the provided workspace list.
// This is the ONLY correct way to detect a cmux_orphan condition: look up by ref,
// never by current_directory match (E8 / Codex Finding 1).
func IsOrphan(wsRef string, workspaces []Workspace) bool {
	for _, ws := range workspaces {
		if ws.Ref == wsRef {
			return false
		}
	}
	return true
}
