package state

import "time"

// SchemaVersionCurrent is the only schema version written by this binary.
const SchemaVersionCurrent = 3

// State is stored at ~/.config/cmux-board/state.json (mode 0644).
type State struct {
	SchemaVersion int                          `json:"schema_version"`
	Tickets       map[string]TicketState       `json:"tickets,omitempty"`     // keyed by ticket key
	Activations   map[string][]ActivationEntry `json:"activations,omitempty"` // keyed by ticket key
}

// TicketState holds per-ticket cached data from the tracker plus local-only fields.
// Source is "jira" for imported tickets and "local" for user-created local tickets.
type TicketState struct {
	Key             string     `json:"key"`
	Source          string     `json:"source"`                 // "jira" | "local"
	Summary         string     `json:"summary,omitempty"`
	Status          string     `json:"status,omitempty"`        // Jira-mirrored; empty for local tickets
	LocalStatus     string     `json:"local_status,omitempty"` // local tickets only
	Priority        string     `json:"priority,omitempty"`
	AssigneeID      string     `json:"assignee_id,omitempty"`
	URL             string     `json:"url,omitempty"`           // used by activate.go:186
	Labels          []string   `json:"labels,omitempty"`
	X               int        `json:"x"`
	Y               int        `json:"y"`
	AssignedRepoIDs []string   `json:"assigned_repo_ids,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

// ActivationEntry is one (ticket, repo) activation journal entry.
// Fields are journaled incrementally; complete==false means a prior run was interrupted.
type ActivationEntry struct {
	// Durable keys
	ActivationID string `json:"activation_id"`  // full ULID (canonical durable key)
	ActIDShort   string `json:"act_id_short"`   // first 8 Crockford-base32 chars of ULID, lowercased
	RepoID       string `json:"repo_id"`
	TicketID     string `json:"ticket_id"`
	ApproachName string `json:"approach_name"` // original (unsanitized) for display
	// Computed labels (stored for reconciliation)
	WorktreePath string `json:"worktree_path"`
	BranchName   string `json:"branch_name"`
	ClaudeName   string `json:"claude_name"` // "cmux-board:<ticket>:<act_id_short>"
	CmuxName     string `json:"cmux_name"`   // "<ticket> [<act_id_short>]"
	// External IDs (journaled after each side effect)
	ClaudeShortID   string `json:"claude_short_id,omitempty"`
	ClaudeSessionID string `json:"claude_session_id,omitempty"`
	CmuxWorkspaceID string `json:"cmux_workspace_id,omitempty"`
	AgentPaneRef    string `json:"agent_pane_ref,omitempty"`
	// Journal step tracking (Codex Finding 5)
	Step     string `json:"step"`     // "started"|"worktree_created"|"claude_started"|"cmux_created"
	Complete bool   `json:"complete"` // false until all 3 side effects journaled
	CreatedAt     time.Time  `json:"created_at"`
	LastFocusedAt *time.Time `json:"last_focused_at,omitempty"`
	// Orphan flags
	ClaudeOrphan bool `json:"claude_orphan,omitempty"`
	CmuxOrphan   bool `json:"cmux_orphan,omitempty"`
	// Tombstone for removed tickets
	RemovedAt *time.Time `json:"removed_at,omitempty"` // set when ticket.removed_at is set
}

// Activation step constants.
const (
	StepStarted         = "started"
	StepWorktreeCreated = "worktree_created"
	StepClaudeStarted   = "claude_started"
	StepCmuxCreated     = "cmux_created"
)

// DefaultState returns an empty State with the current schema version.
func DefaultState() State {
	return State{
		SchemaVersion: SchemaVersionCurrent,
		Tickets:       make(map[string]TicketState),
		Activations:   make(map[string][]ActivationEntry),
	}
}
