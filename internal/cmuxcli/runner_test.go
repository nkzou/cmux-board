package cmuxcli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeFakeCmux writes an executable shell script named "cmux" to dir.
// Tests prepend dir to PATH so the runner invokes this script instead of any real cmux.
func writeFakeCmux(t *testing.T, dir, script string) {
	t.Helper()
	p := filepath.Join(dir, "cmux")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeCmux: %v", err)
	}
}

// prependPath sets PATH=dir:<orig> for the duration of the test.
func prependPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TC-1: success on first attempt.
func TestRunCmux_SuccessFirstAttempt(t *testing.T) {
	dir := t.TempDir()
	writeFakeCmux(t, dir, "#!/bin/sh\necho '{\"ok\":true}'\nexit 0\n")
	prependPath(t, dir)

	stdout, _, err := runCmux(context.Background(), "identify")
	if err != nil {
		t.Fatalf("TC-1: unexpected error: %v", err)
	}
	if !bytes.Contains(stdout, []byte(`"ok":true`)) {
		t.Errorf("TC-1: stdout missing expected JSON: %q", stdout)
	}
}

// TC-2: retry on exit error, succeed on third attempt; assert exactly 3 invocations
// at ≥180ms spacing.
func TestRunCmux_RetrySucceedThirdAttempt(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	timesPath := filepath.Join(dir, "times")
	// Script: increments counter; exits 1 on attempts 1 and 2; exits 0 on attempt 3+.
	// Records invocation timestamp (nanoseconds) on each call.
	script := `#!/bin/sh
COUNTER_FILE="` + counterPath + `"
TIMES_FILE="` + timesPath + `"
n=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" > "$COUNTER_FILE"
# Record monotonic-ish time in nanoseconds (using date +%s%N when available, fallback %s).
ns=$(date +%s%N 2>/dev/null)
if [ "${ns#*N}" = "$ns" ] && [ -n "$ns" ]; then
  echo "$ns" >> "$TIMES_FILE"
else
  echo "${ns%N}000000000" >> "$TIMES_FILE"
fi
if [ "$n" -lt 3 ]; then exit 1; fi
echo "ok"
exit 0
`
	writeFakeCmux(t, dir, script)
	prependPath(t, dir)

	stdout, _, err := runCmux(context.Background(), "identify")
	if err != nil {
		t.Fatalf("TC-2: unexpected error: %v", err)
	}
	if !bytes.Contains(stdout, []byte("ok")) {
		t.Errorf("TC-2: stdout missing 'ok': %q", stdout)
	}

	got, _ := os.ReadFile(counterPath)
	if strings.TrimSpace(string(got)) != "3" {
		t.Errorf("TC-2: want exactly 3 invocations, got %q", got)
	}

	// Verify spacing >= 180ms between consecutive invocations.
	timesRaw, _ := os.ReadFile(timesPath)
	lines := strings.Split(strings.TrimSpace(string(timesRaw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("TC-2: want 3 timestamps, got %d: %v", len(lines), lines)
	}
	parseNs := func(s string) int64 {
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return 0
		}
		return v
	}
	t1, t2, t3 := parseNs(lines[0]), parseNs(lines[1]), parseNs(lines[2])
	const minSpacing = int64(180 * time.Millisecond) // 20ms tolerance below 200ms target
	if t2-t1 < minSpacing {
		t.Errorf("TC-2: attempt 2 spacing %dns < %dns (180ms)", t2-t1, minSpacing)
	}
	if t3-t2 < minSpacing {
		t.Errorf("TC-2: attempt 3 spacing %dns < %dns (180ms)", t3-t2, minSpacing)
	}
}

// TC-3: exhaustion returns ErrCmuxUnreachable.
func TestRunCmux_ExhaustionReturnsErrCmuxUnreachable(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	script := `#!/bin/sh
COUNTER_FILE="` + counterPath + `"
n=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" > "$COUNTER_FILE"
echo "boom" 1>&2
exit 1
`
	writeFakeCmux(t, dir, script)
	prependPath(t, dir)

	_, _, err := runCmux(context.Background(), "identify")
	if err == nil {
		t.Fatal("TC-3: want non-nil error after exhaustion")
	}
	if !errors.Is(err, ErrCmuxUnreachable) {
		t.Errorf("TC-3: errors.Is(err, ErrCmuxUnreachable) must be true, got %v", err)
	}

	got, _ := os.ReadFile(counterPath)
	if strings.TrimSpace(string(got)) != "3" {
		t.Errorf("TC-3: want exactly 3 invocations, got %q", got)
	}
}

// TC-4: socket-broken pattern in stderr triggers a retry even on exit 0.
func TestRunCmux_SocketBrokenPatternTriggersRetry(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	// First attempt: exit 0 but stderr has "connection refused" → must retry.
	// Second attempt: clean exit 0.
	script := `#!/bin/sh
COUNTER_FILE="` + counterPath + `"
n=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" > "$COUNTER_FILE"
if [ "$n" -eq 1 ]; then
  echo "connection refused" 1>&2
  exit 0
fi
echo "clean"
exit 0
`
	writeFakeCmux(t, dir, script)
	prependPath(t, dir)

	stdout, _, err := runCmux(context.Background(), "identify")
	if err != nil {
		t.Fatalf("TC-4: unexpected error: %v", err)
	}
	if !bytes.Contains(stdout, []byte("clean")) {
		t.Errorf("TC-4: stdout missing 'clean': %q", stdout)
	}
	got, _ := os.ReadFile(counterPath)
	if strings.TrimSpace(string(got)) != "2" {
		t.Errorf("TC-4: want 2 invocations, got %q", got)
	}
}

// TC-5: context cancellation stops retries.
// The test cancels the context while the runner is sleeping between retries.
// We assert (a) ctx.Err is propagated, (b) fewer than cmuxRetryCount attempts complete.
func TestRunCmux_ContextCancellationStopsRetries(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	// Always exits 1 so the runner would otherwise retry up to 3 times.
	script := `#!/bin/sh
COUNTER_FILE="` + counterPath + `"
n=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" > "$COUNTER_FILE"
exit 1
`
	writeFakeCmux(t, dir, script)
	prependPath(t, dir)

	// Use a manual cancel goroutine instead of WithTimeout to avoid races where the
	// deadline fires during cmd.Run() and kills the child before it can write the counter.
	// We cancel ~250ms in: attempt 1 completes (~few ms), 200ms sleep starts, then cancel
	// fires mid-sleep so the runner returns ctx.Err before attempt 2 launches.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(250 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	_, _, err := runCmux(ctx, "identify")
	if err == nil {
		t.Fatal("TC-5: want non-nil error after cancellation")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("TC-5: want context.Canceled or context.DeadlineExceeded, got %v", err)
	}
	got, _ := os.ReadFile(counterPath)
	n, _ := strconv.Atoi(strings.TrimSpace(string(got)))
	if n < 1 {
		t.Errorf("TC-5: want >= 1 invocation, got %d (counter=%q)", n, got)
	}
	if n >= cmuxRetryCount {
		t.Errorf("TC-5: want < %d invocations after cancellation, got %d", cmuxRetryCount, n)
	}
}

// TestRunner_NeverCallComment asserts runner.go contains the // Never call: comment
// at package head (the literal allowed match for F-NEW and T-039).
func TestRunner_NeverCallComment(t *testing.T) {
	src, err := os.ReadFile("runner.go")
	if err != nil {
		t.Fatalf("cannot read runner.go: %v", err)
	}
	if !bytes.Contains(src, []byte("// Never call:")) {
		t.Fatal("runner.go must contain '// Never call:' comment per F-NEW invariant")
	}
}
