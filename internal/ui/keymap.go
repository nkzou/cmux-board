package ui

import (
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkzou/cmux-board/internal/state"
)

// Key binding constants. All key string literals in handlers MUST use these constants;
// bare string literals in switch cases are forbidden (T-057 mandatory invariant 3).
const (
	// Normal mode navigation (vim-style)
	KeyLeft  = "h"
	KeyDown  = "j"
	KeyUp    = "k"
	KeyRight = "l"

	// Normal mode actions
	KeyActivate    = "enter"
	KeyNewApproach = "N" // capital N — new approach regardless of existing activations
	KeyManage      = "m" // open activation picker for current ticket regardless of count
	KeyAssignRepos = "a" // open assignment editor
	KeyFilter      = "/"
	KeyHelp        = "?"
	KeyQuit        = "q"

	// Post-it board keybinds (T-501).
	KeyImportJira   = "i"
	KeyCreateLocal  = "c"
	KeyRemoveTicket = "x"
	KeyCycleStatus  = "s"
	KeyZCycleNext   = "tab"
	KeyZCyclePrev   = "shift+tab"

	// Picker mode
	KeyPickerFocus   = "enter"
	KeyPickerNew     = "n" // lowercase n — new approach from picker
	KeyPickerRespawn = "r"
	KeyPickerDelete  = "d"
	KeyPickerCancel  = "esc"

	// Assignment editor
	KeyAssignToggle = " " // space
	KeyAssignCommit = "enter"
	KeyAssignCancel = "esc"

	// Approach-name input
	KeyApproachCommit = "enter"
	KeyApproachCancel = "esc"

	// Universal
	KeyEsc = "esc"
)

// handleNormalMode handles key events when mode == ModeNormal.
func (m Model) handleNormalMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	snap := m.snapshot
	// Reconcile zOrder + selection against the current snapshot before dispatching.
	// View() reconciles on a value-copy that never reaches Update, so on the first
	// keypress selectedKey is "" and remove/cycle/arrows short-circuit. Reconciling
	// here ensures every keybind sees an up-to-date selection.
	if snap != nil {
		m.reconcileZOrder(snap)
	}
	if snap == nil || len(snap.Tickets) == 0 {
		// No tickets — only mode-switching and quit are meaningful.
		switch msg.String() {
		case KeyHelp:
			m.mode = ModeHelp
		case KeyQuit:
			m.mode = ModeShuttingDown
			return m, tea.Quit
		case KeyFilter:
			m.mode = ModeFilter
			m.filterInput.Focus()
		case KeyImportJira:
			m.mode = ModeImportInput
			m.importInput.SetValue("")
			m.importInput.Focus()
		case KeyCreateLocal:
			m.mode = ModeCreateInput
			m.createInput.SetValue("")
			m.createInput.Focus()
		}
		return m, nil
	}

	switch msg.String() {
	case KeyLeft, "left":
		if next := nearestWest(snap, m.selectedKey); next != "" {
			m.selectedKey = next
		}
	case KeyRight, "right":
		if next := nearestEast(snap, m.selectedKey); next != "" {
			m.selectedKey = next
		}
	case KeyDown, "down":
		if next := nearestSouth(snap, m.selectedKey); next != "" {
			m.selectedKey = next
		}
	case KeyUp, "up":
		if next := nearestNorth(snap, m.selectedKey); next != "" {
			m.selectedKey = next
		}
	case KeyZCycleNext:
		m.selectedKey = zCycleNext(m.zOrder, m.selectedKey)
	case KeyZCyclePrev:
		m.selectedKey = zCyclePrev(m.zOrder, m.selectedKey)
	case KeyActivate:
		if m.selectedKey == "" {
			return m, nil
		}
		return m.tryActivate(m.selectedKey)
	case KeyNewApproach:
		// Capital N: new approach regardless of existing activations (F16).
		m.previousMode = ModeNormal
		m.mode = ModeApproachName
		m.approachNameInput.SetValue("")
		m.approachNameInput.Focus()
	case KeyManage:
		if m.selectedKey == "" {
			return m, nil
		}
		return m.openManageActivations(m.selectedKey)
	case KeyAssignRepos:
		if m.selectedKey == "" {
			return m, nil
		}
		m.assignmentEditor = newAssignmentEditorState(m.cfg, m.snapshot, m.selectedKey)
		m.mode = ModeAssignmentEditor
		return m, nil
	case KeyImportJira:
		m.mode = ModeImportInput
		m.importInput.SetValue("")
		m.importInput.Focus()
	case KeyCreateLocal:
		m.mode = ModeCreateInput
		m.createInput.SetValue("")
		m.createInput.Focus()
	case KeyRemoveTicket:
		if m.selectedKey == "" {
			return m, nil
		}
		key := m.selectedKey
		if err := m.store.Mutate(func(s *state.State) error {
			state.RemoveTicket(s, key)
			return nil
		}); err != nil {
			return m.pushToast("remove failed: " + err.Error())
		}
		snap2, rev := m.store.Snapshot()
		m.snapshot = snap2
		m.snapshotRev = rev
	case KeyCycleStatus:
		if m.selectedKey == "" {
			return m, nil
		}
		key := m.selectedKey
		if err := m.store.Mutate(func(s *state.State) error {
			state.CycleStatus(s, key)
			return nil
		}); err != nil {
			return m.pushToast("cycle status failed: " + err.Error())
		}
		snap2, rev := m.store.Snapshot()
		m.snapshot = snap2
		m.snapshotRev = rev
	case KeyFilter:
		m.mode = ModeFilter
		m.filterInput.Focus()
	case KeyHelp:
		m.mode = ModeHelp
	case KeyQuit:
		m.mode = ModeShuttingDown
		return m, tea.Quit
	}
	return m, nil
}

// nearestEast returns the key of the nearest ticket strictly to the east of fromKey.
// East means t.X > from.X; ties broken by smallest dx then smallest |dy|.
// Returns "" if no ticket qualifies.
func nearestEast(snap *state.State, fromKey string) string {
	from, ok := snap.Tickets[fromKey]
	if !ok {
		return ""
	}
	best := ""
	bestDx, bestDy := math.MaxInt, math.MaxInt
	for k, t := range snap.Tickets {
		if k == fromKey || t.X <= from.X {
			continue
		}
		dx := t.X - from.X
		dy := abs(t.Y - from.Y)
		if dx < bestDx || (dx == bestDx && dy < bestDy) {
			best, bestDx, bestDy = k, dx, dy
		}
	}
	return best
}

// nearestWest returns the key of the nearest ticket strictly to the west of fromKey.
func nearestWest(snap *state.State, fromKey string) string {
	from, ok := snap.Tickets[fromKey]
	if !ok {
		return ""
	}
	best := ""
	bestDx, bestDy := math.MaxInt, math.MaxInt
	for k, t := range snap.Tickets {
		if k == fromKey || t.X >= from.X {
			continue
		}
		dx := from.X - t.X
		dy := abs(t.Y - from.Y)
		if dx < bestDx || (dx == bestDx && dy < bestDy) {
			best, bestDx, bestDy = k, dx, dy
		}
	}
	return best
}

// nearestSouth returns the key of the nearest ticket strictly to the south (higher Y).
func nearestSouth(snap *state.State, fromKey string) string {
	from, ok := snap.Tickets[fromKey]
	if !ok {
		return ""
	}
	best := ""
	bestDy, bestDx := math.MaxInt, math.MaxInt
	for k, t := range snap.Tickets {
		if k == fromKey || t.Y <= from.Y {
			continue
		}
		dy := t.Y - from.Y
		dx := abs(t.X - from.X)
		if dy < bestDy || (dy == bestDy && dx < bestDx) {
			best, bestDy, bestDx = k, dy, dx
		}
	}
	return best
}

// nearestNorth returns the key of the nearest ticket strictly to the north (lower Y).
func nearestNorth(snap *state.State, fromKey string) string {
	from, ok := snap.Tickets[fromKey]
	if !ok {
		return ""
	}
	best := ""
	bestDy, bestDx := math.MaxInt, math.MaxInt
	for k, t := range snap.Tickets {
		if k == fromKey || t.Y >= from.Y {
			continue
		}
		dy := from.Y - t.Y
		dx := abs(t.X - from.X)
		if dy < bestDy || (dy == bestDy && dx < bestDx) {
			best, bestDy, bestDx = k, dy, dx
		}
	}
	return best
}

// zCycleNext advances selectedKey to the next entry in zOrder (wraps around).
// Returns selectedKey unchanged if it is not in zOrder or zOrder is empty.
func zCycleNext(zOrder []string, selectedKey string) string {
	if len(zOrder) == 0 {
		return selectedKey
	}
	for i, k := range zOrder {
		if k == selectedKey {
			return zOrder[(i+1)%len(zOrder)]
		}
	}
	// Not found: select first.
	return zOrder[0]
}

// zCyclePrev retreats selectedKey to the previous entry in zOrder (wraps around).
// Returns selectedKey unchanged if it is not in zOrder or zOrder is empty.
func zCyclePrev(zOrder []string, selectedKey string) string {
	if len(zOrder) == 0 {
		return selectedKey
	}
	for i, k := range zOrder {
		if k == selectedKey {
			return zOrder[(i-1+len(zOrder))%len(zOrder)]
		}
	}
	// Not found: select last.
	return zOrder[len(zOrder)-1]
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// handlePickerMode handles key events when mode == ModePicker.
func (m Model) handlePickerMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.pickerState == nil {
		m.mode = ModeNormal
		return m, nil
	}
	ps := m.pickerState
	switch msg.String() {
	case KeyPickerFocus:
		if len(ps.entries) == 0 {
			return m, nil
		}
		entry := ps.entries[ps.cursorIdx]
		m.pickerState = nil
		m.mode = ModeNormal
		return m.focusActivation(entry)
	case KeyPickerNew:
		m.pickerState = nil
		m.previousMode = ModePicker
		m.mode = ModeApproachName
		m.approachNameInput.SetValue("")
		m.approachNameInput.Focus()
	case KeyPickerRespawn:
		// TODO(T-063): respawn selected activation.
		return m, nil
	case KeyPickerDelete:
		if len(ps.entries) == 0 {
			return m, nil
		}
		entry := ps.entries[ps.cursorIdx]
		activationID := entry.ActivationID
		if err := m.store.Mutate(func(s *state.State) error {
			return removeActivation(s, activationID)
		}); err != nil {
			return m.pushToast("delete failed: " + err.Error())
		}
		// Re-fetch from updated snapshot.
		snap, rev := m.store.Snapshot()
		m.snapshot = snap
		m.snapshotRev = rev
		newEntries := state.FindActivations(snap, ps.ticketID, ps.repoID)
		if len(newEntries) == 0 {
			m.pickerState = nil
			m.mode = ModeNormal
			return m, nil
		}
		m.pickerState = &pickerState{
			ticketID:  ps.ticketID,
			repoID:    ps.repoID,
			entries:   newEntries,
			cursorIdx: min(ps.cursorIdx, len(newEntries)-1),
		}
	case KeyPickerCancel: // == KeyEsc == "esc"
		m.pickerState = nil
		m.mode = ModeNormal
	case KeyDown, "down":
		if ps.cursorIdx < len(ps.entries)-1 {
			m.pickerState.cursorIdx++
		}
	case KeyUp, "up":
		if ps.cursorIdx > 0 {
			m.pickerState.cursorIdx--
		}
	}
	return m, nil
}

// handleApproachNameMode routes key events to the text input and handles commit/cancel.
func (m Model) handleApproachNameMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case KeyApproachCommit:
		name := m.approachNameInput.Value()
		if name == "" {
			return m, nil
		}
		m.approachNameInput.SetValue("")
		m.approachNameInput.Blur()
		m.mode = ModeNormal
		// Determine (ticketID, repoID) from context.
		var ticketID, repoID string
		if m.previousMode == ModePicker && m.pickerState != nil {
			ticketID = m.pickerState.ticketID
			repoID = m.pickerState.repoID
			m.pickerState = nil
		} else {
			// TODO(T-402c): use m.selectedKey for ticketID once navigation is wired.
			// Guard until T-402c: no-op if no selectedKey.
			return m, nil
		}
		m.previousMode = ModeNormal
		return m.tryActivateWithRepo(ticketID, repoID, name)
	case KeyApproachCancel:
		m.approachNameInput.SetValue("")
		m.approachNameInput.Blur()
		// Restore the mode we came from (ModeNormal or ModePicker).
		m.mode = m.previousMode
		m.previousMode = ModeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.approachNameInput, cmd = m.approachNameInput.Update(msg)
		return m, cmd
	}
}

// handleAssignmentEditorMode handles key events in ModeAssignmentEditor.
func (m Model) handleAssignmentEditorMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.assignmentEditor == nil {
		m.mode = ModeNormal
		return m, nil
	}
	ed := m.assignmentEditor
	switch msg.String() {
	case KeyAssignToggle:
		if len(ed.repoIDs) == 0 {
			return m, nil
		}
		id := ed.repoIDs[ed.cursorIdx]
		m.assignmentEditor.checked[id] = !m.assignmentEditor.checked[id]
	case KeyAssignCommit:
		// Build newAssigned: all repoIDs where checked == true, in repoIDs order.
		newAssigned := make([]string, 0, len(ed.repoIDs))
		for _, id := range ed.repoIDs {
			if ed.checked[id] {
				newAssigned = append(newAssigned, id)
			}
		}
		ticketID := ed.ticketID
		if err := m.store.Mutate(func(s *state.State) error {
			return state.ApplyAssignments(s, ticketID, newAssigned)
		}); err != nil {
			return m.pushToast("assignment failed: " + err.Error())
		}
		snap, rev := m.store.Snapshot()
		m.snapshot = snap
		m.snapshotRev = rev
		m.assignmentEditor = nil
		m.mode = ModeNormal
	case KeyAssignCancel:
		m.assignmentEditor = nil
		m.mode = ModeNormal
	case KeyDown, "down":
		if ed.cursorIdx < len(ed.repoIDs)-1 {
			m.assignmentEditor.cursorIdx++
		}
	case KeyUp, "up":
		if ed.cursorIdx > 0 {
			m.assignmentEditor.cursorIdx--
		}
	}
	return m, nil
}

// handleRepoPickerMode handles key events in ModeRepoPicker (first-touch / multi-repo picker).
func (m Model) handleRepoPickerMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.repoPicker == nil {
		m.mode = ModeNormal
		return m, nil
	}
	rp := m.repoPicker
	switch msg.String() {
	case KeyPickerFocus: // "enter" — same constant as KeyActivate
		if len(rp.rows) == 0 {
			return m, nil
		}
		row := rp.rows[rp.cursorIdx]
		if row.stale {
			// Stale repo: surface toast and cancel (E-MR1).
			m.repoPicker = nil
			m.mode = ModeNormal
			return m.pushToast("repo " + row.repoID + " not registered — use picker key `a` to remove it")
		}
		ticketID := rp.ticketID
		firstTouch := rp.firstTouch
		manageIntent := rp.manageIntent
		chosenID := row.repoID
		m.repoPicker = nil
		m.mode = ModeNormal

		if firstTouch {
			// Append chosen repo to assigned_repo_ids.
			if err := m.store.Mutate(func(s *state.State) error {
				return state.AddAssignment(s, ticketID, chosenID)
			}); err != nil {
				return m.pushToast("assign failed: " + err.Error())
			}
			snap, rev := m.store.Snapshot()
			m.snapshot = snap
			m.snapshotRev = rev
		}

		if manageIntent {
			// Open activation picker for the chosen repo without activating.
			m.pickerState = forcePickerState(m.snapshot, ticketID, chosenID)
			m.mode = ModePicker
			return m, nil
		}

		// Normal activation flow: check activations for (ticketID, chosenID).
		ps := newPickerState(m.snapshot, ticketID, chosenID)
		if ps == nil {
			activations := state.FindActivations(m.snapshot, ticketID, chosenID)
			if len(activations) == 0 {
				return m.tryActivateWithRepo(ticketID, chosenID, "")
			}
			// 1 activation → focus directly (T-063).
			return m.focusActivation(activations[0])
		}
		// 2+ activations: open activation picker.
		m.pickerState = ps
		m.mode = ModePicker
		return m, nil

	case KeyEsc:
		m.repoPicker = nil
		m.mode = ModeNormal

	case KeyDown, "down":
		if rp.cursorIdx < len(rp.rows)-1 {
			m.repoPicker.cursorIdx++
		}
	case KeyUp, "up":
		if rp.cursorIdx > 0 {
			m.repoPicker.cursorIdx--
		}
	}
	return m, nil
}

// handleHelpMode exits on any key.
func (m Model) handleHelpMode(_ tea.KeyMsg) (Model, tea.Cmd) {
	m.mode = ModeNormal
	return m, nil
}
