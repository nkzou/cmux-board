package jira

import (
	"context"
	"fmt"
	"strings"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// rawKeyWorkflowTransitions is the key under Ticket.Raw where the lazy transition cache is stored.
// Value type: map[string]string (transition name → transition ID).
const rawKeyWorkflowTransitions = "workflow_transitions"

// LoadTransitionCache lazily fetches workflow transitions for a ticket and stashes the
// name→ID map in ticket.Raw[rawKeyWorkflowTransitions].
//
// Cache hit (already populated): returns nil without making an HTTP request.
// Cache miss: calls fetchTransitions and builds the map.
// On error: Raw is NOT modified (no partial write).
//
// This is IN-MEMORY ONLY — do NOT call state.Store.Mutate here.
// The Raw field is ephemeral and intentionally excluded from state.json persistence.
//
// Called by M-004 push handler before invoking TransitionStatus when RequiresWorkflowID is true.
func LoadTransitionCache(ctx context.Context, client *jiraClient, ticket *tracker.Ticket) error {
	// Cache hit check.
	if existing, ok := ticket.Raw[rawKeyWorkflowTransitions]; ok {
		if m, ok := existing.(map[string]string); ok && len(m) > 0 {
			return nil // cache already populated
		}
	}

	entries, err := fetchTransitions(ctx, client, ticket.ID)
	if err != nil {
		return fmt.Errorf("failed to load transition cache for %s: %w", ticket.Key, err)
	}

	cache := make(map[string]string, len(entries))
	for _, e := range entries {
		cache[e.Name] = e.ID
	}
	ticket.Raw[rawKeyWorkflowTransitions] = cache

	return nil
}

// TransitionIDFromRaw reads the cached workflow transition ID for a given target status name.
// The lookup is case-insensitive.
// Returns ("", false) when Raw is nil, the cache key is absent, or no match is found.
// This is a pure in-memory operation — no context needed.
func TransitionIDFromRaw(ticket *tracker.Ticket, toStatusName string) (string, bool) {
	if ticket.Raw == nil {
		return "", false
	}
	raw, ok := ticket.Raw[rawKeyWorkflowTransitions]
	if !ok || raw == nil {
		return "", false
	}
	cache, ok := raw.(map[string]string)
	if !ok {
		return "", false
	}
	for name, id := range cache {
		if strings.EqualFold(name, toStatusName) {
			return id, true
		}
	}
	return "", false
}

// WipeTransitionCache clears the workflow transition cache from a ticket's Raw map.
// Called by the poll handler after each ListTickets refresh to clear stale IDs.
// This operates on an in-memory tracker.Ticket value — NOT on state.json.
func WipeTransitionCache(ticket *tracker.Ticket) {
	if ticket.Raw != nil {
		delete(ticket.Raw, rawKeyWorkflowTransitions)
	}
}
