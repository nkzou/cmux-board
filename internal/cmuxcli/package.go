// Package cmuxcli provides typed wrappers for the cmux CLI.
//
// Every public function issues exactly one cmux subcommand via os/exec.
// Argv is always constructed via exec.Command(name, args...) — never via string formatting.
// Stdout and stderr are captured separately. Errors include the exact argv.
//
// Additive-only constraint: this package creates and observes cmux workspaces but NEVER
// destroys, closes, or mutates existing ones beyond the allowed set:
//   - cmux new-workspace  (create)
//   - cmux new-pane       (create)
//   - cmux focus-pane     (focus)
//   - cmux set-status     (status pill)
//   - cmux list-workspaces / cmux list-panes / cmux identify (read-only)
//
// Never call: cmux workspace-action --action close-others, cmux workspace-action --action
// close-above, cmux workspace-action --action close-below, cmux close-window, cmux kill.
// (This comment is the ONE allowed match for the T-039 negative grep.)
package cmuxcli
