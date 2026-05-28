package claudecli

import (
	"context"
	"fmt"
)

// IsOrphan re-queries the live claude agent list for the given worktree and returns
// true if the cached shortID is no longer present. This is called lazily on activation
// focus (not on every tick) to avoid hammering the claude CLI.
//
// shortID is the 8-hex-char identifier stored in ActivationEntry.ClaudeShortID.
// worktree is the absolute path to the git linked worktree (passed as --cwd filter).
//
// A fresh `claude agents --json --cwd <worktree>` is issued on every call; there is no
// in-process caching here. The caller (picker focus handler in M-008) is responsible
// for caching the result in the BubbleTea model between keypresses.
func IsOrphan(ctx context.Context, worktree, shortID string) (bool, error) {
	if shortID == "" {
		// No short ID recorded → treat as orphan (activation was never fully journaled).
		return true, nil
	}
	entries, err := Agents(ctx, AgentsArgs{CWD: worktree})
	if err != nil {
		return false, fmt.Errorf("orphan check failed for worktree %s: %w", worktree, err)
	}
	match := FindByShortID(entries, shortID)
	return match == nil, nil
}

// Respawn launches a new Claude background session for an orphaned activation.
// It is a thin wrapper around LaunchBackground. The caller MUST journal the new
// BGResult.ShortID into the activation entry via store.Mutate after this returns.
//
// Respawn is invoked by the picker `r` key handler in the BubbleTea update loop (M-008).
// A new cmux workspace is also created for the respawned session; that is M-008's concern.
func Respawn(ctx context.Context, args BGArgs) (BGResult, error) {
	return LaunchBackground(ctx, args)
}
