//go:build integration

package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"context"
)

// TestCheckCmuxMissingFromPATH verifies checkCmux returns "cmux pre-flight failed"
// when cmux binary is absent from PATH.
func TestCheckCmuxMissingFromPATH(t *testing.T) {
	t.Parallel()
	orig := os.Getenv("PATH")
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	defer t.Setenv("PATH", orig)

	_, err := checkCmux(context.Background())
	if err == nil {
		t.Fatal("expected error when cmux missing from PATH")
	}
	if !contains(err.Error(), "cmux pre-flight failed") {
		t.Errorf("error %q does not contain 'cmux pre-flight failed'", err.Error())
	}
}

// TestCheckClaudeVersionTooOld uses a mock claude binary returning "2.1.149".
func TestCheckClaudeVersionTooOld(t *testing.T) {
	t.Parallel()
	// Build mock binary that prints "2.1.149"
	dir := t.TempDir()
	src := filepath.Join(dir, "claude_main.go")
	if err := os.WriteFile(src, []byte(`package main
import "fmt"
func main() { fmt.Println("Claude Code 2.1.149") }
`), 0644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "claude")
	if out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput(); err != nil {
		t.Fatalf("build mock: %v\n%s", err, out)
	}

	orig := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+orig)
	defer t.Setenv("PATH", orig)

	_, err := checkClaudeVersion(context.Background())
	if err == nil {
		t.Fatal("expected error for version 2.1.149")
	}
	if !contains(err.Error(), "2.1.149") {
		t.Errorf("error %q does not contain '2.1.149'", err.Error())
	}
	if !contains(err.Error(), "2.1.150") {
		t.Errorf("error %q does not contain '2.1.150'", err.Error())
	}
}

// TestCheckAgentViewEnvVarSet verifies refusal when CLAUDE_CODE_DISABLE_AGENT_VIEW is set.
func TestCheckAgentViewEnvVarSet(t *testing.T) {
	t.Setenv("CLAUDE_CODE_DISABLE_AGENT_VIEW", "1")

	err := checkAgentView(context.Background())
	if err == nil {
		t.Fatal("expected error when CLAUDE_CODE_DISABLE_AGENT_VIEW=1")
	}
	if !contains(err.Error(), "agent view is disabled") {
		t.Errorf("error %q does not contain 'agent view is disabled'", err.Error())
	}
}
