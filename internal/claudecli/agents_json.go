package claudecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// AgentsArgs controls the `claude agents --json` invocation.
type AgentsArgs struct {
	// CWD filters results to sessions whose cwd matches this path.
	// If empty, all sessions are returned (no --cwd flag).
	// claude normalizes /tmp ↔ /private/tmp on macOS; pass the worktree path as-is.
	CWD string
}

// AgentEntry is one entry from `claude agents --json` output.
// All fields are parsed defensively; unknown values use zero values or "unknown".
//
// JSON schema (verified live, RESEARCH §5):
//
//	{ "pid": N, "cwd": "...", "kind": "background"|"interactive", "startedAt": N,
//	  "sessionId": "uuid", "name": "...", "status": "idle"|"busy"|"waiting"|... }
type AgentEntry struct {
	PID       int    `json:"pid"`
	CWD       string `json:"cwd"`
	Kind      string `json:"kind"`
	StartedAt int64  `json:"startedAt"`
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

// knownStatuses is the set of documented status values.
// Entries with other values are normalized to "unknown" rather than causing errors.
var knownStatuses = map[string]bool{
	"idle": true, "busy": true, "waiting": true,
	"working": true, "failed": true, "stopped": true, "completed": true,
}

// Agents runs `claude agents --json [--cwd <cwd>]` and returns the parsed entries.
// Unknown status strings are normalized to "unknown". Malformed individual entries
// are skipped with a slog warning. An empty array response is not an error.
func Agents(ctx context.Context, args AgentsArgs) ([]AgentEntry, error) {
	argv := []string{"agents", "--json"}
	if args.CWD != "" {
		argv = append(argv, "--cwd", args.CWD)
	}

	cmd := exec.CommandContext(ctx, "claude", argv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude agents --json failed (stderr: %s): %w",
			stderr.String(), err)
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("claude agents --json: failed to parse JSON array: %w", err)
	}

	entries := make([]AgentEntry, 0, len(raw))
	for _, r := range raw {
		var e AgentEntry
		if err := json.Unmarshal(r, &e); err != nil {
			slog.Warn("claude agents: skipping malformed entry", "raw", string(r), "err", err)
			continue
		}
		if !knownStatuses[e.Status] && e.Status != "" {
			slog.Debug("claude agents: normalizing unknown status",
				"original", e.Status, "sessionId", e.SessionID)
			e.Status = "unknown"
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// FindByShortID returns the first AgentEntry whose SessionID has shortID as a prefix.
// In case of multiple matches (theoretically impossible at 8 hex chars), the newest
// startedAt wins. Returns nil if not found.
func FindByShortID(entries []AgentEntry, shortID string) *AgentEntry {
	var best *AgentEntry
	for i := range entries {
		e := &entries[i]
		if strings.HasPrefix(e.SessionID, shortID) {
			if best == nil || e.StartedAt > best.StartedAt {
				best = e
			}
		}
	}
	return best
}

// FindByActIDShort returns all AgentEntry values whose Name contains actIDShort as a
// substring. Used by F-NEW4 startup reconciliation to discover resources created in
// the after-effect-before-journal kill window.
func FindByActIDShort(entries []AgentEntry, actIDShort string) []AgentEntry {
	var out []AgentEntry
	for _, e := range entries {
		if strings.Contains(e.Name, actIDShort) {
			out = append(out, e)
		}
	}
	return out
}
