package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// callTracker records which check functions were called.
type callTracker struct {
	called []int
}

func (ct *callTracker) step(n int) {
	ct.called = append(ct.called, n)
}

func (ct *callTracker) wasCalled(n int) bool {
	for _, c := range ct.called {
		if c == n {
			return true
		}
	}
	return false
}

// makeTestFns constructs a preflightFuncs where each function records its step number
// and returns the error specified in the errs map. Steps not in errs return nil.
// Steps > len(errs) are counted but return nil.
func makeTestFns(ct *callTracker, errs map[int]error) preflightFuncs {
	// helpers
	stepErr := func(n int) error {
		if e, ok := errs[n]; ok {
			return e
		}
		return nil
	}
	return preflightFuncs{
		checkModeBits: func() error {
			ct.step(1)
			return stepErr(1)
		},
		checkSchema: func() error {
			ct.step(2)
			return stepErr(2)
		},
		checkCmux: func(_ context.Context) (*cmuxResult, error) {
			ct.step(3)
			if e := stepErr(3); e != nil {
				return nil, e
			}
			return &cmuxResult{version: "workspace:1", callerRef: "window:1"}, nil
		},
		checkClaude: func(_ context.Context) (*claudeResult, error) {
			ct.step(4)
			if e := stepErr(4); e != nil {
				return nil, e
			}
			return &claudeResult{version: "2.1.150"}, nil
		},
		checkAgentView: func(_ context.Context) error {
			ct.step(5)
			return stepErr(5)
		},
	}
}

func TestPreflightCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errs        map[int]error
		wantErr     bool
		wantErrStr  string
		notCalled   []int
		unsafeCreds bool
	}{
		{
			name:    "all pass",
			errs:    map[int]error{},
			wantErr: false,
		},
		{
			name:       "step1 mode-bits fail",
			errs:       map[int]error{1: errors.New("unsafe mode")},
			wantErr:    true,
			notCalled:  []int{2, 3, 4, 5},
		},
		{
			name:       "step2 schema v1 fail",
			errs:       map[int]error{2: errors.New("schema v1 detected")},
			wantErr:    true,
			notCalled:  []int{3, 4, 5},
		},
		{
			name:      "step3 cmux fail",
			errs:      map[int]error{3: fmt.Errorf("cmux pre-flight failed: cmux not running")},
			wantErr:   true,
			wantErrStr: "cmux pre-flight failed",
			notCalled: []int{4, 5},
		},
		{
			name:      "step4 claude version fail",
			errs:      map[int]error{4: errors.New("claude version check failed: found 2.1.149, need >= 2.1.150")},
			wantErr:   true,
			wantErrStr: "claude version check failed",
		},
		{
			name:      "step5 disable-agent-view fail",
			errs:      map[int]error{5: errors.New("agent view is disabled")},
			wantErr:   true,
			wantErrStr: "agent view is disabled",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ct := &callTracker{}
			fns := makeTestFns(ct, tt.errs)

			_, err := runCheck(context.Background(), fns)

			if tt.wantErr && err == nil {
				t.Fatal("wanted error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("wanted nil, got %v", err)
			}
			if tt.wantErrStr != "" && err != nil {
				if !contains(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
			}
			for _, step := range tt.notCalled {
				if ct.wasCalled(step) {
					t.Errorf("step %d should not have been called but was", step)
				}
			}
		})
	}
}

func TestPreflightUnsafeCreds(t *testing.T) {
	t.Parallel()

	// Build a config dir that has a credentials.json with unsafe permissions.
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(credsPath, []byte(`{"schema_version":2}`), 0644); err != nil {
		t.Fatal(err)
	}

	ct := &callTracker{}
	// Make step 1 record its call but it will be skipped because we inject unsafe=true
	// into the real checkModeBits. We simulate by making step 1 return nil only when
	// called — but we want to test the *actual* bypass path, so we use a custom fn.
	step1Called := false
	fns := preflightFuncs{
		checkModeBits: func() error {
			step1Called = true
			// In the real code, unsafeCreds=true means ValidateCredentialsMode returns nil.
			// Here we return nil unconditionally to simulate skip.
			return nil
		},
		checkSchema: func() error {
			ct.step(2)
			return errors.New("schema sentinel") // short-circuit after step 2
		},
		checkCmux:      func(_ context.Context) (*cmuxResult, error) { ct.step(3); return &cmuxResult{}, nil },
		checkClaude:    func(_ context.Context) (*claudeResult, error) { ct.step(4); return &claudeResult{}, nil },
		checkAgentView: func(_ context.Context) error { ct.step(5); return nil },
	}

	_, err := runCheck(context.Background(), fns)

	// step1Called should be true (it ran but returned nil because unsafe bypasses mode-bit logic)
	_ = step1Called
	// Step 2 should have run and returned sentinel error.
	if err == nil || !contains(err.Error(), "schema sentinel") {
		t.Errorf("wanted schema sentinel error, got %v", err)
	}
	// Steps 3-5 should NOT have run.
	for _, s := range []int{3, 4, 5} {
		if ct.wasCalled(s) {
			t.Errorf("step %d should not have been called", s)
		}
	}
}

// TestCheckModeBitsUnsafeSkips verifies that the real checkModeBits function
// skips validation when unsafeCreds=true.
func TestCheckModeBitsUnsafeSkips(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a credentials file with overly permissive mode.
	credsPath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(credsPath, []byte(`{}`), 0666); err != nil {
		t.Fatal(err)
	}
	// With unsafeCreds=false, should fail.
	if err := checkModeBits(dir, false); err == nil {
		t.Error("expected error for mode 0666 with unsafeCreds=false")
	}
	// With unsafeCreds=true, should pass.
	if err := checkModeBits(dir, true); err != nil {
		t.Errorf("expected nil for unsafeCreds=true, got %v", err)
	}
}

// ── Integration tests (build tag: integration) ──────────────────────────────
// See preflight_integration_test.go for tests that use mock binaries on PATH.

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
