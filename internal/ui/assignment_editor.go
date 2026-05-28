package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
)

// newAssignmentEditorState returns an initialized assignmentEditorState.
// repoIDs is the sorted set of all IDs from cfg.Repos.
// Stale IDs (in assigned_repo_ids but not in cfg.Repos) are appended at the end.
func newAssignmentEditorState(cfg *config.Config, snap *state.State, ticketID string) *assignmentEditorState {
	// Build sorted repo ID list from config.
	repoIDs := make([]string, 0, len(cfg.Repos))
	for id := range cfg.Repos {
		repoIDs = append(repoIDs, id)
	}
	sort.Strings(repoIDs)

	assigned := state.AssignedRepoIDs(snap, ticketID)
	assignedSet := make(map[string]bool, len(assigned))
	for _, id := range assigned {
		assignedSet[id] = true
	}

	// Append stale IDs (assigned but not in config) at the end.
	for _, id := range assigned {
		if _, inConfig := cfg.Repos[id]; !inConfig {
			repoIDs = append(repoIDs, id)
		}
	}

	checked := make(map[string]bool, len(repoIDs))
	for _, id := range repoIDs {
		checked[id] = assignedSet[id]
	}

	return &assignmentEditorState{
		ticketID:  ticketID,
		repoIDs:   repoIDs,
		checked:   checked,
		cursorIdx: 0,
	}
}

// renderAssignmentEditor renders the repo assignment editor overlay.
func (m Model) renderAssignmentEditor() string {
	if m.assignmentEditor == nil {
		return ""
	}
	ed := m.assignmentEditor
	colors := defaultColors()

	titleStyle := lipgloss.NewStyle().Foreground(colors.primary).Bold(true)
	cursorStyle := lipgloss.NewStyle().Background(colors.overlay).Foreground(colors.text)
	normalStyle := lipgloss.NewStyle().Foreground(colors.subtext)
	checkStyle := lipgloss.NewStyle().Foreground(colors.success)
	uncheckStyle := lipgloss.NewStyle().Foreground(colors.muted)
	warningStyle := lipgloss.NewStyle().Foreground(colors.warning)
	footerStyle := lipgloss.NewStyle().Foreground(colors.muted)

	lines := []string{
		titleStyle.Render(fmt.Sprintf("Assign repos: %s", ed.ticketID)),
		"",
	}

	for i, id := range ed.repoIDs {
		_, inConfig := m.cfg.Repos[id]

		var rowText string
		if !inConfig {
			// Stale entry.
			checkbox := checkStyle.Render("[x]")
			if !ed.checked[id] {
				checkbox = uncheckStyle.Render("[ ]")
			}
			rowText = fmt.Sprintf("%s %s  (%s)", checkbox,
				id, warningStyle.Render("!! not registered — remove to clean up"))
		} else {
			repo := m.cfg.Repos[id]
			checkbox := checkStyle.Render("[x]")
			if !ed.checked[id] {
				checkbox = uncheckStyle.Render("[ ]")
			}
			rowText = fmt.Sprintf("%s %s  (%s  %s)", checkbox, id, repo.Name, repo.Path)
		}

		if i == ed.cursorIdx {
			lines = append(lines, cursorStyle.Render(rowText))
		} else {
			lines = append(lines, normalStyle.Render(rowText))
		}
	}

	lines = append(lines, "", footerStyle.Render("space toggle  ·  enter commit  ·  esc cancel"))

	content := lipgloss.NewStyle().
		Border(columnBorder).
		BorderForeground(colors.primary).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))

	return renderWithOverlay(m.width, m.height, content, colors)
}

