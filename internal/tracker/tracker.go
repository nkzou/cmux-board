package tracker

import (
	"context"
	"errors"
	"time"
)

// ErrConflict is returned by TransitionStatus when the ticket's current remote
// status no longer matches expectedFromStatus (OCC mismatch or HTTP 409).
// UI resolution: snap card back, show toast "status changed remotely — refresh and retry."
var ErrConflict = errors.New("expected status mismatch")

// ErrInvalidTransition is returned by TransitionStatus when no workflow transition
// exists from the current status to the target status (HTTP 400 workflow-forbidden
// or no matching transition in the transitions list).
// UI resolution: snap card back, show toast "this transition is not allowed by the workflow."
var ErrInvalidTransition = errors.New("transition not permitted by workflow")

// Capabilities describes which optional features an IssueTracker adapter supports.
type Capabilities struct {
	HasAssignees                 bool
	HasLabels                    bool
	HasPriority                  bool
	RequiresWorkflowID           bool // tracker needs workflow transition IDs (Jira)
	SupportsBoardSummary         bool // ListBoards returns meaningful data
	TransitionFromStatusRequired bool // adapter honors expectedFromStatus for OCC
}

// UserIdentity holds the authenticated user's identity.
type UserIdentity struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
}

// BoardSummary is a lightweight descriptor returned by ListBoards.
type BoardSummary struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// Column represents a single column in a board layout.
type Column struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name,omitempty"`
	StatusIDs []string `json:"statusIds,omitempty"` // tracker-native status IDs mapping to this column
}

// Board represents a full board with its column layout.
type Board struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
	Columns []Column `json:"columns,omitempty"`
}

// Ticket represents a single issue/ticket with normalized fields.
// Raw is an adapter escape hatch for tracker-specific data (e.g., Jira workflow transition IDs).
type Ticket struct {
	ID         string         `json:"id,omitempty"`
	Key        string         `json:"key,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	Status     string         `json:"status,omitempty"`
	URL        string         `json:"url,omitempty"`
	AssigneeID string         `json:"assigneeId,omitempty"`
	Labels     []string       `json:"labels,omitempty"`
	Priority   string         `json:"priority,omitempty"`
	UpdatedAt  time.Time      `json:"updatedAt,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// IssueTracker is the port (interface) that all tracker adapters must implement.
// The Jira adapter is the only v1 implementation; Linear/GitHub Issues can plug in later.
type IssueTracker interface {
	WhoAmI(ctx context.Context) (UserIdentity, error)
	ListBoards(ctx context.Context) ([]BoardSummary, error)
	GetBoard(ctx context.Context, boardID string) (Board, error)
	ListTickets(ctx context.Context, boardID string, since *time.Time) ([]Ticket, error)
	TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error
	Capabilities() Capabilities
}
