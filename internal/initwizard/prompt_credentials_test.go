//go:build testing

package initwizard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nkzou/cmux-board/internal/secretsink"
)

// resetSecretsForTest clears the secretsink for test isolation.
// We access the unexported reset via the sink_test.go helper pattern by
// calling Register("") which is a no-op, but we can't call resetSecrets() from here.
// Instead, we rely on test isolation via the testing tag and per-test tracking.

func TestPromptJiraCredentials_Interactive(t *testing.T) {
	// Inject: site on first line, token on second line (test seam — no tty masking)
	input := "myorg.atlassian.net\n" + strings.Repeat("a", 25) + "\n"
	r := strings.NewReader(input)
	var w bytes.Buffer
	creds, err := PromptJiraCredentials(r, &w, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Site != "myorg.atlassian.net" {
		t.Errorf("site: expected %q, got %q", "myorg.atlassian.net", creds.Site)
	}
	if len(creds.APIToken) < minTokenLength {
		t.Errorf("token too short: %q", creds.APIToken)
	}
}

func TestPromptJiraCredentials_SkipSiteHint(t *testing.T) {
	// When siteHint is provided, skip the site prompt
	token := strings.Repeat("b", 25)
	r := strings.NewReader(token + "\n")
	var w bytes.Buffer
	creds, err := PromptJiraCredentials(r, &w, "hint.atlassian.net", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Site != "hint.atlassian.net" {
		t.Errorf("expected site from hint %q, got %q", "hint.atlassian.net", creds.Site)
	}
}

func TestPromptJiraCredentials_TokenFromStdin(t *testing.T) {
	token := strings.Repeat("c", 25)
	r := strings.NewReader(token + "\n")
	var w bytes.Buffer
	creds, err := PromptJiraCredentials(r, &w, "org.atlassian.net", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.APIToken != token {
		t.Errorf("expected token %q, got %q", token, creds.APIToken)
	}
	if !strings.Contains(w.String(), "Reading API token from stdin") {
		t.Errorf("expected stdin indicator in output, got: %q", w.String())
	}
}

func TestPromptJiraCredentials_EmptyTokenStdinError(t *testing.T) {
	r := strings.NewReader("\n") // empty line
	var w bytes.Buffer
	_, err := PromptJiraCredentials(r, &w, "org.atlassian.net", true)
	if err == nil {
		t.Fatal("expected error for empty token in stdin mode")
	}
}

func TestPromptJiraCredentials_StripHTTPS(t *testing.T) {
	token := strings.Repeat("d", 25)
	r := strings.NewReader("https://myorg.atlassian.net/\n" + token + "\n")
	var w bytes.Buffer
	creds, err := PromptJiraCredentials(r, &w, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Site != "myorg.atlassian.net" {
		t.Errorf("expected stripped site, got %q", creds.Site)
	}
}

func TestPromptJiraCredentials_RejectPlaceholderToken(t *testing.T) {
	placeholder := atlassianPlaceholderToken + strings.Repeat("x", 30)
	r := strings.NewReader(placeholder + "\n")
	var w bytes.Buffer
	_, err := PromptJiraCredentials(r, &w, "org.atlassian.net", true)
	if err == nil {
		t.Fatal("expected error for placeholder token")
	}
	if !strings.Contains(err.Error(), "documentation example") {
		t.Errorf("expected helpful error message, got: %v", err)
	}
}

func TestPromptJiraCredentials_RegistersSecret(t *testing.T) {
	// Count before
	before := secretsink.RegisteredCount()

	token := strings.Repeat("e", 25)
	r := strings.NewReader(token + "\n")
	var w bytes.Buffer
	_, err := PromptJiraCredentials(r, &w, "org.atlassian.net", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := secretsink.RegisteredCount()
	if after <= before {
		t.Errorf("expected secretsink.Register to be called; before=%d, after=%d", before, after)
	}
}
