package initwizard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/kevin-zou/cmux-board/internal/secretsink"
)

// maxSitePromptRetries is the maximum number of invalid site input attempts.
const maxSitePromptRetries = 3

// atlassianPlaceholderToken is the example token from Atlassian's docs page — catch
// accidental copy-paste.
const atlassianPlaceholderToken = "ATATT3xFfGF0example"

// minTokenLength is the soft minimum length for an API token.
const minTokenLength = 20

// JiraCredentials holds the collected Jira auth material.
type JiraCredentials struct {
	Site     string // e.g. "datadog.atlassian.net"
	APIToken string // raw token, never logged
}

// There is intentionally no --api-token flag (F4: token must never appear in argv).
// The only token ingestion paths are interactive masked prompt or --api-token-stdin.

// PromptJiraCredentials prompts for site and api_token.
// If siteHint is non-empty (from --site flag), the site prompt is skipped.
// If tokenFromStdin is true, the token is read as a plain line from r (no masking needed
// because the caller has already redirected stdin from a pipe or 1Password CLI).
// Otherwise, masking is applied via golang.org/x/term (ReadPassword) when r is os.Stdin.
// Returns an error if r is closed before valid input is collected.
func PromptJiraCredentials(
	r io.Reader,
	w io.Writer,
	siteHint string,
	tokenFromStdin bool,
) (JiraCredentials, error) {
	var creds JiraCredentials

	// Wrap r in a single bufio.Reader so all reads share the same buffer.
	// This prevents data loss when promptSite and ReadToken both read from the same pipe.
	br := bufio.NewReader(r)

	// --- Site prompt ---
	if siteHint != "" {
		creds.Site = normalizeSite(siteHint)
	} else {
		site, err := promptSiteBR(br, w)
		if err != nil {
			return JiraCredentials{}, err
		}
		creds.Site = site
	}

	// --- Token prompt ---
	token, err := ReadTokenBR(w, br, r, tokenFromStdin)
	if err != nil {
		return JiraCredentials{}, err
	}
	if err := validateToken(token, tokenFromStdin); err != nil {
		return JiraCredentials{}, err
	}
	creds.APIToken = token

	// Register the raw API token with secretsink BEFORE any downstream use.
	// This is the authoritative registration point for the raw token
	// (per CONVENTIONS.md "Slog redaction at the Writer stage").
	secretsink.Register(creds.APIToken)

	return creds, nil
}

// ReadToken reads the API token from stdin if apiTokenStdin is true,
// otherwise prompts interactively with masked echo. Never accepts the token as a
// function argument to prevent accidental argv leakage.
// This is the public form that creates a fresh bufio.Reader; use ReadTokenBR when
// a shared bufio.Reader is already in use.
func ReadToken(w io.Writer, r io.Reader, apiTokenStdin bool) (string, error) {
	return ReadTokenBR(w, bufio.NewReader(r), r, apiTokenStdin)
}

// ReadTokenBR reads the API token using a pre-created bufio.Reader (shared with site prompt).
func ReadTokenBR(w io.Writer, br *bufio.Reader, r io.Reader, apiTokenStdin bool) (string, error) {
	if apiTokenStdin {
		fmt.Fprintln(w, "Reading API token from stdin...")
		line, err := br.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", fmt.Errorf("reading API token from stdin: %w", err)
		}
		return strings.TrimSpace(line), nil
	}

	// Detection logic: use real tty masking when r is os.Stdin, test seam otherwise.
	if r == os.Stdin {
		// Real tty: use term.ReadPassword
		fmt.Fprint(w, "Jira API token (input will be masked):\n> ")
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(w) // newline after masked input
		if err != nil {
			return "", fmt.Errorf("reading API token: %w", err)
		}
		return strings.TrimSpace(string(tokenBytes)), nil
	}

	// Test seam: plain read from br (no tty masking available in tests).
	fmt.Fprint(w, "Jira API token (input will be masked):\n> ")
	line, err := br.ReadString('\n')
	fmt.Fprintln(w) // simulate newline after masked input
	if err != nil && len(line) == 0 {
		return "", fmt.Errorf("reading API token: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// promptSite prompts the user for the Jira site URL with up to maxSitePromptRetries
// attempts. Strips https:// prefix and trailing slash. Validates the format.
// Deprecated: use promptSiteBR to share a bufio.Reader with subsequent prompts.
func promptSite(r io.Reader, w io.Writer) (string, error) {
	return promptSiteBR(bufio.NewReader(r), w)
}

// promptSiteBR prompts for the Jira site using a pre-created bufio.Reader.
func promptSiteBR(reader *bufio.Reader, w io.Writer) (string, error) {
	for attempt := 0; attempt < maxSitePromptRetries; attempt++ {
		fmt.Fprintln(w, "Jira site URL (e.g. yourorg.atlassian.net):")
		fmt.Fprint(w, "> ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", fmt.Errorf("reading site: %w", err)
		}
		site := normalizeSite(strings.TrimSpace(line))
		if validateSite(site) {
			return site, nil
		}
		fmt.Fprintln(w, "Expected format: <yourorg>.atlassian.net")
	}
	return "", fmt.Errorf("invalid site URL after %d attempts", maxSitePromptRetries)
}

// normalizeSite strips https:// prefix and trailing slash.
func normalizeSite(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimRight(s, "/")
	return s
}

// validateSite checks that the site is a valid Atlassian host.
func validateSite(site string) bool {
	if site == "" || strings.ContainsAny(site, " \t") {
		return false
	}
	return strings.HasSuffix(site, ".atlassian.net") || strings.HasSuffix(site, ".jira.com")
}

// validateToken checks the token for common issues.
// In piped mode (fromStdin true), returns error on invalid; in interactive mode
// the caller should re-prompt (but we return the error here for callers to handle).
func validateToken(token string, fromStdin bool) error {
	if token == "" {
		return fmt.Errorf("API token must not be empty")
	}
	if strings.HasPrefix(token, atlassianPlaceholderToken) {
		return fmt.Errorf("API token looks like the Atlassian documentation example — use your real token")
	}
	if len(token) < minTokenLength {
		return fmt.Errorf("API token is too short (%d chars); expected at least %d", len(token), minTokenLength)
	}
	return nil
}
