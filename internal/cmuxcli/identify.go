package cmuxcli

import (
	"context"
	"encoding/json"
	"fmt"
)

// Identify calls `cmux --json identify` and returns the caller + focused pane context
// plus the socket path.
//
// Exact argv: cmux --json identify
//
// JSON output shape (RESEARCH §7):
//
//	{
//	  "caller":  { "pane_ref":"pane:1", "workspace_ref":"workspace:1", ... },
//	  "focused": { "pane_ref":"pane:1", "workspace_ref":"workspace:1", ... },
//	  "socket_path": "/Users/.../cmux.sock"
//	}
func Identify(ctx context.Context) (IdentifyResult, error) {
	stdout, _, err := runCmux(ctx, "--json", "identify")
	if err != nil {
		return IdentifyResult{}, fmt.Errorf("cmux identify failed: %w", err)
	}
	var result IdentifyResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return IdentifyResult{}, fmt.Errorf("failed to parse cmux identify output: %w", err)
	}
	return result, nil
}
