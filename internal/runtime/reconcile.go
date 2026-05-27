package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kevin-zou/cmux-board/internal/claudecli"
	"github.com/kevin-zou/cmux-board/internal/cmuxcli"
	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/git"
	"github.com/kevin-zou/cmux-board/internal/state"
)

// ReconcileIncompleteActivations scans all complete:false activation entries and
// attempts to harvest their external resource IDs from git, claude, and cmux.
// It journals harvested IDs via store.Mutate and promotes complete:true when all
// three resources are observed.
//
// Error policy: scanner failures are logged as warnings and treated as "not found"
// for that scanner. Reconciliation continues for remaining entries. A combined
// error is returned at the end if any scanner calls failed.
func ReconcileIncompleteActivations(ctx context.Context, store *state.Store, cfg *config.Config) error {
	snap, _ := store.Snapshot()
	incomplete := state.AllIncompleteActivations(snap)
	if len(incomplete) == 0 {
		return nil
	}

	var errs []string

	for _, entry := range incomplete {
		foundWorktree, foundClaude, foundCmux := false, false, false

		// ── Scanner 1: git worktrees ──────────────────────────────────────────────
		repo, ok := cfg.Repos[entry.RepoID]
		if !ok {
			slog.Warn("reconcile: repo not registered for activation",
				"activation_id", entry.ActivationID, "repo_id", entry.RepoID)
		} else {
			worktrees, err := git.ListWorktrees(ctx, repo.Path)
			if err != nil {
				slog.Warn("reconcile: git worktree list failed",
					"activation_id", entry.ActivationID, "err", err)
				errs = append(errs, fmt.Sprintf("git[%s]: %v", entry.ActIDShort, err))
			} else {
				for _, wt := range worktrees {
					if strings.Contains(wt.Path, entry.ActIDShort) ||
						strings.Contains(wt.Branch, entry.ActIDShort) {
						harvestedPath := wt.Path
						harvestedBranch := wt.Branch
						foundWorktree = true
						if entry.Step == state.StepStarted {
							if err := store.Mutate(func(s *state.State) error {
								act := findActivationMutable(s, entry.ActivationID)
								if act == nil {
									return nil
								}
								act.WorktreePath = harvestedPath
								act.BranchName = harvestedBranch
								act.Step = state.StepWorktreeCreated
								return nil
							}); err != nil {
								slog.Warn("reconcile: failed to journal worktree harvest",
									"activation_id", entry.ActivationID, "err", err)
								errs = append(errs, fmt.Sprintf("mutate-wt[%s]: %v", entry.ActIDShort, err))
							}
						}
						break
					}
				}
			}
		}

		// ── Scanner 2: claude agents ──────────────────────────────────────────────
		agents, err := claudecli.Agents(ctx, claudecli.AgentsArgs{CWD: entry.WorktreePath})
		if err != nil {
			slog.Warn("reconcile: claude agents failed",
				"activation_id", entry.ActivationID, "err", err)
			errs = append(errs, fmt.Sprintf("claude[%s]: %v", entry.ActIDShort, err))
		} else {
			matches := claudecli.FindByActIDShort(agents, entry.ActIDShort)
			if len(matches) > 0 {
				ag := matches[0]
				foundClaude = true
				if entry.Step == state.StepStarted || entry.Step == state.StepWorktreeCreated {
					harvestedSessionID := ag.SessionID
					harvestedShortID := ""
					if len(ag.SessionID) >= 8 {
						harvestedShortID = ag.SessionID[:8]
					}
					if err := store.Mutate(func(s *state.State) error {
						act := findActivationMutable(s, entry.ActivationID)
						if act == nil {
							return nil
						}
						act.ClaudeSessionID = harvestedSessionID
						act.ClaudeShortID = harvestedShortID
						act.Step = state.StepClaudeStarted
						return nil
					}); err != nil {
						slog.Warn("reconcile: failed to journal claude harvest",
							"activation_id", entry.ActivationID, "err", err)
						errs = append(errs, fmt.Sprintf("mutate-claude[%s]: %v", entry.ActIDShort, err))
					}
				}
			}
		}

		// ── Scanner 3: cmux workspaces ────────────────────────────────────────────
		workspaces, err := cmuxcli.ListWorkspaces(ctx)
		if err != nil {
			slog.Warn("reconcile: cmux list-workspaces failed",
				"activation_id", entry.ActivationID, "err", err)
			errs = append(errs, fmt.Sprintf("cmux[%s]: %v", entry.ActIDShort, err))
		} else {
			for _, ws := range workspaces {
				// Match by act_id_short in title (Codex Finding 6).
				// NEVER match by current_directory (Codex Finding 1 / additive-only).
				if strings.Contains(ws.Title, entry.ActIDShort) {
					harvestedWsRef := ws.Ref
					foundCmux = true
					if entry.Step != state.StepCmuxCreated {
						panes, pErr := cmuxcli.ListPanes(ctx, ws.Ref)
						if pErr != nil {
							slog.Warn("reconcile: cmux list-panes failed",
								"activation_id", entry.ActivationID, "ws_ref", ws.Ref, "err", pErr)
							errs = append(errs, fmt.Sprintf("cmux-panes[%s]: %v", entry.ActIDShort, pErr))
							break
						}
						agentPaneRef, pErr := cmuxcli.AgentPaneRef(panes)
						if pErr != nil {
							slog.Warn("reconcile: agent pane ref resolution failed",
								"activation_id", entry.ActivationID, "ws_ref", ws.Ref, "err", pErr)
							errs = append(errs, fmt.Sprintf("cmux-paneref[%s]: %v", entry.ActIDShort, pErr))
							break
						}
						harvestedPaneRef := agentPaneRef
						if err := store.Mutate(func(s *state.State) error {
							act := findActivationMutable(s, entry.ActivationID)
							if act == nil {
								return nil
							}
							act.CmuxWorkspaceID = harvestedWsRef
							act.AgentPaneRef = harvestedPaneRef
							act.Step = state.StepCmuxCreated
							return nil
						}); err != nil {
							slog.Warn("reconcile: failed to journal cmux harvest",
								"activation_id", entry.ActivationID, "err", err)
							errs = append(errs, fmt.Sprintf("mutate-cmux[%s]: %v", entry.ActIDShort, err))
						}
					}
					break
				}
			}
		}

		// ── Promotion rule ────────────────────────────────────────────────────────
		foundCount := 0
		if foundWorktree {
			foundCount++
		}
		if foundClaude {
			foundCount++
		}
		if foundCmux {
			foundCount++
		}
		if foundCount == 3 {
			if err := store.Mutate(func(s *state.State) error {
				act := findActivationMutable(s, entry.ActivationID)
				if act == nil {
					return nil
				}
				act.Complete = true
				return nil
			}); err != nil {
				slog.Warn("reconcile: failed to promote activation to complete",
					"activation_id", entry.ActivationID, "err", err)
				errs = append(errs, fmt.Sprintf("promote[%s]: %v", entry.ActIDShort, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("reconcile: scanner failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ResumeActivation runs only the missing steps for an incomplete activation entry.
// Called when the user presses r on an [incomplete] picker entry.
// Each missing step is journaled via store.Mutate (same pattern as Activate).
//
// Additive-only: never deletes or modifies external resources.
// If CreateWorktreeAt returns ErrWorktreePathExists, the step is treated as already
// done and the function proceeds to the next step.
func ResumeActivation(
	ctx context.Context,
	store *state.Store,
	cfg *config.Config,
	activationID string,
) (state.ActivationEntry, error) {
	snap, _ := store.Snapshot()
	entry, ok := state.FindActivationByID(snap, activationID)
	if !ok {
		return state.ActivationEntry{}, fmt.Errorf("activation %s not found", activationID)
	}

	repo, ok := cfg.Repos[entry.RepoID]
	if !ok {
		return state.ActivationEntry{}, fmt.Errorf("repo %q not registered", entry.RepoID)
	}

	worktreeDir := strings.TrimSuffix(entry.WorktreePath, "/")

	// Step 1: create worktree if not done.
	if entry.Step == state.StepStarted {
		err := git.CreateWorktreeAt(repo.Path, worktreeDir, entry.BranchName, repo.DefaultBranch)
		if err != nil {
			// ErrWorktreePathExists: treat as already created — proceed.
			if !isWorktreePathExists(err) {
				return state.ActivationEntry{}, fmt.Errorf("failed to create worktree: %w", err)
			}
		}
		if err := store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, activationID)
			if act == nil {
				return fmt.Errorf("activation %s not found after worktree create", activationID)
			}
			act.Step = state.StepWorktreeCreated
			return nil
		}); err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to journal worktree_created: %w", err)
		}
		// Re-read for updated step.
		snap, _ = store.Snapshot()
		if e, ok := state.FindActivationByID(snap, activationID); ok {
			entry = e
		}
	}

	// Step 2: launch claude if not done.
	if entry.Step == state.StepWorktreeCreated {
		tmpl := cfg.Claude.StarterPrompt
		if tmpl == "" {
			tmpl = ""
		}
		bgResult, err := claudecli.LaunchBackground(ctx, claudecli.BGArgs{
			Worktree:       worktreeDir,
			Name:           entry.ClaudeName,
			Prompt:         tmpl,
			Model:          cfg.Claude.Model,
			PermissionMode: cfg.Claude.PermissionMode,
		})
		if err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to launch claude: %w", err)
		}
		if err := store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, activationID)
			if act == nil {
				return fmt.Errorf("activation %s not found after claude launch", activationID)
			}
			act.ClaudeShortID = bgResult.ShortID
			act.Step = state.StepClaudeStarted
			return nil
		}); err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to journal claude_started: %w", err)
		}
		snap, _ = store.Snapshot()
		if e, ok := state.FindActivationByID(snap, activationID); ok {
			entry = e
		}
	}

	// Step 3: create cmux workspace if not done.
	if entry.Step == state.StepClaudeStarted {
		agentCmd := "claude attach " + entry.ClaudeShortID
		wsRef, err := cmuxcli.NewWorkspaceWithLayout(ctx, cmuxcli.NewWorkspaceArgs{
			Name:               entry.CmuxName,
			CWD:                worktreeDir,
			AgentAttachCommand: agentCmd,
		})
		if err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to create cmux workspace: %w", err)
		}
		panes, err := cmuxcli.ListPanes(ctx, wsRef)
		if err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to list panes: %w", err)
		}
		agentPaneRef, err := cmuxcli.AgentPaneRef(panes)
		if err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to resolve agent pane ref: %w", err)
		}
		if err := store.Mutate(func(s *state.State) error {
			act := findActivationMutable(s, activationID)
			if act == nil {
				return fmt.Errorf("activation %s not found after cmux create", activationID)
			}
			act.CmuxWorkspaceID = wsRef
			act.AgentPaneRef = agentPaneRef
			act.Step = state.StepCmuxCreated
			act.Complete = true
			return nil
		}); err != nil {
			return state.ActivationEntry{}, fmt.Errorf("failed to journal cmux_created: %w", err)
		}
	}

	finalSnap, _ := store.Snapshot()
	result, ok2 := state.FindActivationByID(finalSnap, activationID)
	if !ok2 {
		return state.ActivationEntry{}, fmt.Errorf("activation %s missing from state after resume", activationID)
	}
	return *result, nil
}

// findActivationMutable returns a pointer into the live state slice for the given activationID.
// MUST only be called inside a store.Mutate closure. Returns nil on miss.
func findActivationMutable(s *state.State, activationID string) *state.ActivationEntry {
	for ticketID := range s.Activations {
		for i := range s.Activations[ticketID] {
			if s.Activations[ticketID][i].ActivationID == activationID {
				return &s.Activations[ticketID][i]
			}
		}
	}
	return nil
}

// isWorktreePathExists returns true when err wraps git.ErrWorktreePathExists.
func isWorktreePathExists(err error) bool {
	return errors.Is(err, git.ErrWorktreePathExists)
}
