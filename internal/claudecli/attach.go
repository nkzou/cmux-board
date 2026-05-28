package claudecli

import "fmt"

// BuildAttachCommand returns the shell command string that the cmux workspace layout
// JSON injects into the agent pane terminal. cmux sends this as keystrokes (with Enter)
// to the terminal surface; it is NOT executed by cmux-board's own process.
//
// The short_id is the 8-hex-char identifier captured from `claude --bg` stdout
// (stored as ActivationEntry.ClaudeShortID).
//
// CMUX_CLAUDE_HOOKS_DISABLED=1 bypasses the cmux claude wrapper. The wrapper's
// builtin-subcommand list (agents|auth|...) does not include 'attach', so it
// would otherwise inject '--session-id NEW_UUID --settings ...' before our
// argv. Claude's commander then no longer parses 'attach' as a subcommand —
// it treats 'attach <id>' as the [prompt] positional and opens a fresh
// interactive session with 'attach' as the first user message. The
// CMUX_CLAUDE_HOOKS_DISABLED=1 prefix makes the wrapper exec the real claude
// unchanged, so 'claude attach <id>' attaches to the background session as
// intended.
//
// Example return value: "CMUX_CLAUDE_HOOKS_DISABLED=1 claude attach 3174068b"
func BuildAttachCommand(shortID string) string {
	return fmt.Sprintf("CMUX_CLAUDE_HOOKS_DISABLED=1 claude attach %s", shortID)
}
