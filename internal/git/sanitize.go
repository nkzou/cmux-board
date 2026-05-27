package git

// SanitizeBranchSegment lowercases s and replaces any character not in
// [a-z0-9._-] with a dash, then collapses consecutive dashes and trims
// leading/trailing dashes.  It is the exported form of sanitizeBranchName.
// Used by the activation orchestrator (T-062) to derive stable directory and
// branch-name segments from user-supplied approach names.
func SanitizeBranchSegment(s string) string {
	return sanitizeBranchName(s)
}
