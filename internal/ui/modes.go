package ui

// Mode controls which key handler is active and which overlays are visible.
type Mode int

const (
	ModeNormal           Mode = iota
	ModePicker                // activation picker overlay open
	ModeApproachName          // text input for naming a new approach
	ModeAssignmentEditor      // repo assignment editor overlay open
	ModeConfirm               // confirmation prompt (currently unused in v1)
	ModeHelp                  // help overlay
	ModeFilter                // ticket filter input active
	ModeShuttingDown          // terminal clean-up in progress
	ModeRepoPicker            // repo picker overlay (first-touch or multi-repo disambiguation)
)
