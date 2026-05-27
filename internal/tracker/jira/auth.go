package jira

import (
	"encoding/base64"
)

// Credentials holds the authentication credentials for a Jira Cloud instance.
type Credentials struct {
	Site     string // e.g. "datadog.atlassian.net" (no scheme, no trailing slash)
	Email    string
	APIToken string
}

// buildBasicAuth computes the base64-encoded Basic auth credential string.
// Used only in tests (F20.e meta-fixture) — the adapter delegates auth to go-atlassian.
// This function MUST NOT be logged; it produces a derived credential value.
func buildBasicAuth(email, token string) string {
	return base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
}
