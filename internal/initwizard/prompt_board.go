package initwizard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kevin-zou/cmux-board/internal/tracker"
)

// maxBoardPickRetries is the maximum number of invalid board selection attempts.
const maxBoardPickRetries = 3

// PickBoard fetches the user's boards and, if boardIDOverride is empty, presents a
// numbered menu on w/r (writer/reader). If boardIDOverride is non-empty, validates
// the ID against the fetched list without prompting.
// Returns the selected BoardSummary or an error.
func PickBoard(ctx context.Context, w io.Writer, r io.Reader, a tracker.IssueTracker, boardIDOverride string) (tracker.BoardSummary, error) {
	boards, err := a.ListBoards(ctx)
	if err != nil {
		return tracker.BoardSummary{}, fmt.Errorf("failed to list boards: %w", err)
	}
	if len(boards) == 0 {
		return tracker.BoardSummary{}, fmt.Errorf("no boards found for this account; check that the API token has board access")
	}

	// Flag-override mode: validate ID exists, return without prompting.
	if boardIDOverride != "" {
		for _, b := range boards {
			if b.ID == boardIDOverride {
				return b, nil
			}
		}
		return tracker.BoardSummary{}, fmt.Errorf("board ID %q not found in account's board list", boardIDOverride)
	}

	// Interactive mode: print numbered list and prompt.
	for i, b := range boards {
		fmt.Fprintf(w, "  %d. %s (%s)\n", i+1, b.Name, b.ID)
	}
	fmt.Fprintf(w, "Select board [1-%d]: ", len(boards))

	reader := bufio.NewReader(r)
	for attempt := 0; attempt < maxBoardPickRetries; attempt++ {
		if attempt > 0 {
			// Re-print prompt for retries
			fmt.Fprintf(w, "Select board [1-%d]: ", len(boards))
		}
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return tracker.BoardSummary{}, fmt.Errorf("reading board selection: %w", err)
		}
		input := strings.TrimSpace(line)
		n, parseErr := strconv.Atoi(input)
		if parseErr == nil && n >= 1 && n <= len(boards) {
			return boards[n-1], nil
		}
		fmt.Fprintln(w, "Invalid selection. Enter a number from the list.")
	}
	return tracker.BoardSummary{}, fmt.Errorf("no board selected after %d attempts", maxBoardPickRetries)
}
