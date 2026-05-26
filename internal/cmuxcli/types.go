package cmuxcli

// PaneContext is the caller/focused pane context returned by `cmux --json identify`.
// JSON shape (RESEARCH §7):
//
//	{
//	  "is_browser_surface": false,
//	  "pane_ref": "pane:1",
//	  "surface_ref": "surface:1",
//	  "surface_type": "terminal",
//	  "tab_ref": "tab:1",
//	  "window_ref": "window:1",
//	  "workspace_ref": "workspace:1"
//	}
type PaneContext struct {
	IsBrowserSurface bool   `json:"is_browser_surface"`
	PaneRef          string `json:"pane_ref"`
	SurfaceRef       string `json:"surface_ref"`
	SurfaceType      string `json:"surface_type"`
	TabRef           string `json:"tab_ref"`
	WindowRef        string `json:"window_ref"`
	WorkspaceRef     string `json:"workspace_ref"`
}

// IdentifyResult is the parsed output of `cmux --json identify`.
// JSON shape (RESEARCH §7):
//
//	{
//	  "caller":  { "pane_ref": "pane:1", "workspace_ref": "workspace:1", ... },
//	  "focused": { "pane_ref": "pane:1", "workspace_ref": "workspace:1", ... },
//	  "socket_path": "/Users/.../cmux.sock"
//	}
type IdentifyResult struct {
	Caller     PaneContext `json:"caller"`
	Focused    PaneContext `json:"focused"`
	SocketPath string      `json:"socket_path"`
}

// Workspace is one entry from `cmux --json list-workspaces` → workspaces[*].
// JSON shape (RESEARCH §7):
//
//	{
//	  "ref": "workspace:1",
//	  "title": "release suffix",
//	  "current_directory": "/Users/kevin.zou/git/openkanban",
//	  "selected": true,
//	  "pinned": false,
//	  "index": 0
//	}
//
// CurrentDirectory is parsed for diagnostic logging only. Foreign-workspace adoption
// by current_directory heuristic is forbidden (E8 / Codex Finding 1).
type Workspace struct {
	Ref              string `json:"ref"`
	Title            string `json:"title"`
	CurrentDirectory string `json:"current_directory"`
	Selected         bool   `json:"selected"`
	Pinned           bool   `json:"pinned"`
	Index            int    `json:"index"`
}

// Pane is one entry from `cmux --json list-panes` → panes[*].
// Inferred snake_case schema (RESEARCH §7 general).
type Pane struct {
	Ref          string `json:"ref"`
	WorkspaceRef string `json:"workspace_ref,omitempty"`
	Index        int    `json:"index"`
	Active       bool   `json:"active"`
}
