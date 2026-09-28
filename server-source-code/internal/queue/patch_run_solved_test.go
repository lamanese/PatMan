package queue

import "testing"

// A run_patch task that fires late for a solved run must be dropped like
// for any other finished run.
func TestSolvedIsTerminalForDispatch(t *testing.T) {
	if !terminalPatchRunStatuses["solved"] {
		t.Fatal("solved must be in terminalPatchRunStatuses")
	}
	if dispatchablePatchRunStatuses["solved"] {
		t.Fatal("solved must never be dispatchable")
	}
}
