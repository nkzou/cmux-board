package initwizard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

const maxColumnNameRetries = 5

// ConfigureColumns discovers the statuses on the board and prompts the user to
// map them into columns. Returns the configured []tracker.Column.
//
// Discovery runs `acli jira workitem search --jql "project = KEY" --limit 100 --json`
// via the adapter. The project key is resolved from the board's location string.
//
// NOTE: tracker.Column.StatusIDs stores status NAMES for the acli backend, not
// numeric tracker IDs. The downstream Resolve function compares ticket.Status
// (a name) against these values. Comparison is case-insensitive in Resolve.
func ConfigureColumns(ctx context.Context, w io.Writer, r io.Reader, a tracker.IssueTracker, boardID string) ([]tracker.Column, error) {
	reader := bufio.NewReader(r)

	// Discover statuses from existing tickets on the board.
	statuses, err := discoverStatuses(ctx, a, boardID)
	if err != nil {
		fmt.Fprintf(w, "Warning: could not discover statuses from board (%v). Proceeding with manual entry.\n", err)
		statuses = nil
	}

	if len(statuses) > 0 {
		fmt.Fprintf(w, "Discovered %d statuses on this board: %s\n", len(statuses), strings.Join(statuses, ", "))
		fmt.Fprint(w, "Auto-map (one column per status, in discovered order)? [Y/n]: ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return nil, fmt.Errorf("reading auto-map choice: %w", err)
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "n" {
			// Auto-map: each status becomes its own column.
			cols := make([]tracker.Column, len(statuses))
			for i, s := range statuses {
				cols[i] = tracker.Column{
					ID:        slugifyStatus(s),
					Name:      s,
					StatusIDs: []string{s},
				}
			}
			return cols, nil
		}
		fmt.Fprintln(w, "Entering custom column builder.")
	} else {
		fmt.Fprintln(w, "No statuses discovered. Entering custom column builder.")
		fmt.Fprintln(w, "(Add tickets to your board first, or enter status names manually.)")
	}

	return buildColumnsInteractive(w, reader, statuses)
}

// discoverStatuses runs a JQL search on the board's project and returns the
// distinct status names seen across all returned tickets, sorted alphabetically.
func discoverStatuses(ctx context.Context, a tracker.IssueTracker, boardID string) ([]string, error) {
	// ListTickets uses the board ID to resolve the project key internally.
	tickets, err := a.ListTickets(ctx, boardID, nil)
	if err != nil {
		return nil, fmt.Errorf("listing tickets for status discovery: %w", err)
	}
	if len(tickets) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(tickets))
	for _, t := range tickets {
		if t.Status != "" {
			seen[t.Status] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

// buildColumnsInteractive runs the interactive column-building loop.
// discoveredStatuses is used for validation hints; may be nil/empty.
func buildColumnsInteractive(w io.Writer, reader *bufio.Reader, discoveredStatuses []string) ([]tracker.Column, error) {
	discovered := make(map[string]struct{}, len(discoveredStatuses))
	for _, s := range discoveredStatuses {
		discovered[strings.ToLower(s)] = struct{}{}
	}

	var cols []tracker.Column

	for {
		colNum := len(cols) + 1
		name, err := promptNonEmpty(w, reader, fmt.Sprintf("Column %d name", colNum), maxColumnNameRetries)
		if err != nil {
			return nil, fmt.Errorf("column name: %w", err)
		}

		hint := ""
		if len(discoveredStatuses) > 0 {
			hint = fmt.Sprintf(" (valid: %s)", strings.Join(discoveredStatuses, ", "))
		}
		fmt.Fprintf(w, "Statuses for %q (comma-separated%s): ", name, hint)
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return nil, fmt.Errorf("reading statuses: %w", err)
		}

		var statuses []string
		for _, raw := range strings.Split(line, ",") {
			s := strings.TrimSpace(raw)
			if s == "" {
				continue
			}
			// Warn on unknown statuses but allow (user may know more than the board shows).
			if len(discovered) > 0 {
				if _, ok := discovered[strings.ToLower(s)]; !ok {
					fmt.Fprintf(w, "Warning: %q was not in discovered statuses — accepted anyway.\n", s)
				}
			}
			statuses = append(statuses, s)
		}
		if len(statuses) == 0 {
			fmt.Fprintln(w, "No statuses entered; column requires at least one status. Try again.")
			continue
		}

		cols = append(cols, tracker.Column{
			ID:        slugifyStatus(name),
			Name:      name,
			StatusIDs: statuses,
		})

		fmt.Fprint(w, "Add another column? [y/N]: ")
		more, err := reader.ReadString('\n')
		if err != nil && len(more) == 0 {
			// EOF → treat as N.
			break
		}
		if strings.ToLower(strings.TrimSpace(more)) != "y" {
			break
		}
	}

	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns defined; at least one column is required")
	}
	return cols, nil
}

// promptNonEmpty prompts with the given label and retries up to maxRetries times
// if the user enters an empty string. Returns an error after maxRetries failures.
func promptNonEmpty(w io.Writer, reader *bufio.Reader, label string, maxRetries int) (string, error) {
	for i := 0; i < maxRetries; i++ {
		fmt.Fprintf(w, "%s: ", label)
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", fmt.Errorf("reading %s: %w", label, err)
		}
		s := strings.TrimSpace(line)
		if s != "" {
			return s, nil
		}
		fmt.Fprintln(w, "Value cannot be empty. Try again.")
	}
	return "", fmt.Errorf("%s: no value entered after %d attempts", label, maxRetries)
}

// slugifyStatus converts a status name into a safe column ID.
// "In Progress" → "in-progress"
func slugifyStatus(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	// Collapse consecutive dashes, trim leading/trailing.
	s := b.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
