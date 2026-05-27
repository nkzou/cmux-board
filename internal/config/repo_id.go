package config

import "github.com/kevin-zou/cmux-board/internal/tracker"

// DeriveRepoID computes the default repo_id from a display name.
// Algorithm: lowercase → replace runs of non-[a-z0-9-] with '-' → trim '-' from ends.
// Examples:
//
//	"MyService"    → "myservice"
//	"cmux-board"   → "cmux-board"
//	"My Repo 2!"   → "my-repo-2"
//	"  leading  "  → "leading"
//	"---"          → "" (returns empty; caller should reject empty slug)
func DeriveRepoID(displayName string) string {
	return tracker.Slug(displayName)
}
