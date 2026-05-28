package claudecli

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// PromptData is the template execution context for starter_prompt_template.
// All fields are read-only — do not add pointer fields.
type PromptData struct {
	Ticket       tracker.Ticket   // tracker-owned fields (Key, Summary, Status, URL, …)
	Repo         config.RepoEntry // the repo bound to this activation
	WorktreePath string           // absolute path to the created worktree
	ApproachName string           // the (unsanitized) approach name for display
}

// RenderPrompt executes tmplStr (Go text/template) with data.
// Returns the rendered string or a wrapped error if the template is invalid
// or execution fails. The rendered string is safe to pass directly as the
// final argv token to claude --bg (no shell quoting is needed because
// exec.Command takes it as a single argument, not a shell string).
func RenderPrompt(tmplStr string, data PromptData) (string, error) {
	t, err := template.New("starter").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse starter_prompt_template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render starter_prompt_template: %w", err)
	}
	return buf.String(), nil
}
