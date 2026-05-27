package jira

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// WhoAmI validates credentials and returns the authenticated user's identity.
// Uses go-atlassian MySelf.Details (GET /rest/api/3/myself).
func (a *JiraAdapter) WhoAmI(ctx context.Context) (tracker.UserIdentity, error) {
	slog.Debug("jira request", "method", "GET", "path", "/rest/api/3/myself")

	user, resp, err := a.v3.MySelf.Details(ctx, nil)
	if err != nil {
		if resp != nil {
			return tracker.UserIdentity{}, fmt.Errorf("failed to call WhoAmI: %w", mapResponseError(resp))
		}
		return tracker.UserIdentity{}, fmt.Errorf("failed to call WhoAmI: %w", err)
	}

	slog.Debug("jira response", "status", resp.Code)

	return tracker.UserIdentity{
		ID:          user.AccountID,
		DisplayName: user.DisplayName,
		Email:       user.EmailAddress,
	}, nil
}
