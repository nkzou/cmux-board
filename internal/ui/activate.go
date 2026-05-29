package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oklog/ulid/v2"

	"github.com/nkzou/cmux-board/internal/claudecli"
	"github.com/nkzou/cmux-board/internal/cmuxcli"
	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/git"
	"github.com/nkzou/cmux-board/internal/state"
)

// actIDShortRegex validates Crockford-base32 lowercase 8-char act_id_short values.
// Crockford-base32 alphabet (lowercase): 0-9 and a-z minus i, l, o, u.
// This is NOT the same as hex — hex uses only [0-9a-f].
const actIDShortRegex = `^[0-9a-hjkmnp-tv-z]{8}$`

// ErrRepoNotRegistered is returned when the requested repoID is absent from cfg.Repos.
type ErrRepoNotRegistered struct {
	RepoID string
}

func (e ErrRepoNotRegistered) Error() string {
	return fmt.Sprintf("repo %q not registered — run cmux-board repos add", e.RepoID)
}

// ActivationHooks provides injectable kill points for fault-injection testing.
// In production, all hooks are nil (no-ops). In tests, hooks are set to
// panic("kill") at the appropriate point to simulate process kills.
//
// Use callHook(fn) to invoke any hook — it is nil-safe and incurs no overhead
// when all hooks are nil (the common production case).
type ActivationHooks struct {
	// Hooks called BEFORE each side effect.
	BeforeInitialJournal func()
	BeforeWorktree       func()
	BeforeClaude         func()
	BeforeCmux           func()

	// Hooks called AFTER the side effect's external call returns,
	// BEFORE the next store.Mutate.
	AfterWorktreeBeforeJournal func()
	AfterClaudeBeforeJournal   func()
	AfterCmuxBeforeJournal     func()

	// Hooks called AFTER each store.Mutate completes.
	AfterWorktreeJournal func()
	AfterClaudeJournal   func()
	AfterCmuxJournal     func()
}

// callHook calls fn if non-nil. Zero-overhead in production (nil check is inlined).
func callHook(fn func()) {
	if fn != nil {
		fn()
	}
}

// hook returns the function field f from h if h is non-nil, otherwise nil.
// Usage: callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.BeforeWorktree }))
func hook(h *ActivationHooks, field func(*ActivationHooks) func()) func() {
	if h == nil {
		return nil
	}
	return field(h)
}

// activateDeps groups injectable side-effect functions.
// In production, all fields are nil and the real package-level functions are used.
// In tests, fields are set to mocks.
type activateDeps struct {
	createWorktree   func(repoPath, absPath, branchName, baseBranch string) error
	launchBackground func(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error)
	newWorkspace     func(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error)
	listPanes        func(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error)
	hooks            *ActivationHooks
}

func (d activateDeps) withDefaults() activateDeps {
	if d.createWorktree == nil {
		d.createWorktree = func(repoPath, absPath, branchName, baseBranch string) error {
			return git.CreateWorktreeAt(repoPath, absPath, branchName, baseBranch)
		}
	}
	if d.launchBackground == nil {
		d.launchBackground = claudecli.LaunchBackground
	}
	if d.newWorkspace == nil {
		d.newWorkspace = cmuxcli.NewWorkspaceWithLayout
	}
	if d.listPanes == nil {
		d.listPanes = cmuxcli.ListPanes
	}
	return d
}

// Activate creates a new activation end-to-end for (ticketID, repoID, approach).
//
// Sequence:
//  1. Resolve repo entry; abort early if missing.
//  2. Generate ULID activation_id BEFORE any side effect.
//  3. Compute resource labels (worktree_path, branch_name, claude_name, cmux_name).
//  4. Uniquify worktree path; on ErrUniquifyExhausted abort BEFORE journaling.
//  5. Journal entry with step:"started" BEFORE first side effect.
//  6. Side effect 1: git.CreateWorktreeAt.
//  7. Side effect 2: claudecli.LaunchBackground (cmd.Dir = worktreePath, NEVER --cwd).
//  8. Side effect 3: cmuxcli.NewWorkspaceWithLayout.
//
// All state mutations go through store.Mutate. Direct *state.State field assignment
// outside a Mutate closure is forbidden (Codex Finding 4).
func Activate(
	ctx context.Context,
	store *state.Store,
	cfg *config.Config,
	ticketID, repoID, approach string,
) (state.ActivationEntry, error) {
	return activate(ctx, store, cfg, ticketID, repoID, approach, activateDeps{})
}

// activate is the testable core; deps are resolved to defaults when nil.
func activate(
	ctx context.Context,
	store *state.Store,
	cfg *config.Config,
	ticketID, repoID, approach string,
	deps activateDeps,
) (state.ActivationEntry, error) {
	deps = deps.withDefaults()

	// 0. Resolve repo entry.
	repo, ok := cfg.Repos[repoID]
	if !ok {
		return state.ActivationEntry{}, ErrRepoNotRegistered{RepoID: repoID}
	}

	// 1. Generate activation_id (ULID) BEFORE any side effect.
	actID := ulid.Make()
	actIDShort := strings.ToLower(actID.String())[:8] // 8-char Crockford-base32, NOT hex

	// 2. Compute resource labels.
	sanitizedApproach := git.SanitizeBranchSegment(approach)
	baseSegment := repoID + "-" + ticketID + "-" + sanitizedApproach + "-" + actIDShort
	branchName := git.SanitizeBranchSegment(baseSegment)
	worktreePath := filepath.Join(cfg.WorktreeBaseDir, baseSegment) + "/"
	claudeName := "cmux-board:" + ticketID + ":" + actIDShort
	cmuxName := ticketID + " [" + actIDShort + "]"

	// 3. Uniquify worktree path (paranoia; act_id_short makes collision vanishingly rare).
	finalPath, suffix, err := Uniquify(worktreePath)
	if err != nil {
		if errors.Is(err, ErrUniquifyExhausted) {
			// Abort BEFORE journaling — no entry written.
			return state.ActivationEntry{}, ErrUniquifyExhausted
		}
		return state.ActivationEntry{}, fmt.Errorf("failed to uniquify worktree path: %w", err)
	}
	if suffix != "" {
		// Apply same suffix to branch_name so labels are consistent.
		// claude_name and cmux_name do NOT receive the suffix (reconciliation
		// matches by act_id_short substring alone, not by suffix).
		branchName = branchName + suffix
	}

	// 3b. Render starter prompt AFTER finalPath is known so {{.WorktreePath}}
	// expands to the actual on-disk path rather than an empty string.
	snap, _ := store.Snapshot()
	ticketForPrompt := state.PromptTicket(state.TicketState{Key: ticketID})
	if ts, ok := snap.Tickets[ticketID]; ok {
		ticketForPrompt = state.PromptTicket(ts)
	}
	promptData := claudecli.PromptData{
		Ticket:       ticketForPrompt,
		Repo:         repo,
		WorktreePath: finalPath,
		ApproachName: approach,
	}
	tmpl := cfg.Claude.StarterPrompt
	if tmpl == "" {
		tmpl = config.DefaultStarterPromptTemplate
	}
	prompt, err := claudecli.RenderPrompt(tmpl, promptData)
	if err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to render starter prompt: %w", err)
	}

	// 4. Journal entry BEFORE any side effect.
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.BeforeInitialJournal }))
	now := time.Now()
	initialEntry := state.ActivationEntry{
		ActivationID: actID.String(),
		ActIDShort:   actIDShort,
		RepoID:       repoID,
		TicketID:     ticketID,
		ApproachName: approach, // original (unsanitized) for display (E14)
		WorktreePath: finalPath,
		BranchName:   branchName,
		ClaudeName:   claudeName,
		CmuxName:     cmuxName,
		Step:         state.StepStarted,
		Complete:     false,
		CreatedAt:    now,
	}
	if err := store.Mutate(func(s *state.State) error {
		if s.Activations == nil {
			s.Activations = make(map[string][]state.ActivationEntry)
		}
		s.Activations[ticketID] = append(s.Activations[ticketID], initialEntry)
		return nil
	}); err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to journal activation start: %w", err)
	}

	// 5. Side effect 1: create git worktree.
	// Strip trailing slash before passing to CreateWorktreeAt.
	worktreeDir := strings.TrimSuffix(finalPath, "/")
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.BeforeWorktree }))
	if err := deps.createWorktree(repo.Path, worktreeDir, branchName, repo.DefaultBranch); err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to create worktree: %w", err)
	}
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.AfterWorktreeBeforeJournal }))
	if err := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, actID.String())
		if act == nil {
			return fmt.Errorf("activation %s not found in state after worktree create", actID.String())
		}
		act.Step = state.StepWorktreeCreated
		act.WorktreePath = finalPath
		act.BranchName = branchName
		return nil
	}); err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to journal worktree_created: %w", err)
	}
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.AfterWorktreeJournal }))

	// 6. Side effect 2: launch claude --bg.
	// Uses cmd.Dir = worktreeDir (NEVER --cwd; see CONVENTIONS Immutable Constraints).
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.BeforeClaude }))
	bgResult, err := deps.launchBackground(ctx, claudecli.BGArgs{
		Worktree:       worktreeDir,
		Name:           claudeName,
		Prompt:         prompt,
		Model:          cfg.Claude.Model,
		PermissionMode: cfg.Claude.PermissionMode,
	})
	if err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to launch claude: %w", err)
	}
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.AfterClaudeBeforeJournal }))
	if err := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, actID.String())
		if act == nil {
			return fmt.Errorf("activation %s not found in state after claude launch", actID.String())
		}
		act.ClaudeShortID = bgResult.ShortID
		act.Step = state.StepClaudeStarted
		return nil
	}); err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to journal claude_started: %w", err)
	}
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.AfterClaudeJournal }))

	// 7. Side effect 3: create cmux workspace.
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.BeforeCmux }))
	agentCmd := claudecli.BuildAttachCommand(bgResult.ShortID)
	wsRef, err := deps.newWorkspace(ctx, cmuxcli.NewWorkspaceArgs{
		Name:               cmuxName,
		CWD:                worktreeDir,
		AgentAttachCommand: agentCmd,
	})
	if err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to create cmux workspace: %w", err)
	}
	panes, err := deps.listPanes(ctx, wsRef)
	if err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to list panes for new workspace: %w", err)
	}
	agentPaneRef, err := cmuxcli.AgentPaneRef(panes)
	if err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to resolve agent pane ref: %w", err)
	}
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.AfterCmuxBeforeJournal }))

	now2 := time.Now()
	if err := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, actID.String())
		if act == nil {
			return fmt.Errorf("activation %s not found in state after cmux create", actID.String())
		}
		act.CmuxWorkspaceID = wsRef
		act.AgentPaneRef = agentPaneRef
		act.Step = state.StepCmuxCreated
		act.Complete = true
		act.LastFocusedAt = &now2
		return nil
	}); err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to journal cmux_created: %w", err)
	}
	callHook(hook(deps.hooks, func(h *ActivationHooks) func() { return h.AfterCmuxJournal }))

	// Return the final committed entry from a fresh snapshot.
	finalSnap, _ := store.Snapshot()
	result, ok2 := state.FindActivationByID(finalSnap, actID.String())
	if !ok2 {
		return state.ActivationEntry{}, fmt.Errorf("activation %s missing from state after completion", actID.String())
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

// ActivateCmd wraps Activate as a BubbleTea command.
// The returned tea.Cmd runs Activate in a goroutine and emits activationDoneMsg on completion.
func ActivateCmd(
	ctx context.Context,
	store *state.Store,
	cfg *config.Config,
	ticketID, repoID, approach string,
) tea.Cmd {
	return func() tea.Msg {
		entry, err := Activate(ctx, store, cfg, ticketID, repoID, approach)
		activationID := ""
		if err == nil {
			activationID = entry.ActivationID
		}
		return activationDoneMsg{
			TicketID:     ticketID,
			ActivationID: activationID,
			Err:          err,
		}
	}
}
