package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// issueStatusResponse is the Jira GET /rest/api/3/issue/{id}?fields=status response.
type issueStatusResponse struct {
	Fields struct {
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// transitionsResponse is the Jira GET /rest/api/3/issue/{id}/transitions response.
type transitionsResponse struct {
	Transitions []transitionEntry `json:"transitions"`
}

// transitionEntry represents a single available workflow transition.
type transitionEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

// fetchTransitions returns the raw transitions list for a ticket.
// Used by TransitionStatus (step 3) and by the T-024 cache loader.
func fetchTransitions(ctx context.Context, client *jiraClient, ticketID string) ([]transitionEntry, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", ticketID)
	resp, err := client.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transitions for %s: %w", ticketID, err)
	}

	body, err := classifyResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transitions for %s: %w", ticketID, err)
	}

	var tr transitionsResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("failed to decode transitions for %s: %w", ticketID, err)
	}

	return tr.Transitions, nil
}

// TransitionStatus implements the four-step OCC-emulated workflow transition:
//
//  1. GET current status — detect OCC mismatch with expectedFromStatus.
//  2. If mismatch → return ErrConflict immediately (no further HTTP calls).
//  3. GET transitions — find the entry whose to.name matches toStatus (case-insensitive).
//     If not found → return ErrInvalidTransition.
//  4. POST transition — expect HTTP 204. On 409 → ErrConflict. On 400 → classifyResponse.
//
// This is the most critical adapter method; it must distinguish ErrConflict (OCC race)
// from ErrInvalidTransition (workflow configuration issue).
func (a *JiraAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	// Step 1: GET current status.
	statusPath := fmt.Sprintf("/rest/api/3/issue/%s?fields=status", ticketID)
	statusResp, err := a.client.Do(ctx, http.MethodGet, statusPath, nil)
	if err != nil {
		return fmt.Errorf("failed to get current status for %s: %w", ticketID, err)
	}

	statusBody, err := classifyResponse(statusResp)
	if err != nil {
		return fmt.Errorf("failed to get current status for %s: %w", ticketID, err)
	}

	var issueStatus issueStatusResponse
	if err := json.Unmarshal(statusBody, &issueStatus); err != nil {
		return fmt.Errorf("failed to decode status response for %s: %w", ticketID, err)
	}

	// Step 2: OCC check — if current status differs from expected, abort immediately.
	// CRITICAL: No further HTTP calls on mismatch (avoids wasted requests).
	if !strings.EqualFold(issueStatus.Fields.Status.Name, expectedFromStatus) {
		return tracker.ErrConflict
	}

	// Step 3: GET available transitions and find the one matching toStatus.
	transitions, err := fetchTransitions(ctx, a.client, ticketID)
	if err != nil {
		return fmt.Errorf("failed to find transition for %s → %s: %w", ticketID, toStatus, err)
	}

	var transitionID string
	for _, t := range transitions {
		if strings.EqualFold(t.To.Name, toStatus) {
			transitionID = t.ID
			break
		}
	}

	if transitionID == "" {
		// No matching transition exists — this is a workflow configuration issue, not an OCC race.
		return tracker.ErrInvalidTransition
	}

	// Step 4: POST the transition.
	// Body must be JSON-encoded to prevent injection if transition IDs ever contain special chars.
	postBody, err := json.Marshal(map[string]any{
		"transition": map[string]any{"id": transitionID},
	})
	if err != nil {
		return fmt.Errorf("failed to encode transition request for %s: %w", ticketID, err)
	}

	transitionPath := fmt.Sprintf("/rest/api/3/issue/%s/transitions", ticketID)
	postResp, err := a.client.Do(ctx, http.MethodPost, transitionPath, bytes.NewReader(postBody))
	if err != nil {
		return fmt.Errorf("failed to post transition for %s: %w", ticketID, err)
	}

	_, err = classifyResponse(postResp)
	if err != nil {
		return fmt.Errorf("failed to transition %s to %s: %w", ticketID, toStatus, err)
	}

	return nil
}
