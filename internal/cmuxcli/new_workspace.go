package cmuxcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// NewWorkspaceArgs holds the parameters for creating a new workspace with a two-pane layout.
type NewWorkspaceArgs struct {
	// Name is the display title for the new workspace (e.g. "PROJ-42 [abc12345]").
	Name string
	// CWD is the working directory for the shell pane (the worktree path).
	CWD string
	// AgentAttachCommand is the command run in the agent (left) pane,
	// e.g. "claude attach 7c5dcf5d".
	AgentAttachCommand string
	// ShellCommand is the command run in the worktree shell (right) pane.
	// Defaults to an empty string (opens the default shell).
	ShellCommand string
	// Focus controls whether the new workspace is immediately focused. Default false.
	Focus bool
}

// NewWorkspaceWithLayout calls `cmux new-workspace` with an inline two-pane layout JSON
// and returns the workspace ref string (e.g. "workspace:4").
//
// Exact argv:
//
//	cmux new-workspace --name <name> --cwd <cwd> --layout <json> [--focus true]
//
// Layout JSON structure (RESEARCH §7 "cmux new-workspace --layout"):
//
//	{
//	  "direction": "horizontal",
//	  "split": 0.5,
//	  "children": [
//	    { "pane": { "surfaces": [{"type":"terminal","command":"claude attach <short_id>"}] } },
//	    { "pane": { "surfaces": [{"type":"terminal","command":""}] } }
//	  ]
//	}
//
// stdout response (RESEARCH §7): `OK workspace:4` — a single plain-text line.
// The token after "OK " is the workspace ref.
//
// ADDITIVE-ONLY: this function creates a new workspace. It never closes, replaces, or
// mutates an existing workspace. The caller (T-062 activation orchestrator) is responsible
// for verifying path uniqueness BEFORE calling here.
func NewWorkspaceWithLayout(ctx context.Context, args NewWorkspaceArgs) (string, error) {
	layoutJSON, err := buildLayoutJSON(args.AgentAttachCommand, args.ShellCommand)
	if err != nil {
		return "", fmt.Errorf("failed to build layout JSON: %w", err)
	}

	cmdArgs := []string{
		"new-workspace",
		"--name", args.Name,
		"--cwd", args.CWD,
		"--layout", layoutJSON,
	}
	if args.Focus {
		cmdArgs = append(cmdArgs, "--focus", "true")
	}

	stdout, _, err := runCmux(ctx, cmdArgs...)
	if err != nil {
		return "", fmt.Errorf("cmux new-workspace failed: %w", err)
	}

	ref, err := parseWorkspaceRef(strings.TrimSpace(string(stdout)))
	if err != nil {
		return "", fmt.Errorf("failed to parse cmux new-workspace output %q: %w",
			strings.TrimSpace(string(stdout)), err)
	}
	return ref, nil
}

// twoPane is the inline layout schema for a horizontal split with two terminal panes.
type twoPane struct {
	Direction string     `json:"direction"`
	Split     float64    `json:"split"`
	Children  []paneNode `json:"children"`
}

type paneNode struct {
	Pane paneBody `json:"pane"`
}

type paneBody struct {
	Surfaces []surfaceSpec `json:"surfaces"`
}

type surfaceSpec struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// buildLayoutJSON returns the inline layout JSON for a horizontal split:
// left pane runs agentCmd, right pane runs shellCmd (empty string = default shell).
func buildLayoutJSON(agentCmd, shellCmd string) (string, error) {
	layout := twoPane{
		Direction: "horizontal",
		Split:     0.5,
		Children: []paneNode{
			{Pane: paneBody{Surfaces: []surfaceSpec{{Type: "terminal", Command: agentCmd}}}},
			{Pane: paneBody{Surfaces: []surfaceSpec{{Type: "terminal", Command: shellCmd}}}},
		},
	}
	b, err := json.Marshal(layout)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseWorkspaceRef extracts the workspace ref from the `OK workspace:N` stdout line.
// Returns an error if the line does not start with "OK ".
func parseWorkspaceRef(line string) (string, error) {
	const prefix = "OK "
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unexpected new-workspace output (want \"OK <ref>\"): %q", line)
	}
	ref := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if ref == "" {
		return "", fmt.Errorf("empty workspace ref in output: %q", line)
	}
	return ref, nil
}
