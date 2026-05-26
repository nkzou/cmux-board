package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// myselfResponse is the Jira /rest/api/3/myself response shape.
type myselfResponse struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

// WhoAmI validates credentials and returns the authenticated user's identity.
// Endpoint: GET /rest/api/3/myself
func (a *JiraAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	resp, err := a.client.Do(ctx, http.MethodGet, "/rest/api/3/myself", nil)
	if err != nil {
		return tracker.UserIdentity{}, fmt.Errorf("failed to call WhoAmI: %w", err)
	}

	body, err := classifyResponse(resp)
	if err != nil {
		return tracker.UserIdentity{}, fmt.Errorf("failed to call WhoAmI: %w", err)
	}

	var r myselfResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return tracker.UserIdentity{}, fmt.Errorf("failed to decode WhoAmI response: %w", err)
	}

	return tracker.UserIdentity{
		ID:          r.AccountID,
		DisplayName: r.DisplayName,
		Email:       r.Email,
	}, nil
}
