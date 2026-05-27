package initwizard

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// adapterNameJira is the canonical adapter name for Jira Cloud.
const adapterNameJira = "jira"

// maxAdapterPickRetries is the maximum number of invalid input attempts before error.
const maxAdapterPickRetries = 5

// AdapterChoice is the result of PromptAdapterPick.
type AdapterChoice struct {
	AdapterName string // e.g. "jira"
}

// PromptAdapterPick writes the adapter menu to w and reads a selection from r.
// In v1, only "jira" is available; any valid input selects it.
// Returns the chosen AdapterChoice or an error if r is closed before a valid selection
// or max retries are exceeded.
func PromptAdapterPick(r io.Reader, w io.Writer) (AdapterChoice, error) {
	fmt.Fprintln(w, "Select an adapter:")
	fmt.Fprintln(w, "  1) Jira (Cloud)")
	fmt.Fprintln(w, "  -- (more adapters coming)")
	fmt.Fprintln(w)

	reader := bufio.NewReader(r)
	for attempt := 0; attempt < maxAdapterPickRetries; attempt++ {
		fmt.Fprint(w, "Enter selection [1]: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return AdapterChoice{}, fmt.Errorf("reading adapter selection: %w", err)
		}
		input := strings.TrimSpace(line)
		switch input {
		case "", "1", adapterNameJira:
			return AdapterChoice{AdapterName: adapterNameJira}, nil
		default:
			fmt.Fprintln(w, "Invalid selection. Enter 1 or press Enter for the default.")
		}
	}
	return AdapterChoice{}, fmt.Errorf("no valid adapter selection after %d attempts", maxAdapterPickRetries)
}
