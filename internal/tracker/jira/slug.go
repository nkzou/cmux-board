package jira

import "strings"

// Slug converts a string into a URL/identifier-safe slug:
//  1. lowercase the input
//  2. replace any rune not in [a-z0-9-] with '-'
//  3. collapse consecutive '-' runs into a single '-'
//  4. trim leading and trailing '-'
//
// Slug("") returns "" without panicking.
// Slug is exported so M-007 can call jira.Slug(name) for repo_id default derivation.
func Slug(s string) string {
	s = strings.ToLower(s)

	// Replace any rune not in [a-z0-9-] with '-'.
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, s)

	// Collapse consecutive '-' runs.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Trim leading and trailing '-'.
	s = strings.Trim(s, "-")

	return s
}
