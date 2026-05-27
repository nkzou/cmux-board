package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oklog/ulid/v2"

	"github.com/kevin-zou/cmux-board/internal/claudecli"
	"github.com/kevin-zou/cmux-board/internal/cmuxcli"
	"github.com/kevin-zou/cmux-board/internal/config"
	"github.com/kevin-zou/cmux-board/internal/git"
	"github.com/kevin-zou/cmux-board/internal/state"
	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// actIDShortRegex validates Crockford-base32 lowercase 8-char act_id_short values.
// Crockford-base32 alphabet (lowercase): 0-9 and a-z minus i, l, o, u.
// This is NOT the same as hex — hex uses only [0-9a-f].
const actIDShortRegex = `^[0-9a-hjkmnp-tv-z]{8}$`

// actIDShortRE is the compiled actIDShortRegex.
var actIDShortRE = regexp.MustCompile(actIDShortRegex)

// ErrRepoNotRegistered is returned when the requested repoID is absent from cfg.Repos.
type ErrRepoNotRegistered struct {
	RepoID string
}

func (e ErrRepoNotRegistered) Error() string {
	return fmt.Sprintf("repo %q not registered — run cmux-board repos add", e.RepoID)
}

// activateDeps groups injectable side-effect functions.
// In production, all fields are nil and the real package-level functions are used.
// In tests, fields are set to mocks.
type activateDeps struct {
	createWorktree   func(repoPath, absPath, branchName, baseBranch string) error
	launchBackground func(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error)
	newWorkspace     func(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error)
	listPanes        func(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error)
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

	// 0b. Render starter prompt.
	snap, _ := store.Snapshot()
	var ticketForPrompt tracker.Ticket
	if ts, ok := snap.Tickets[ticketID]; ok {
		ticketForPrompt = tracker.Ticket{
			Key:     ts.Key,
			Summary: ts.Summary,
			Status:  ts.Status,
			URL:     ts.URL,
		}
	}
	promptData := claudecli.PromptData{
		Ticket:       ticketForPrompt,
		Repo:         repo,
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

	// 4. Journal entry BEFORE any side effect.
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
	if err := deps.createWorktree(repo.Path, worktreeDir, branchName, repo.DefaultBranch); err != nil {
		return state.ActivationEntry{}, fmt.Errorf("failed to create worktree: %w", err)
	}
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

	// 6. Side effect 2: launch claude --bg.
	// Uses cmd.Dir = worktreeDir (NEVER --cwd; see CONVENTIONS Immutable Constraints).
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

	// 7. Side effect 3: create cmux workspace.
	agentCmd := "claude attach " + bgResult.ShortID
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

	// Return the final committed entry from a fresh snapshot.
	finalSnap, _ := store.Snapshot()
	result, ok2 := state.FindActivationByID(finalSnap, actID.String())
	if !ok2 {
		return state.ActivationEntry{}, fmt.Errorf("activation %s missing from state after completion", actID.String())
	}
	return *result, nil
}

// generateActID generates a new ULID and returns its full string form and the
// 8-char Crockford-base32 lowercase short form.
func generateActID() (ulid.ULID, string) {
	actID := ulid.Make()
	return actID, strings.ToLower(actID.String())[:8]
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
		return activationDoneMsg{ActivationID: activationID, Err: err}
	}
}
