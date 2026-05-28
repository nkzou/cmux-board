// Package verify provides a Go test wrapper for the seven-arm negative-grep
// enforcement script (scripts/check_additive_only.sh). Running `go test .`
// at the repo root executes the same checks as `make verify-additive`, so
// regressions are caught by both make targets and CI test suites.
package verify_test

import (
	"os/exec"
	"testing"
)

// TestAdditiveOnly runs scripts/check_additive_only.sh as a subprocess.
// The test fails if the script exits non-zero, which means at least one of
// the seven grep arms found a forbidden pattern in the production source tree.
//
// This test is the canonical enforcement of the CONVENTIONS.md immutable
// constraints listed in the script. It deliberately does NOT inline the grep
// logic — the script is the single source of truth.
func TestAdditiveOnly(t *testing.T) {
	cmd := exec.Command("bash", "scripts/check_additive_only.sh", "--all")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("check_additive_only.sh output:\n%s", out)
		t.Fatalf("verify-additive FAIL: %v", err)
	}
	t.Logf("check_additive_only.sh output:\n%s", out)
}
