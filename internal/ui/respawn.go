package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/claudecli"
	"github.com/nkzou/cmux-board/internal/cmuxcli"
	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// RespawnResult is the outcome of a RespawnActivation call.
type RespawnResult struct {
	Err error
}

type respawnClaudeCli interface {
	Respawn(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error)
}

type respawnCmuxCli interface {
	ListPanes(ctx context.Context, wsRef string) ([]cmuxcli.Pane, error)
	NewWorkspaceWithLayout(ctx context.Context, args cmuxcli.NewWorkspaceArgs) (string, error)
	FocusPane(ctx context.Context, wsRef, paneRef string) error
}

// RespawnActivation launches a replacement Claude session for an existing
// activation, creates a new additive cmux workspace attached to it, then focuses
// the new agent pane. The existing worktree and branch are reused.
func RespawnActivation(
	ctx context.Context,
	store *state.Store,
	cfg *config.Config,
	cmuxClient respawnCmuxCli,
	claudeClient respawnClaudeCli,
	activationID string,
) RespawnResult {
	snap, _ := store.Snapshot()
	activation, ok := state.FindActivationByID(snap, activationID)
	if !ok {
		return RespawnResult{Err: fmt.Errorf("activation %s not found", activationID)}
	}
	repo, ok := cfg.Repos[activation.RepoID]
	if !ok {
		return RespawnResult{Err: fmt.Errorf("repo %s not registered", activation.RepoID)}
	}

	ticketForPrompt := state.PromptTicket(state.TicketState{Key: activation.TicketID})
	if ts, ok := snap.Tickets[activation.TicketID]; ok {
		ticketForPrompt = state.PromptTicket(ts)
	}

	tmpl := cfg.Claude.StarterPrompt
	if tmpl == "" {
		tmpl = config.DefaultStarterPromptTemplate
	}
	worktreeDir := strings.TrimSuffix(activation.WorktreePath, "/")
	prompt, err := claudecli.RenderPrompt(tmpl, claudecli.PromptData{
		Ticket:       ticketForPrompt,
		Repo:         repo,
		WorktreePath: activation.WorktreePath,
		ApproachName: activation.ApproachName,
	})
	if err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to render starter prompt: %w", err)}
	}

	bgResult, err := claudeClient.Respawn(ctx, claudecli.BGArgs{
		Worktree:       worktreeDir,
		Name:           activation.ClaudeName,
		Prompt:         prompt,
		Model:          cfg.Claude.Model,
		PermissionMode: cfg.Claude.PermissionMode,
	})
	if err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to respawn claude: %w", err)}
	}

	if err := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, activationID)
		if act == nil {
			return fmt.Errorf("activation %s not found after claude respawn", activationID)
		}
		act.ClaudeShortID = bgResult.ShortID
		act.ClaudeSessionID = ""
		act.ClaudeOrphan = false
		act.Step = state.StepClaudeStarted
		act.Complete = false
		return nil
	}); err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to journal claude respawn: %w", err)}
	}

	agentCmd := claudecli.BuildAttachCommand(bgResult.ShortID)
	wsRef, err := cmuxClient.NewWorkspaceWithLayout(ctx, cmuxcli.NewWorkspaceArgs{
		Name:               activation.CmuxName,
		CWD:                worktreeDir,
		AgentAttachCommand: agentCmd,
	})
	if err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to create cmux workspace for respawn: %w", err)}
	}
	panes, err := cmuxClient.ListPanes(ctx, wsRef)
	if err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to list panes for respawned workspace: %w", err)}
	}
	agentPaneRef, err := cmuxcli.AgentPaneRef(panes)
	if err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to resolve respawned agent pane ref: %w", err)}
	}

	if err := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, activationID)
		if act == nil {
			return fmt.Errorf("activation %s not found after cmux respawn", activationID)
		}
		act.CmuxWorkspaceID = wsRef
		act.AgentPaneRef = agentPaneRef
		act.CmuxOrphan = false
		act.Step = state.StepCmuxCreated
		act.Complete = true
		return nil
	}); err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to journal cmux respawn: %w", err)}
	}

	if err := cmuxClient.FocusPane(ctx, wsRef, agentPaneRef); err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to focus respawned pane: %w", err)}
	}
	now := time.Now()
	if err := store.Mutate(func(s *state.State) error {
		act := findActivationMutable(s, activationID)
		if act == nil {
			return fmt.Errorf("activation %s not found after respawn focus", activationID)
		}
		act.LastFocusedAt = &now
		return nil
	}); err != nil {
		return RespawnResult{Err: fmt.Errorf("failed to update respawn focus time: %w", err)}
	}

	return RespawnResult{}
}

func (m Model) tryRespawnActivation(entry state.ActivationEntry) (Model, tea.Cmd) {
	if m.activatingTickets[entry.TicketID] {
		return m.pushToast(entry.TicketID + " is already being activated")
	}
	if m.activatingTickets == nil {
		m.activatingTickets = make(map[string]bool)
	}
	wasEmpty := len(m.activatingTickets) == 0
	m.activatingTickets[entry.TicketID] = true

	cmd := RespawnCmd(m.ctx, m.store, m.cfg, entry)
	if wasEmpty {
		return m, tea.Batch(cmd, spinnerTickCmd())
	}
	return m, cmd
}

// RespawnCmd wraps RespawnActivation as a BubbleTea command.
func RespawnCmd(ctx context.Context, store *state.Store, cfg *config.Config, entry state.ActivationEntry) tea.Cmd {
	return func() tea.Msg {
		result := RespawnActivation(ctx, store, cfg, &realCmuxCli{}, &realClaudeCli{}, entry.ActivationID)
		return respawnDoneMsg{
			TicketID:     entry.TicketID,
			RepoID:       entry.RepoID,
			ActivationID: entry.ActivationID,
			Err:          result.Err,
		}
	}
}

type respawnDoneMsg struct {
	TicketID     string
	RepoID       string
	ActivationID string
	Err          error
}

func (m Model) handleRespawnDone(msg respawnDoneMsg) (Model, tea.Cmd) {
	m = m.clearActivating(msg.TicketID)
	m, refreshCmd := m.refreshSnapshot()

	if m.pickerState != nil && m.pickerState.ticketID == msg.TicketID && m.pickerState.repoID == msg.RepoID {
		entries := state.FindActivations(m.snapshot, msg.TicketID, msg.RepoID)
		m.pickerState.entries = entries
		m.pickerState.cursorIdx = indexActivation(entries, msg.ActivationID)
	}

	if msg.Err != nil {
		mAfter, toastCmd := m.pushToast(userFriendlyError(msg.Err))
		return mAfter, tea.Batch(refreshCmd, toastCmd)
	}
	mAfter, toastCmd := m.pushToast("claude session respawned — new tab opened")
	return mAfter, tea.Batch(refreshCmd, toastCmd)
}

func indexActivation(entries []state.ActivationEntry, activationID string) int {
	if len(entries) == 0 {
		return 0
	}
	for i, entry := range entries {
		if entry.ActivationID == activationID {
			return i
		}
	}
	return 0
}

func (r *realClaudeCli) Respawn(ctx context.Context, args claudecli.BGArgs) (claudecli.BGResult, error) {
	return claudecli.Respawn(ctx, args)
}
