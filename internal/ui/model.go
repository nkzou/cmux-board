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

// dragState tracks an in-flight mouse drag.
// It is in-memory only and never persisted (no JSON tags).
// Cleared to nil on mouse-release (T-404).
type dragState struct {
	key            string // ticket key being dragged
	pressX, pressY int    // cursor position at mouse-press
	motion         bool   // true once cursor moves from press position
	// Live drag position. Updated on every motion event; canvas reads these
	// when rendering the card being dragged so movement looks smooth instead of
	// snapping from press to release.
	currentX, currentY int
}

// Model is the root BubbleTea model for cmux-board.
// It implements tea.Model: Init, Update, and View.
//
// All external I/O (polling, activation, focus) is performed in tea.Cmd goroutines
// that communicate results back as tea.Msg values. Update itself is non-blocking.
type Model struct {
	// Startup context — captured at NewModel time; all I/O Cmds close over this.
	// Cancelled when cmux-board shuts down.
	// CONVENTIONS exception: context-in-struct is explicitly documented here
	// because the BubbleTea Model owns goroutine lifetime.
	ctx context.Context

	// Core data (read from store snapshots)
	cfg         *config.Config
	store       *state.Store
	tr          tracker.IssueTracker // may be nil in tests
	snapshot    *state.State
	snapshotRev uint64

	// Freeform board navigation state (M-4)
	selectedKey string     // key of the currently selected post-it; "" = none
	zOrder      []string   // render order: last element is topmost (on top)
	dragging    *dragState // non-nil while a drag is in flight

	// UI mode
	mode Mode

	// Previous mode — used by handleApproachNameMode to restore context on Esc.
	previousMode Mode

	// Inline inputs (one each; only one active at a time)
	approachNameInput textinput.Model
	filterInput       textinput.Model
	filterQuery       string
	importInput       textinput.Model // Jira-key import overlay (T-502)
	createInput       textinput.Model // local-ticket creation overlay (T-503)

	// Overlay sub-states (nil when not active)
	pickerState      *pickerState
	assignmentEditor *assignmentEditorState
	repoPicker       *repoPickerState

	// Status pills
	trackerPill pillState
	cmuxPill    pillState
	claudePill  pillState

	// Toast queue
	toasts []toastEntry

	// Activation in-flight tracking. Key is ticketID; presence means an activation
	// is currently running for that ticket and a second one must be blocked.
	// spinnerFrame is advanced by spinnerTickMsg to animate the indicator on
	// activating tickets. The tick is only armed while activatingTickets is
	// non-empty; otherwise it stays at 0.
	activatingTickets map[string]bool
	spinnerFrame      int

	// Poll failure state (for backoff display)
	pollFailedAt *time.Time
	pollErrCode  string

	// Window dimensions (set on tea.WindowSizeMsg)
	width  int
	height int

	// Mouse-event diagnostic counters. Visible in the status bar so the user can
	// confirm whether mouse messages are arriving from the terminal at all
	// (relevant inside multiplexers like cmux/tmux). Reset on app restart.
	// Only rendered when debug is true (set via WithDebug from --debug flag).
	mousePressCount   int
	mouseMotionCount  int
	mouseReleaseCount int
	// Last-drag diagnostic — shows whether press hit a zone, whether motion was
	// detected, and the final committed (x, y) so we can see drag-handler health
	// at a glance.
	dragDebug string
	// debug gates the visibility of the counters above and dragDebug. False
	// by default; opt in via the --debug flag on dock.
	debug bool
}

// WithDebug returns a copy of m with the debug flag set to enabled. When true
// the status bar exposes mouse-event counters and the last-drag diagnostic.
func (m Model) WithDebug(enabled bool) Model {
	m.debug = enabled
	return m
}

// NewModel constructs a Model from cfg and store. Takes an initial snapshot so the
// board is populated before the first render. The approach-name input is initialized
// but not focused; it is focused when the mode transitions to ModeApproachName.
// ctx is stored for use by I/O Cmds; pass context.Background() in tests.
func NewModel(cfg *config.Config, store *state.Store) Model {
	return NewModelWithContext(context.Background(), cfg, store)
}

// NewModelWithTracker is like NewModelWithContext but also accepts an IssueTracker.
// Used in tests and in the production dock command after T-502 adds import functionality.
func NewModelWithTracker(ctx context.Context, cfg *config.Config, store *state.Store, tr tracker.IssueTracker) Model {
	m := NewModelWithContext(ctx, cfg, store)
	m.tr = tr
	return m
}

// NewModelWithContext is like NewModel but accepts an explicit context for production use.
// The `dock` cobra subcommand should pass a context that it cancels on shutdown.
func NewModelWithContext(ctx context.Context, cfg *config.Config, store *state.Store) Model {
	input := textinput.New()
	input.Placeholder = "approach name"
	input.CharLimit = 64

	fi := textinput.New()
	fi.Placeholder = "Filter tickets..."
	fi.CharLimit = 100
	fi.Width = 30

	ii := textinput.New()
	ii.Placeholder = "JIRA-123"
	ii.CharLimit = 64

	ci := textinput.New()
	ci.Placeholder = "ticket name"
	ci.CharLimit = 128

	snap, rev := store.Snapshot()

	return Model{
		ctx:               ctx,
		cfg:               cfg,
		store:             store,
		snapshot:          snap,
		snapshotRev:       rev,
		selectedKey:       "",
		zOrder:            nil,
		dragging:          nil,
		mode:              ModeNormal,
		approachNameInput: input,
		filterInput:       fi,
		importInput:       ii,
		createInput:       ci,
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
