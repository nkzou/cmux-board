package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/nkzou/cmux-board/internal/config"
	"github.com/nkzou/cmux-board/internal/state"
	"github.com/nkzou/cmux-board/internal/tracker"
)

// Model is the root BubbleTea model for the cmux-board dock UI.
// It implements tea.Model: Init, Update, and View.
//
// All external I/O (polling, activation, focus) is performed in tea.Cmd goroutines
// that communicate results back as tea.Msg values. Update itself is non-blocking.
type Model struct {
	// Startup context — captured at NewModel time; all I/O Cmds close over this.
	// Cancelled when the dock shuts down.
	ctx context.Context

	// Core data (read from store snapshots)
	cfg         *config.Config
	store       *state.Store
	tr          tracker.IssueTracker // may be nil in tests (drag no-ops when nil)
	snapshot    *state.State
	snapshotRev uint64

	// Board render state
	board           state.BoardSnapshot
	tickets         []state.TicketState // visible (removed_at == nil), sorted by column
	unmappedTickets []state.TicketState
	activeColIdx    int
	activeTicketIdx int

	// UI mode
	mode Mode

	// Previous mode — used by handleApproachNameMode to restore context on Esc.
	previousMode Mode

	// Inline inputs (one each; only one active at a time)
	approachNameInput textinput.Model
	filterInput       textinput.Model
	filterQuery       string

	// Overlay sub-states (nil when not active)
	pickerState      *pickerState
	assignmentEditor *assignmentEditorState
	repoPicker       *repoPickerState

	// Drag state (T-065)
	dragging         bool
	dragTicketID     string
	dragFromColumn   string // column ID at drag-start; used for snap-back on conflict
	dragTargetColumn string // column ID of current hover target

	// Status pills
	trackerPill pillState
	cmuxPill    pillState
	claudePill  pillState

	// Toast queue
	toasts []toastEntry

	// Poll failure state (for backoff display)
	pollFailedAt *time.Time
	pollErrCode  string

	// Window dimensions (set on tea.WindowSizeMsg)
	width  int
	height int
}

// NewModel constructs a Model from cfg and store. Takes an initial snapshot so the
// board is populated before the first render. The approach-name input is initialized
// but not focused; it is focused when the mode transitions to ModeApproachName.
// ctx is stored for use by I/O Cmds; pass context.Background() in tests.
func NewModel(cfg *config.Config, store *state.Store) Model {
	return NewModelWithContext(context.Background(), cfg, store)
}

// NewModelWithContext is like NewModel but accepts an explicit context for production use.
// The dock cobra command should pass a context that it cancels on shutdown.
func NewModelWithContext(ctx context.Context, cfg *config.Config, store *state.Store) Model {
	input := textinput.New()
	input.Placeholder = "approach name"
	input.CharLimit = 64

	fi := textinput.New()
	fi.Placeholder = "Filter tickets..."
	fi.CharLimit = 100
	fi.Width = 30

	snap, rev := store.Snapshot()

	return Model{
		ctx:               ctx,
		cfg:               cfg,
		store:             store,
		snapshot:          snap,
		snapshotRev:       rev,
		board:             snap.Board,
		mode:              ModeNormal,
		approachNameInput: input,
		filterInput:       fi,
	}
}

// Init implements tea.Model. Returns a batch that enters the alt screen and fires the
// first poll tick. The poller goroutine is started externally (by the cobra command's
// Run function); tickPollMsg triggers the first poll request via the event loop.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg { return tickPollMsg{} },
	)
}
