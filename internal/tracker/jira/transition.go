package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// acliWorkitemViewResult is the JSON shape from 'acli jira workitem view KEY --json'.
// Observed fields: id, key, fields.{assignee, description, issuetype, status, summary}.
type acliWorkitemViewResult struct {
	ID     string              `json:"id"`
	Key    string              `json:"key"`
	Fields acliWorkitemFields  `json:"fields"`
}

type acliWorkitemFields struct {
	Status *acliStatus `json:"status"`
}

// acliTransitionResult is the JSON shape from 'acli jira workitem transition --json'.
type acliTransitionResult struct {
	Results      []acliTransitionResultItem `json:"results"`
	TotalCount   int                        `json:"totalCount"`
	SuccessCount int                        `json:"successCount"`
}

type acliTransitionResultItem struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

// TransitionStatus implements OCC-emulated workflow transition via acli:
//
//  1. GET current status via 'acli jira workitem view KEY --json' → parse fields.status.name
//  2. If !EqualFold(currentStatus, expectedFromStatus) → return ConflictError immediately.
//  3. POST via 'acli jira workitem transition --key KEY --status toStatus --yes --json'
//  4. Parse JSON result; map FAILURE messages to ErrInvalidTransition or errFatal.
func (a *JiraAdapter) TransitionStatus(ctx context.Context, ticketID, expectedFromStatus, toStatus string) error {
	// Step 1: GET current status.
	currentStatus, err := a.fetchCurrentStatus(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("TransitionStatus: failed to get current status for %s: %w", ticketID, err)
	}

	// Step 2: OCC check — abort immediately if mismatch.
	if !strings.EqualFold(currentStatus, expectedFromStatus) {
		return tracker.ConflictError{ServerStatus: currentStatus}
	}

	// Step 3: POST the transition.
	stdout, stderr, exitCode, err := a.runner(ctx,
		"jira", "workitem", "transition",
		"--key", ticketID,
		"--status", toStatus,
		"--yes",
		"--json",
	)
	if err != nil {
		return fmt.Errorf("TransitionStatus: failed to run acli: %w", err)
	}

	// Check stderr for auth before parsing JSON.
	stderrStr := strings.TrimSpace(string(stderr))
	if isAuthStderr(stderrStr) {
		return &errAuth{msg: "TransitionStatus: not authenticated — run 'acli jira auth login --web'"}
	}

	if exitCode == -1 {
		return &errFatal{msg: "TransitionStatus: acli not found or failed to start", stderr: stderrStr}
	}

	// Parse the JSON result.
	stdoutStr := strings.TrimSpace(string(stdout))
	if stdoutStr == "" {
		return &errFatal{msg: fmt.Sprintf("TransitionStatus: empty output from acli for %s", ticketID), stderr: stderrStr}
	}

	var result acliTransitionResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return fmt.Errorf("TransitionStatus: failed to parse acli output: %w", err)
	}

	// Step 4: inspect the result.
	if result.SuccessCount == 0 && len(result.Results) > 0 {
		msg := result.Results[0].Message
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "no allowed transitions") ||
			strings.Contains(lower, "invalid transition") ||
			strings.Contains(lower, "transition not allowed") {
			return tracker.ErrInvalidTransition
		}
		if strings.Contains(lower, "does not exist") {
			return &errFatal{msg: fmt.Sprintf("TransitionStatus: ticket not found: %s", msg)}
		}
		return &errFatal{msg: fmt.Sprintf("TransitionStatus: transition failed: %s", msg)}
	}

	return nil
}

// fetchCurrentStatus retrieves the current status name for a ticket via
// 'acli jira workitem view KEY --json'.
func (a *JiraAdapter) fetchCurrentStatus(ctx context.Context, ticketID string) (string, error) {
	stdout, stderr, exitCode, err := a.runner(ctx,
		"jira", "workitem", "view", ticketID, "--json",
	)
	if err != nil {
		return "", fmt.Errorf("fetchCurrentStatus: failed to run acli: %w", err)
	}

	if ferr := mapACLIError(stdout, stderr, exitCode, "fetchCurrentStatus"); ferr != nil {
		return "", ferr
	}

	var result acliWorkitemViewResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", fmt.Errorf("fetchCurrentStatus: failed to parse acli output: %w", err)
	}

	if result.Fields.Status == nil {
		return "", &errFatal{msg: fmt.Sprintf("fetchCurrentStatus: no status field for ticket %s", ticketID)}
	}

	return result.Fields.Status.Name, nil
}
