#!/usr/bin/env bash
# check_additive_only.sh — seven-arm negative-grep enforcement.
#
# Exit code: 0 if all arms return zero matches, 1 on first violation.
# Pass --all to report every failing arm before exiting.
#
# Arms enforce the CONVENTIONS.md immutable constraints:
#   1. Additive-only cmux destructors (F-NEW)
#   2. claude --bg uses cmd.Dir, never --cwd (F13, Codex Finding 8)
#   3. cmux focus-pane, never bare cmux focus (F14, Codex Finding 8)
#   4. MergePulledTickets is the only Tickets-map write path (F-NEW5, Codex Finding 9)
#   5. No direct *State field mutation outside store files (F17, Codex Finding 4)
#   6. HTTP headers never logged (F20, Codex Finding 3)
#   7. No git-status dirty-worktree check in focus/activate paths (E9)

set -euo pipefail

ALL=0
FAILURES=0

for arg in "$@"; do
  [ "$arg" = "--all" ] && ALL=1
done

fail() {
  local arm="$1"
  echo "FAIL arm $arm — forbidden pattern found:"
  FAILURES=$((FAILURES + 1))
  if [ "$ALL" -eq 0 ]; then
    exit 1
  fi
}

# Helper: run git grep, exclude comment lines (lines starting with optional whitespace + //).
# Prints matching lines; returns 0 if matches found, 1 if none.
grep_no_comments() {
  git grep -nE "$@" 2>/dev/null | grep -v '^\S\+:[0-9]\+:\s*//' || true
}

# ── Arm 1: Additive-only cmux destructors ───────────────────────────────────
# No production code may call cmux close-workspace, cmux kill, or equivalents.
# The approved "// Never call:" documentation comments in internal/cmuxcli/ are
# excluded by the comment-line filter below.
echo "Arm 1: cmux destructors (close-workspace|kill) in production code..."
MATCHES=$(grep_no_comments 'cmux (close-workspace|kill)' -- internal/ ':!*_test.go')
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "1 (cmux destructors)"
else
  echo "  OK (0 matches)"
fi

# ── Arm 2: claude --bg must use cmd.Dir, never --cwd ────────────────────────
# Check only bg.go (the only file allowed to construct claude --bg argv).
echo "Arm 2: claude --cwd in internal/claudecli/bg.go..."
MATCHES=$(git grep -nE 'claude.*--cwd' -- internal/claudecli/bg.go ':!*_test.go' 2>/dev/null || true)
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "2 (claude --cwd)"
else
  echo "  OK (0 matches)"
fi

# ── Arm 3: cmux focus-pane, not bare cmux focus ──────────────────────────────
# Catches argv slices with 'cmux focus' not followed by '-' (i.e., not focus-pane/focus-window).
echo "Arm 3: bare 'cmux focus' (not cmux focus-pane) in production code..."
MATCHES=$(grep_no_comments 'cmux focus[^-]' -- internal/ ':!*_test.go')
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "3 (bare cmux focus)"
else
  echo "  OK (0 matches)"
fi

# ── Arm 4: MergePulledTickets is the only Tickets= write path ────────────────
echo "Arm 4: direct .Tickets= assignment outside store files..."
MATCHES=$(git grep -nE 'state\.State\)\.Tickets\s*=' -- internal/ ':!*_store*.go' ':!*_test.go' 2>/dev/null || true)
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "4 (direct Tickets=)"
else
  echo "  OK (0 matches)"
fi

# ── Arm 5: No direct *State mutation outside store ───────────────────────────
echo "Arm 5: direct *State field assignment outside store files..."
MATCHES=$(git grep -nE 'state\.State\)\.\w+\s*=' -- internal/ ':!*_store*.go' ':!*_test.go' 2>/dev/null || true)
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "5 (direct *State mutation)"
else
  echo "  OK (0 matches)"
fi

# ── Arm 6: HTTP headers never logged ────────────────────────────────────────
echo "Arm 6: HTTP header logging in production code..."
MATCHES=$(grep_no_comments 'log.*\.(Header|Request\.Header|Response\.Header)' -- internal/ ':!*_test.go')
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "6 (HTTP header logging)"
else
  echo "  OK (0 matches)"
fi

# ── Arm 7: No dirty-worktree check in focus/activate paths ──────────────────
echo "Arm 7: git status in focus.go / activate.go..."
MATCHES=$(git grep -nE '"git",\s*"status"' -- internal/ui/focus.go internal/ui/activate.go ':!*_test.go' 2>/dev/null || true)
if [ -n "$MATCHES" ]; then
  echo "$MATCHES"
  fail "7 (git status in focus/activate)"
else
  echo "  OK (0 matches)"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
if [ "$FAILURES" -gt 0 ]; then
  echo ""
  echo "FAIL: $FAILURES arm(s) found violations."
  exit 1
fi

echo ""
echo "PASS: all seven arms clean."
exit 0
