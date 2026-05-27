package initwizard

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrintDockSnippet_OutputStructure(t *testing.T) {
	var buf bytes.Buffer
	PrintDockSnippet(&buf, "/home/user/.config/cmux-board")
	out := buf.String()
	if len(out) == 0 {
		t.Error("PrintDockSnippet produced empty output")
	}
	if !strings.Contains(out, "cmux-board") {
		t.Errorf("expected 'cmux-board' in output, got: %q", out)
	}
	if !strings.Contains(out, "dock.json") {
		t.Errorf("expected 'dock.json' hint in output, got: %q", out)
	}
}

func TestPrintDockSnippet_ConfigDirInOutput(t *testing.T) {
	var buf bytes.Buffer
	configDir := "/home/user/.config/cmux-board"
	PrintDockSnippet(&buf, configDir)
	out := buf.String()
	if !strings.Contains(out, configDir) {
		t.Errorf("expected configDir %q in output, got: %q", configDir, out)
	}
}

func TestPrintDockSnippet_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	PrintDockSnippet(&buf, "/tmp/cfg")
	out := buf.String()

	// Extract JSON between banners.
	lines := strings.Split(out, "\n")
	var jsonLines []string
	inJSON := false
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "---") {
			if inJSON {
				break
			}
			inJSON = true
			continue
		}
		if inJSON {
			jsonLines = append(jsonLines, l)
		}
	}
	jsonStr := strings.Join(jsonLines, "\n")
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		t.Fatal("no JSON content found between dock snippet banners")
	}
	// Strip comment lines before parsing.
	var cleanLines []string
	for _, l := range strings.Split(jsonStr, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			cleanLines = append(cleanLines, l)
		}
	}
	jsonPart := strings.Join(cleanLines, "\n")

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(jsonPart), &raw); err != nil {
		t.Errorf("dock snippet contains invalid JSON: %v\nContent:\n%s", err, jsonPart)
	}
}

func TestPrintTemplateHelp_CoversAllVariables(t *testing.T) {
	var buf bytes.Buffer
	PrintTemplateHelp(&buf)
	out := buf.String()

	expected := []string{
		"{{.Ticket.Key}}",
		"{{.Ticket.Summary}}",
		"{{.Ticket.Status}}",
		"{{.Ticket.URL}}",
		"{{.Repo.Name}}",
		"{{.Repo.Path}}",
		"{{.WorktreePath}}",
		"{{.ApproachName}}",
	}
	for _, v := range expected {
		if !strings.Contains(out, v) {
			t.Errorf("expected template variable %q in PrintTemplateHelp output, not found", v)
		}
	}
}

func TestFinalize_CallsBothOutputs(t *testing.T) {
	dir := t.TempDir()
	input := testWizardInput()
	r := strings.NewReader("y\n")
	var w bytes.Buffer

	if err := Finalize(context.Background(), &w, r, input, dir); err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	out := w.String()

	// Dock snippet banner
	if !strings.Contains(out, "Add this to ~/.config/cmux/dock.json") {
		t.Errorf("expected dock snippet banner in finalize output, got: %q", out)
	}

	// Template help banner
	if !strings.Contains(out, "Starter prompt template variables") {
		t.Errorf("expected template help banner in finalize output, got: %q", out)
	}
}

func TestPrintDockSnippet_BinaryPathFromConfigDir(t *testing.T) {
	// Verify that the binary path in the snippet is derived from configDir.
	configDir := "/custom/install/path"
	var buf bytes.Buffer
	PrintDockSnippet(&buf, configDir)
	out := buf.String()

	// The binary path is filepath.Join(configDir, "..", "cmux-board")
	expectedPath := filepath.Join(configDir, "..", "cmux-board")
	if !strings.Contains(out, expectedPath) {
		t.Errorf("expected binary path %q in dock snippet, got: %q", expectedPath, out)
	}
}
