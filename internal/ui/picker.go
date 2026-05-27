package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kevin-zou/cmux-board/internal/state"
)

// newPickerState returns an initialized pickerState for (ticketID, repoID).
// Returns nil when 0 or 1 activations exist (caller handles those cases directly).
func newPickerState(snap *state.State, ticketID, repoID string) *pickerState {
	entries := state.FindActivations(snap, ticketID, repoID)
	if len(entries) <= 1 {
		return nil
	}
	return &pickerState{
		ticketID:  ticketID,
		repoID:    repoID,
		entries:   entries,
		cursorIdx: 0,
	}
}

// renderPicker renders the activation picker overlay.
func (m Model) renderPicker() string {
	if m.pickerState == nil {
		return ""
	}
	ps := m.pickerState
	colors := defaultColors()

	title := fmt.Sprintf("Activations: %s [%s]", ps.ticketID, ps.repoID)
	titleStyle := lipgloss.NewStyle().
		Foreground(colors.primary).
		Bold(true)

	lines := []string{titleStyle.Render(title), ""}

	cursorStyle := lipgloss.NewStyle().
		Background(colors.overlay).
		Foreground(colors.text)
	normalStyle := lipgloss.NewStyle().Foreground(colors.subtext)
	badgeStyle := lipgloss.NewStyle().Foreground(colors.warning)

	for i, entry := range ps.entries {
		var relTime string
		if entry.LastFocusedAt != nil {
			relTime = formatDuration(time.Since(*entry.LastFocusedAt)) + " ago"
		} else {
			relTime = "never"
		}

		glyphs := activationGlyphs(entry, m)

		line := fmt.Sprintf("%s / %s  (%s, %s)  %s",
			entry.RepoID,
			entry.ApproachName,
			entry.CmuxName,
			relTime,
			glyphs,
		)
		// Trim trailing whitespace from empty glyphs.
		line = strings.TrimRight(line, " ")

		if i == ps.cursorIdx {
			lines = append(lines, cursorStyle.Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
		_ = badgeStyle
	}

	lines = append(lines, "", lipgloss.NewStyle().Foreground(colors.muted).Render(
		"enter focus  ·  n new  ·  r respawn  ·  d delete  ·  esc cancel",
	))

	content := lipgloss.NewStyle().
		Border(columnBorder).
		BorderForeground(colors.primary).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))

	return renderWithOverlay(m.width, m.height, content, colors)
}

// activationGlyphs returns the concatenated badge string for an activation entry.
// Returns an empty string when the entry is healthy.
func activationGlyphs(entry state.ActivationEntry, m Model) string {
	var parts []string
	if entry.ClaudeOrphan {
		parts = append(parts, "[orphan]")
	}
	if entry.CmuxOrphan {
		parts = append(parts, "[no-tab]")
	}
	if _, ok := m.cfg.Repos[entry.RepoID]; !ok {
		parts = append(parts, "[?]")
	}
	if !entry.Complete {
		n := stepToResourceCount(entry.Step)
		parts = append(parts, fmt.Sprintf("[incomplete %d/3]", n))
	}
	return strings.Join(parts, " ")
}

// stepToResourceCount maps an activation step to the count of confirmed resources.
// 0 = none, 1 = worktree, 2 = +claude, 3 = +cmux (complete).
func stepToResourceCount(step string) int {
	switch step {
	case state.StepWorktreeCreated:
		return 1
	case state.StepClaudeStarted:
		return 2
	case state.StepCmuxCreated:
		// cmux_created but complete==false is an edge case where reconciliation
		// set the step but did not flip complete yet.
		return 2
	default:
		// "started" or any unknown step
		return 0
	}
}

// removeActivation removes the activation entry with the given activationID from s.Activations.
// No-op if not found. Does NOT call any cmux or claude destructor.
// MUST be called inside a store.Mutate closure.
func removeActivation(s *state.State, activationID string) error {
	for ticketID, entries := range s.Activations {
		for i, entry := range entries {
			if entry.ActivationID == activationID {
				s.Activations[ticketID] = append(entries[:i], entries[i+1:]...)
				return nil
			}
		}
	}
	return nil
}
