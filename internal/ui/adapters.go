package ui

import (
	"context"

	"github.com/nkzou/cmux-board/internal/claudecli"
	"github.com/nkzou/cmux-board/internal/cmuxcli"
)

// realCmuxCli is the production adapter for the cmuxcli interfaces (focusCmuxCli,
// respawnCmuxCli) that delegates to the package-level cmuxcli functions.
type realCmuxCli struct{}

func (r *realCmuxCli) ListWorkspaces(ctx context.Context) ([]cmuxcli.Workspace, error) {
	return cmuxcli.ListWorkspaces(ctx)
}

func (r *realCmuxCli) ListPanes(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error) {
	return cmuxcli.ListPanes(ctx, wsRef)
}

func (r *realCmuxCli) NewWorkspaceWithLayout(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error) {
	return cmuxcli.NewWorkspaceWithLayout(ctx, args)
}

func (r *realCmuxCli) FocusPane(ctx context.Context, wsRef, paneRef string) error {
	return cmuxcli.FocusPane(ctx, wsRef, paneRef)
}

// realClaudeCli is the production adapter for the claudecli interfaces
// (focusClaudeCli, respawnClaudeCli).
type realClaudeCli struct{}

func (r *realClaudeCli) IsOrphan(ctx context.Context, worktree, shortID string) (bool, error) {
	return claudecli.IsOrphan(ctx, worktree, shortID)
}

func (r *realClaudeCli) Respawn(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error) {
	return claudecli.Respawn(ctx, args)
}
