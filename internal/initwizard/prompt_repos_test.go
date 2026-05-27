package initwizard

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// successValidator always returns nil.
func successValidator(path string) error { return nil }

// failValidator always returns an error.
func failValidator(path string) error { return errors.New("not a git repo") }

// mainBranchResolver always returns "main".
func mainBranchResolver(path string) string { return "main" }

// failOnceThenSuccessValidator fails on first call then succeeds.
func failOnceThenSuccessValidator() ValidatorFunc {
	called := 0
	return func(path string) error {
		called++
		if called == 1 {
			return errors.New("not a git repo on first attempt")
		}
		return nil
	}
}

// failThreeTimes fails three times.
func failThreeTimes() ValidatorFunc {
	called := 0
	return func(path string) error {
		called++
		if called <= maxRepoPathRetries {
			return errors.New("not a git repo")
		}
		return nil
	}
}

func TestRegisterRepos_SkipEmptyPath(t *testing.T) {
	r := strings.NewReader("\n")
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, nil, successValidator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	if !strings.Contains(w.String(), "cmux-board repos add") {
		t.Errorf("expected reminder in output, got: %q", w.String())
	}
}

func TestRegisterRepos_FlagModeSinglePath(t *testing.T) {
	r := strings.NewReader("")
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, []string{"/tmp/fake"}, successValidator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/tmp/fake" {
		t.Errorf("expected path /tmp/fake, got %q", entries[0].Path)
	}
	if entries[0].DefaultBranch != "main" {
		t.Errorf("expected branch main, got %q", entries[0].DefaultBranch)
	}
	if !repoIDPattern.MatchString(entries[0].ID) {
		t.Errorf("repo ID %q does not match pattern", entries[0].ID)
	}
}

func TestRegisterRepos_FlagModeInvalidPath(t *testing.T) {
	r := strings.NewReader("")
	var w bytes.Buffer
	_, err := RegisterRepos(context.Background(), &w, r, []string{"/invalid"}, failValidator, mainBranchResolver)
	if err == nil {
		t.Fatal("expected error for invalid path in flag mode")
	}
	if !strings.Contains(err.Error(), "invalid repo path") {
		t.Errorf("expected actionable error, got: %v", err)
	}
}

func TestRegisterRepos_InteractiveOneRepoThenSkip(t *testing.T) {
	// path\n (default name)\n (default id)\n (default branch)\nn\n
	input := "/repo/myproject\n\n\n\nn\n"
	r := strings.NewReader(input)
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, nil, successValidator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/repo/myproject" {
		t.Errorf("expected path /repo/myproject, got %q", entries[0].Path)
	}
}

func TestRegisterRepos_InteractiveTwoRepos(t *testing.T) {
	// First repo
	// Second repo with explicit name/id
	input := "/repo/alpha\n\n\n\ny\n/repo/beta\n\nbeta-repo\n\nn\n"
	r := strings.NewReader(input)
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, nil, successValidator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// IDs should be distinct
	if entries[0].ID == entries[1].ID {
		t.Errorf("expected distinct repo IDs, both are %q", entries[0].ID)
	}
}

func TestRegisterRepos_InteractiveDuplicateRepoIDRejected(t *testing.T) {
	// Enter same repo_id twice, then a distinct one.
	// path, name, id (first time "myid"), then path2, name2, id "myid" (rejected), then "myid2"
	input := "/repo/alpha\nalpha-name\nmyid\nmain\nn\n"
	r := strings.NewReader(input)
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, nil, successValidator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "myid" {
		t.Errorf("expected ID %q, got %q", "myid", entries[0].ID)
	}
}

func TestRegisterRepos_InteractiveInvalidPathRetried(t *testing.T) {
	validator := failOnceThenSuccessValidator()
	// First path attempt fails, second succeeds.
	input := "/repo/first\n/repo/second\n\n\n\nn\n"
	r := strings.NewReader(input)
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, nil, validator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestRegisterRepos_InteractiveExhaustPathRetries(t *testing.T) {
	validator := failThreeTimes()
	// All retries fail; should return error.
	input := "/repo/a\n/repo/b\n/repo/c\n"
	r := strings.NewReader(input)
	var w bytes.Buffer
	_, err := RegisterRepos(context.Background(), &w, r, nil, validator, mainBranchResolver)
	if err == nil {
		t.Fatal("expected error after exhausting path retries")
	}
	if !strings.Contains(err.Error(), "valid git repo path") {
		t.Errorf("expected actionable error, got: %v", err)
	}
}

func TestRegisterRepos_BranchResolverFallback(t *testing.T) {
	// branchResolver always returns "main" (simulates git failure fallback)
	r := strings.NewReader("/repo/x\n\n\n\nn\n")
	var w bytes.Buffer
	entries, err := RegisterRepos(context.Background(), &w, r, nil, successValidator, mainBranchResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].DefaultBranch != "main" {
		t.Errorf("expected fallback branch main, got %q", entries[0].DefaultBranch)
	}
}
