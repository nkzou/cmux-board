package jira

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// TransitionStatus implements the four-step OCC-emulated workflow transition:
//
//  1. GET current status — detect OCC mismatch with expectedFromStatus.
//  2. If mismatch → return ConflictError immediately (no further HTTP calls).
//  3. GET transitions — find the entry whose to.name matches toStatus (case-insensitive).
//     If not found → return ErrInvalidTransition.
//  4. POST transition — expect HTTP 204. On 409 → ErrConflict. On 400 → classify.
func (a *JiraAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	// Step 1: GET current status.
	slog.Debug("jira request", "method", "GET", "path", "/rest/api/3/issue/"+ticketID)

	issue, resp, err := a.v3.Issue.Get(ctx, ticketID, []string{"status"}, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("failed to get current status for %s: %w", ticketID, mapResponseError(resp))
		}
		return fmt.Errorf("failed to get current status for %s: %w", ticketID, err)
	}

	slog.Debug("jira response", "status", resp.Code)

	var currentStatus string
	if issue.Fields != nil && issue.Fields.Status != nil {
		currentStatus = issue.Fields.Status.Name
	}

	// Step 2: OCC check — abort immediately if mismatch.
	if !strings.EqualFold(currentStatus, expectedFromStatus) {
		return tracker.ConflictError{ServerStatus: currentStatus}
	}

	// Step 3: GET available transitions and find matching one.
	slog.Debug("jira request", "method", "GET", "path", "/rest/api/3/issue/"+ticketID+"/transitions")

	transitions, resp, err := a.v3.Issue.Transitions(ctx, ticketID)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("failed to find transition for %s → %s: %w", ticketID, toStatus, mapResponseError(resp))
		}
		return fmt.Errorf("failed to find transition for %s → %s: %w", ticketID, toStatus, err)
	}

	slog.Debug("jira response", "status", resp.Code)

	var transitionID string
	if transitions != nil {
		for _, t := range transitions.Transitions {
			if t.To != nil && strings.EqualFold(t.To.Name, toStatus) {
				transitionID = t.ID
				break
			}
		}
	}

	if transitionID == "" {
		return tracker.ErrInvalidTransition
	}

	// Step 4: POST the transition.
	slog.Debug("jira request", "method", "POST", "path", "/rest/api/3/issue/"+ticketID+"/transitions")

	resp, err = a.v3.Issue.Move(ctx, ticketID, transitionID, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("failed to transition %s to %s: %w", ticketID, toStatus, mapResponseError(resp))
		}
		return fmt.Errorf("failed to transition %s to %s: %w", ticketID, toStatus, err)
	}

	slog.Debug("jira response", "status", resp.Code)
	return nil
}
