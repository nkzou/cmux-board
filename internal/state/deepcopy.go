package state

import "encoding/json"

// deepCopyState returns a deep copy of s by marshaling to JSON and back.
// This ensures full isolation of all maps, slices, and pointer fields.
// Nil maps are re-initialized after the round-trip so callers can always
// assign into them without a nil-map panic.
// Panics on marshal/unmarshal failure, which indicates programmer error
// (a State with non-serializable values should never exist).
func deepCopyState(s *State) *State {
	data, err := json.Marshal(s)
	if err != nil {
		panic("state deepcopy marshal: " + err.Error())
	}
	var cp State
	if err := json.Unmarshal(data, &cp); err != nil {
		panic("state deepcopy unmarshal: " + err.Error())
	}
	// Re-initialize maps that may have been omitted as null during marshal (omitempty on empty map).
	if cp.Tickets == nil {
		cp.Tickets = make(map[string]TicketState)
	}
	if cp.Activations == nil {
		cp.Activations = make(map[string][]ActivationEntry)
	}
	return &cp
}
