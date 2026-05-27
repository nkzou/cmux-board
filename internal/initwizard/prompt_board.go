package initwizard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nkzou/cmux-board/internal/tracker"
)

// maxBoardPickRetries is the maximum number of invalid board ID attempts.
const maxBoardPickRetries = 3

// PickBoard resolves a Jira board from a direct ID. If boardIDOverride is non-empty
// (from --board-id flag), it's used without prompting. Otherwise the user is
// prompted to paste the numeric board ID (from the Jira board URL, e.g.
// https://yoursite.atlassian.net/jira/software/projects/PROJ/boards/42 → "42").
//
// The ID is validated by calling GetBoard, which returns an error if the board
// does not exist. No ListBoards call is made — boards are fetched one at a time
// to keep the wizard fast against accounts with many boards.
func PickBoard(ctx context.Context, w io.Writer, r io.Reader, a tracker.IssueTracker, boardIDOverride string) (tracker.BoardSummary, error) {
	if boardIDOverride != "" {
		return resolveBoardByID(ctx, a, boardIDOverride)
	}

	fmt.Fprintln(w, "Enter the Jira board ID. Find it in the board URL:")
	fmt.Fprintln(w, "  https://<site>.atlassian.net/jira/software/projects/<key>/boards/<ID>")
	reader := bufio.NewReader(r)
	for attempt := 0; attempt < maxBoardPickRetries; attempt++ {
		fmt.Fprint(w, "Board ID: ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return tracker.BoardSummary{}, fmt.Errorf("reading board ID: %w", err)
		}
		id := strings.TrimSpace(line)
		if id == "" {
			fmt.Fprintln(w, "Board ID must not be empty.")
			continue
		}
		summary, err := resolveBoardByID(ctx, a, id)
		if err == nil {
			return summary, nil
		}
		fmt.Fprintf(w, "%v\n", err)
	}
	return tracker.BoardSummary{}, fmt.Errorf("no valid board ID after %d attempts", maxBoardPickRetries)
}

// resolveBoardByID calls GetBoard to validate the ID and capture the board name.
// Returns a BoardSummary with ID, Name, and (when available) Type.
func resolveBoardByID(ctx context.Context, a tracker.IssueTracker, id string) (tracker.BoardSummary, error) {
	board, err := a.GetBoard(ctx, id)
	if err != nil {
		return tracker.BoardSummary{}, fmt.Errorf("board %q not found: %w", id, err)
	}
	return tracker.BoardSummary{ID: board.ID, Name: board.Name}, nil
}
