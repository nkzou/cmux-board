package sync

import (
	"testing"
	"time"
)

// TestBackoffSequence verifies the three-step sequence 60s → 2m → 5m.
func TestBackoffSequence(t *testing.T) {
	// TC-1: first three calls return 60s / 2m / 5m.
	var b Backoff
	want := []time.Duration{60 * time.Second, 2 * time.Minute, 5 * time.Minute}
	for i, w := range want {
		got := b.Next()
		if got != w {
			t.Errorf("TC-1: call %d: want %v, got %v", i+1, w, got)
		}
	}
}

// TestBackoffCapMaintained verifies that calls beyond the third still return 5m.
func TestBackoffCapMaintained(t *testing.T) {
	// TC-2: six consecutive calls; last three must all equal 5m.
	var b Backoff
	var results [6]time.Duration
	for i := range results {
		results[i] = b.Next()
	}
	for i := 3; i < 6; i++ {
		if results[i] != backoffCap {
			t.Errorf("TC-2: call %d: want %v (cap), got %v", i+1, backoffCap, results[i])
		}
	}
}

// TestBackoffReset verifies that Reset() restarts the sequence from 60s.
func TestBackoffReset(t *testing.T) {
	// TC-3: advance to cap, reset, first call must return 60s.
	var b Backoff
	b.Next()
	b.Next()
	b.Next() // now at cap
	b.Reset()
	got := b.Next()
	if got != backoffBase {
		t.Errorf("TC-3: after Reset, want %v, got %v", backoffBase, got)
	}
}

// TestBackoffZeroValue verifies zero-value Backoff is safe to use.
func TestBackoffZeroValue(t *testing.T) {
	// TC-4: declare zero value, first call must return 60s.
	var b Backoff
	got := b.Next()
	if got != backoffBase {
		t.Errorf("TC-4: zero-value first call: want %v, got %v", backoffBase, got)
	}
}
