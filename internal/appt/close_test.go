package appt

import "testing"

// TestOnCloseRunsCleanups pins the quit hook's contract: everything
// registered runs, in order, once — a second run (a second Close) does
// nothing.
func TestOnCloseRunsCleanups(t *testing.T) {
	a := newTestApp(t)
	var ran []string
	a.OnClose(func() { ran = append(ran, "engine") })
	a.OnClose(func() { ran = append(ran, "terminal") })

	a.runClosers()
	a.runClosers()
	if len(ran) != 2 || ran[0] != "engine" || ran[1] != "terminal" {
		t.Fatalf("ran = %v, want the two cleanups once each", ran)
	}
}
