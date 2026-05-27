package claudecli

import "fmt"

// BuildAttachCommand returns the shell command string that the cmux workspace layout
// JSON injects into the agent pane terminal. cmux sends this as keystrokes (with Enter)
// to the terminal surface; it is NOT executed by cmux-board's own process.
//
// The short_id is the 8-hex-char identifier captured from `claude --bg` stdout
// (stored as ActivationEntry.ClaudeShortID).
//
// Example return value: "claude attach 3174068b"
func BuildAttachCommand(shortID string) string {
	return fmt.Sprintf("claude attach %s", shortID)
}
