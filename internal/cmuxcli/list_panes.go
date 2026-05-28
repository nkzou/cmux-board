package cmuxcli

import (
	"context"
	"encoding/json"
	"fmt"
)

// listPanesResponse is the top-level JSON envelope from `cmux --json list-panes`.
// Inferred JSON shape (cmux snake_case convention; RESEARCH §7 general):
//
//	{
//	  "panes": [
//	    { "ref": "pane:7", "workspace_ref": "workspace:4", "index": 0, "active": true },
//	    { "ref": "pane:8", "workspace_ref": "workspace:4", "index": 1, "active": false }
//	  ]
//	}
type listPanesResponse struct {
	Panes []Pane `json:"panes"`
}

// ListPanes calls `cmux --json list-panes --workspace <wsRef>` and returns the pane list.
//
// Exact argv: cmux --json list-panes --workspace <wsRef>
//
// Primary use: resolve agent_pane_ref after NewWorkspaceWithLayout.
// The agent pane is always at index 0 (left side of the horizontal split); callers
// MUST call AgentPaneRef to obtain it rather than hard-coding the index.
func ListPanes(ctx context.Context, wsRef string) ([]Pane, error) {
	if wsRef == "" {
		return nil, fmt.Errorf("ListPanes: wsRef must not be empty")
	}
	stdout, _, err := runCmux(ctx, "--json", "list-panes", "--workspace", wsRef)
	if err != nil {
		return nil, fmt.Errorf("cmux list-panes failed (workspace=%s): %w", wsRef, err)
	}
	var resp listPanesResponse
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse cmux list-panes output: %w", err)
	}
	return resp.Panes, nil
}

// AgentPaneRef returns the ref of the agent (index 0) pane from the provided list.
// Returns an error when the list is empty or no pane at index 0 exists.
// This is the pane that runs `claude attach <short_id>` and must be stored in
// state.activations[*].agent_pane_ref for subsequent FocusPane calls.
func AgentPaneRef(panes []Pane) (string, error) {
	if len(panes) == 0 {
		return "", fmt.Errorf("list-panes returned empty pane list; cannot resolve agent_pane_ref")
	}
	for _, p := range panes {
		if p.Index == 0 {
			return p.Ref, nil
		}
	}
	return "", fmt.Errorf("no pane at index 0 in workspace (got %d panes); cannot resolve agent_pane_ref",
		len(panes))
}
