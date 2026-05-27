package ui

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevin-zou/cmux-board/internal/claudecli"
	"github.com/kevin-zou/cmux-board/internal/cmuxcli"
	"github.com/kevin-zou/cmux-board/internal/state"
)

// FocusResult is the outcome of a Focus call.
type FocusResult struct {
	// NeedsRespawn is true when claude_orphan is detected. The picker should
	// show the [orphan] glyph and wait for the user to press r.
	NeedsRespawn bool

	// NeedsNewTab is true when cmux_orphan was detected and a new workspace
	// was created. The UI should surface a toast informing the user.
	NeedsNewTab bool

	Err error
}

// Focus executes the focus flow for a single activation.
//
// Sequence:
//  1. Claude orphan check (lazy): if ClaudeShortID is no longer present in
//     claude agents --json, mark claude_orphan=true and return NeedsRespawn.
//  2. cmux orphan check: if CmuxWorkspaceID is absent from cmux list-workspaces,
//     mark cmux_orphan=true, create a NEW workspace (additive-only), then update state.
//     NEVER adopt a foreign workspace by current_directory — forbidden (Codex Finding 1).
//  3. FocusPane: call cmuxcli.FocusPane with the cached agent_pane_ref.
//
// All state mutations go through store.Mutate (Codex Finding 4).
func Focus(
	ctx context.Context,
	store *state.Store,
	cmuxClient focusCmuxCli,
	claudeClient focusClaudeCli,
	ticketID string,
	activation *state.ActivationEntry,
) FocusResult {
	// 1. Claude orphan check (lazy).
	orphan, err := claudeClient.IsOrphan(ctx, activation.WorktreePath, activation.ClaudeShortID)
	if err != nil {
		return FocusResult{Err: fmt.Errorf("orphan check failed: %w", err)}
	}
	if orphan {
		if mutErr := store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, activation.ActivationID)
			if act == nil {
				return nil
			}
			act.ClaudeOrphan = true
			return nil
		}); mutErr != nil {
			return FocusResult{Err: fmt.Errorf("failed to mark claude_orphan: %w", mutErr)}
		}
		return FocusResult{NeedsRespawn: true}
	}

	// 2. cmux orphan check.
	workspaces, err := cmuxClient.ListWorkspaces(ctx)
	if err != nil {
		return FocusResult{Err: fmt.Errorf("failed to list cmux workspaces: %w", err)}
	}

	result := FocusResult{}
	currentWsRef := activation.CmuxWorkspaceID
	currentPaneRef := activation.AgentPaneRef

	if cmuxcli.IsOrphan(activation.CmuxWorkspaceID, workspaces) {
		// Log any foreign workspace whose current_directory matches — diagnostic only.
		for _, ws := range workspaces {
			if ws.CurrentDirectory == activation.WorktreePath {
				slog.Debug("foreign workspace at worktree path ignored (additive-only invariant)",
					"foreign_ref", ws.Ref,
					"worktree_path", activation.WorktreePath)
				// Do NOT store this ref — adoption by heuristic is forbidden.
				break
			}
		}

		// Mark orphan.
		if mutErr := store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, activation.ActivationID)
			if act == nil {
				return nil
			}
			act.CmuxOrphan = true
			return nil
		}); mutErr != nil {
			return FocusResult{Err: fmt.Errorf("failed to mark cmux_orphan: %w", mutErr)}
		}

		// Create NEW workspace (additive-only).
		agentCmd := "claude attach " + activation.ClaudeShortID
		wsRef, err := cmuxClient.NewWorkspaceWithLayout(ctx, cmuxcli.NewWorkspaceArgs{
			Name:               activation.CmuxName,
			CWD:                activation.WorktreePath,
			AgentAttachCommand: agentCmd,
		})
		if err != nil {
			return FocusResult{Err: fmt.Errorf("failed to create replacement cmux workspace: %w", err)}
		}

		// Resolve agent pane ref.
		panes, err := cmuxClient.ListPanes(ctx, wsRef)
		if err != nil {
			return FocusResult{Err: fmt.Errorf("failed to list panes: %w", err)}
		}
		newPaneRef, err := cmuxcli.AgentPaneRef(panes)
		if err != nil {
			return FocusResult{Err: fmt.Errorf("failed to resolve agent pane ref: %w", err)}
		}

		// Persist new workspace ref and clear orphan flag.
		if mutErr := store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, activation.ActivationID)
			if act == nil {
				return nil
			}
			act.CmuxWorkspaceID = wsRef
			act.AgentPaneRef = newPaneRef
			act.CmuxOrphan = false
			return nil
		}); mutErr != nil {
			return FocusResult{Err: fmt.Errorf("failed to update cmux workspace after respawn: %w", mutErr)}
		}

		currentWsRef = wsRef
		currentPaneRef = newPaneRef
		result.NeedsNewTab = true
	}

	// 3. Focus pane.
	if err := cmuxClient.FocusPane(ctx, currentWsRef, currentPaneRef); err != nil {
		return FocusResult{Err: fmt.Errorf("failed to focus pane: %w", err)}
	}

	// Record last_focused_at.
	now := time.Now()
	if mutErr := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, activation.ActivationID)
		if act == nil {
			return nil
		}
		act.LastFocusedAt = &now
		return nil
	}); mutErr != nil {
		return FocusResult{Err: fmt.Errorf("failed to update last_focused_at: %w", mutErr)}
	}

	return result
}

// focusCmuxCli is the interface Focus requires from the cmuxcli package.
// Defined at the consumer (here) per CONVENTIONS.md.
type focusCmuxCli interface {
	ListWorkspaces(ctx context.Context) ([]cmuxcli.Workspace, error)
	ListPanes(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error)
	NewWorkspaceWithLayout(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error)
	FocusPane(ctx context.Context, wsRef, paneRef string) error
}

// focusClaudeCli is the interface Focus requires from the claudecli package.
type focusClaudeCli interface {
	IsOrphan(ctx context.Context, worktree, shortID string) (bool, error)
}

// focusActivation is the BubbleTea bridge that runs Focus in a goroutine and returns
// the result as a focusResultMsg. This is the single callsite for T-063 focus in the
// Update loop.
func (m Model) focusActivation(entry state.ActivationEntry) (Model, tea.Cmd) {
	// Capture a copy to pass into the goroutine.
	act := entry
	return m, func() tea.Msg {
		result := Focus(m.ctx, m.store, &realCmuxCli{}, &realClaudeCli{}, act.TicketID, &act)
		return focusResultMsg{result: result, activationID: act.ActivationID}
	}
}

// focusResultMsg is emitted when the Focus goroutine completes.
type focusResultMsg struct {
	result       FocusResult
	activationID string
}

// realCmuxCli is the production adapter for focusCmuxCli that delegates to the
// package-level functions.
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

// realClaudeCli is the production adapter for focusClaudeCli.
type realClaudeCli struct{}

func (r *realClaudeCli) IsOrphan(ctx context.Context, worktree, shortID string) (bool, error) {
	return claudecli.IsOrphan(ctx, worktree, shortID)
}
