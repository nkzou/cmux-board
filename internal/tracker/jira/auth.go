package jira

import (
	"encoding/base64"
)

// Credentials holds the authentication credentials for a Jira Cloud instance.
type Credentials struct {
	Site     string
	Email    string
	APIToken string
}

// buildBasicAuth computes the base64-encoded Basic auth credential string
// from email and API token. The result is used in the Authorization header.
// This function MUST NOT be logged; it produces a derived credential value.
func buildBasicAuth(email, token string) string {
	return base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
}
