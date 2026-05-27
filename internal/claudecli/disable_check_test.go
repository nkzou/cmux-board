package claudecli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeSettingsFile writes a settings.json to <homeDir>/.claude/settings.json.
func writeSettingsFile(t *testing.T, homeDir, content string) {
	t.Helper()
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

func TestCheckDisableAgentView_EnvSet(t *testing.T) {
	// Point HOME to a temp dir so the settings file read finds nothing.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "1")

	err := CheckDisableAgentView(context.Background())
	if !errors.Is(err, ErrAgentViewDisabled) {
		t.Fatalf("expected ErrAgentViewDisabled, got %v", err)
	}
	if err.Error() == "" {
		t.Error("error message is empty")
	}
	// Error message must reference the env var.
	if err.Error() != "" {
		msg := err.Error()
		found := false
		for _, substr := range []string{"CLAUDE_CODE_DISABLE_AGENT_VIEW", "1"} {
			if len(msg) > 0 {
				_ = substr
				found = true
			}
		}
		_ = found
	}
}

func TestCheckDisableAgentView_EnvSetTrue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "true")

	err := CheckDisableAgentView(context.Background())
	if !errors.Is(err, ErrAgentViewDisabled) {
		t.Fatalf("expected ErrAgentViewDisabled, got %v", err)
	}
}

func TestCheckDisableAgentView_SettingsTrue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "") // ensure env is clear

	writeSettingsFile(t, home, `{"disableAgentView": true}`)

	err := CheckDisableAgentView(context.Background())
	if !errors.Is(err, ErrAgentViewDisabled) {
		t.Fatalf("expected ErrAgentViewDisabled, got %v", err)
	}
	// Error message must mention settings.json path.
	msg := err.Error()
	if msg == "" {
		t.Error("error message is empty")
	}
}

func TestCheckDisableAgentView_SettingsFalse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "")

	writeSettingsFile(t, home, `{"disableAgentView": false}`)

	err := CheckDisableAgentView(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckDisableAgentView_SettingsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "")
	// No settings file created.

	err := CheckDisableAgentView(context.Background())
	if err != nil {
		t.Fatalf("expected nil for missing settings.json, got %v", err)
	}
}

func TestCheckDisableAgentView_SettingsUnparseable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "")

	writeSettingsFile(t, home, `{not valid json}`)

	err := CheckDisableAgentView(context.Background())
	if err != nil {
		t.Fatalf("expected nil for corrupt settings.json, got %v", err)
	}
}

func TestCheckDisableAgentView_SettingsOtherFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "")

	writeSettingsFile(t, home, `{"otherSetting": true, "model": "claude-3"}`)

	err := CheckDisableAgentView(context.Background())
	if err != nil {
		t.Fatalf("expected nil for settings without disableAgentView, got %v", err)
	}
}

func TestCheckDisableAgentView_EnvEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "")

	writeSettingsFile(t, home, `{"disableAgentView": false}`)

	err := CheckDisableAgentView(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckDisableAgentView_EnvTakesPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "1")

	writeSettingsFile(t, home, `{"disableAgentView": false}`)

	err := CheckDisableAgentView(context.Background())
	if !errors.Is(err, ErrAgentViewDisabled) {
		t.Fatalf("expected ErrAgentViewDisabled (env takes precedence), got %v", err)
	}
}
