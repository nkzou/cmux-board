package tracker

import "strings"

// Slug converts a display name to a URL-safe kebab-case identifier.
// Algorithm: lowercase, then replace any run of non-[a-z0-9-] characters with a single '-',
// then trim leading/trailing '-'.
//
// Examples:
//
//	Slug("OpenKanban")  → "openkanban"
//	Slug("cmux-board")  → "cmux-board"
//	Slug("My Repo 2!")  → "my-repo-2"
//	Slug("  leading  ") → "leading"
//	Slug("---")         → "" (empty; caller should reject an empty slug)
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
