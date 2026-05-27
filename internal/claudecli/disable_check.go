package claudecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrAgentViewDisabled is returned when claude's agent view is disabled via env or settings.
var ErrAgentViewDisabled = errors.New("claude agent view is disabled")

// claudeSettingsPath returns the default user-level settings file path.
func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// claudeSettings is the minimal shape we care about in ~/.claude/settings.json.
// We only read disableAgentView; all other fields are ignored.
type claudeSettings struct {
	DisableAgentView bool `json:"disableAgentView"`
}

// CheckDisableAgentView returns ErrAgentViewDisabled if the agent view is disabled via
// either the CLAUDE_CODE_DISABLE_AGENT_VIEW environment variable or the
// disableAgentView: true setting in ~/.claude/settings.json.
//
// A missing or unreadable settings.json is silently ignored (not an error);
// only a parseable file with disableAgentView:true triggers the refusal.
func CheckDisableAgentView(_ context.Context) error {
	// Signal 1: environment variable.
	if v := os.Getenv("CLAUDE_CODE_DISABLE_AGENT_VIEW"); v != "" {
		return fmt.Errorf("%w: CLAUDE_CODE_DISABLE_AGENT_VIEW=%s is set — "+
			"unset it to allow cmux-board to use the Claude agent view", ErrAgentViewDisabled, v)
	}

	// Signal 2: ~/.claude/settings.json disableAgentView: true.
	data, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		// Missing or unreadable settings.json: not an error for this check.
		return nil
	}
	var settings claudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		// Unparseable settings.json: don't block startup on a corrupt file.
		return nil
	}
	if settings.DisableAgentView {
		return fmt.Errorf("%w: disableAgentView:true found in ~/.claude/settings.json — "+
			"set it to false or remove it to allow cmux-board to use the Claude agent view",
			ErrAgentViewDisabled)
	}
	return nil
}
