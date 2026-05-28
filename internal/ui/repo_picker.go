package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/nkzou/cmux-board/internal/config"
)

// newRepoPickerState builds a repoPickerState for the given ticketID.
// When firstTouch is true, rows contains all cfg.Repos entries.
// When firstTouch is false (multi-repo), rows contains only the assignedIDs.
// Stale IDs (in assignedIDs but not in cfg.Repos) are included with stale=true.
func newRepoPickerState(cfg *config.Config, ticketID string, assignedIDs []string, firstTouch bool) *repoPickerState {
	var rows []repoPickerRow
	if firstTouch {
		// All registered repos, sorted by ID.
		ids := make([]string, 0, len(cfg.Repos))
		for id := range cfg.Repos {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			repo := cfg.Repos[id]
			rows = append(rows, repoPickerRow{
				repoID:      id,
				displayName: repo.Name,
				stale:       false,
			})
		}
	} else {
		// Only assigned repos; stale entries are those absent from cfg.Repos.
		for _, id := range assignedIDs {
			repo, ok := cfg.Repos[id]
			displayName := id
			if ok {
				displayName = repo.Name
			}
			rows = append(rows, repoPickerRow{
				repoID:      id,
				displayName: displayName,
				stale:       !ok,
			})
		}
	}
	return &repoPickerState{
		ticketID:   ticketID,
		rows:       rows,
		cursorIdx:  0,
		firstTouch: firstTouch,
	}
}

// renderRepoPicker renders the repo picker overlay.
func (m Model) renderRepoPicker() string {
	if m.repoPicker == nil {
		return ""
	}
	rp := m.repoPicker
	colors := defaultColors()

	titleStyle := lipgloss.NewStyle().Foreground(colors.primary).Bold(true)
	cursorStyle := lipgloss.NewStyle().Background(colors.overlay).Foreground(colors.text)
	normalStyle := lipgloss.NewStyle().Foreground(colors.subtext)
	staleStyle := lipgloss.NewStyle().Foreground(colors.warning)
	footerStyle := lipgloss.NewStyle().Foreground(colors.muted)

	title := fmt.Sprintf("Select repo: %s", rp.ticketID)
	lines := []string{titleStyle.Render(title), ""}

	for i, row := range rp.rows {
		var text string
		if row.stale {
			text = fmt.Sprintf("%s  %s", row.repoID, staleStyle.Render("[?]"))
		} else {
			text = fmt.Sprintf("%s  %s", row.repoID, row.displayName)
		}
		if i == rp.cursorIdx {
			lines = append(lines, cursorStyle.Render(text))
		} else {
			lines = append(lines, normalStyle.Render(text))
		}
	}

	lines = append(lines, "", footerStyle.Render("j/k navigate  ·  enter select  ·  esc cancel"))

	content := lipgloss.NewStyle().
		Border(columnBorder).
		BorderForeground(colors.primary).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))

	return renderWithOverlay(m.width, m.height, content, colors)
}

