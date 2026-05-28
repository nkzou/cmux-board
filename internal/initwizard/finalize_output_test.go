package initwizard

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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

func TestFinalize_CallsTemplateHelp(t *testing.T) {
	dir := t.TempDir()
	input := testWizardInput()
	r := strings.NewReader("y\n")
	var w bytes.Buffer

	if err := Finalize(context.Background(), &w, r, input, dir); err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	out := w.String()

	// Template help banner
	if !strings.Contains(out, "Starter prompt template variables") {
		t.Errorf("expected template help banner in finalize output, got: %q", out)
	}
}
