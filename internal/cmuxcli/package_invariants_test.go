package cmuxcli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenTokens are cmux command literals that must never appear in non-comment source code
// within internal/cmuxcli/. Their presence would indicate an additive-only violation (F-NEW).
//
// The list mirrors the // Never call: comment in package.go (and at the head of runner.go).
// If a new destructive command is added to the documentation comment, its token MUST also
// be added here — and vice versa.
var forbiddenTokens = []string{
	"close-others",
	"close-above",
	"close-below",
	"close-window",
	"workspace-action",
	// "cmux kill" specifically — matches the exec.Command argv form to avoid
	// false positives on the bare word "kill" (e.g. signal.Kill, kernel sources).
	"\"cmux kill\"",
}

// TestPackageInvariants_NoDestructiveCmuxCommands walks every .go source file under
// internal/cmuxcli/ and fails if any forbidden token appears on a non-comment line.
//
// The ONE allowed exception: a line whose trimmed content begins with "//" (Go comment).
// The canonical allowed occurrences are:
//   - the "// Never call: ..." block in package.go
//   - the "// Never call: ..." block at the head of runner.go
//
// This is a pre-staging guard for F-NEW. The full-tree check is in M-011 T-084.
func TestPackageInvariants_NoDestructiveCmuxCommands(t *testing.T) {
	srcDir := "."

	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Test files are excluded — they use forbidden tokens in negative assertions
		// (asserting the tokens do NOT appear in production argv). The grep guard targets
		// PRODUCTION code only; the M-011 full-tree check (T-084) covers cross-file flow.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		checkFileForForbiddenTokens(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir error: %v", err)
	}
}

func checkFileForForbiddenTokens(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("cannot open %s: %v", path, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Allow the "// Never call:" comment lines that document the forbidden set.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		for _, token := range forbiddenTokens {
			if strings.Contains(line, token) {
				t.Errorf("%s:%d: forbidden destructive cmux token %q found in non-comment line "+
					"(additive-only violation, F-NEW / T-039)\n  line: %s",
					path, lineNum, token, line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Errorf("scanner error for %s: %v", path, err)
	}
}

// TestPackageInvariants_NoCurrentDirectoryAdoptionInSourceFiles asserts that
// current_directory is not used as an adoption-match key in any non-comment,
// non-struct-tag source context within internal/cmuxcli/ (E8 / Codex Finding 1).
//
// Specifically: the string "current_directory" may appear in JSON struct tags
// (parsing) and in comment lines, but must NOT appear in string comparisons,
// map lookups, equality expressions, or similar adoption logic.
//
// Heuristic: flag any line containing "current_directory" that is not a comment,
// not a struct tag line (contains `json:"current_directory"`), and not a test file.
func TestPackageInvariants_NoCurrentDirectoryAdoptionInSourceFiles(t *testing.T) {
	srcDir := "."

	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Test files are allowed to mention current_directory in their assertions/fixtures.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		checkNoCurrentDirectoryAdoption(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir error: %v", err)
	}
}

func checkNoCurrentDirectoryAdoption(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("cannot open %s: %v", path, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !strings.Contains(line, "current_directory") {
			continue
		}
		// Allow: comments, struct field JSON tags (parsing only), slog key strings.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, `json:"current_directory"`) {
			continue // struct tag — parsing only
		}
		if strings.Contains(line, `"current_directory"`) && strings.Contains(line, "slog") {
			continue // diagnostic log key
		}
		t.Errorf("%s:%d: current_directory appears in non-tag, non-comment context — "+
			"this may indicate foreign-workspace adoption by current_directory heuristic, "+
			"which is forbidden (E8 / Codex Finding 1 / CONVENTIONS.md)\n  line: %s",
			path, lineNum, line)
	}
}
