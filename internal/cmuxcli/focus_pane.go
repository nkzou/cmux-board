package cmuxcli

import (
	"context"
	"fmt"
)

// FocusPane calls `cmux focus-pane --workspace <wsRef> --pane <paneRef>` to bring the
// specified pane to the foreground.
//
// Exact argv: cmux focus-pane --workspace <wsRef> --pane <paneRef>
//
// This is the ONLY supported focus primitive in cmux 0.64.7. There is NO `cmux focus`
// subcommand — that form does not exist (RESEARCH §7). Every call site MUST pass the
// cached agent_pane_ref from state.activations[*].agent_pane_ref.
//
// Forbidden forms (Codex Finding 8 — regression guard in focus_pane_test.go):
//
//	cmux focus <workspace_id>            — subcommand does not exist (rejected by cmux)
//	cmux workspace-action --action focus — no focus action documented (rejected by cmux)
//
// wsRef:   the workspace ref, e.g. "workspace:4"
// paneRef: the agent pane ref, e.g. "pane:7" (from state.activations[*].agent_pane_ref)
func FocusPane(ctx context.Context, wsRef, paneRef string) error {
	if wsRef == "" {
		return fmt.Errorf("FocusPane: wsRef must not be empty")
	}
	if paneRef == "" {
		return fmt.Errorf("FocusPane: paneRef must not be empty")
	}
	_, _, err := runCmux(ctx, "focus-pane", "--workspace", wsRef, "--pane", paneRef)
	if err != nil {
		return fmt.Errorf("cmux focus-pane (workspace=%s, pane=%s) failed: %w",
			wsRef, paneRef, err)
	}
	return nil
}
