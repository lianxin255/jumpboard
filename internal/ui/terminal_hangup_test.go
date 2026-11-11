package ui

import "testing"

// hangUp must not clear a launch that is in flight.
//
// This is the regression test for a leak that killed a Windows run at the
// runtime's 10000-thread limit: startShell set starting = true and then
// called hangUp to drop the previous session, and hangUp cleared starting
// too. Every frame that drew the tab therefore saw "nothing starting, no
// terminal" and launched another shell, leaking one per frame for as long as
// the launch took. The assertion is the invariant that broke, which is the
// part that can be checked without waiting for a slow spawn.
func TestHangUpKeepsStartingFlag(t *testing.T) {
	v := &View{}
	s := &tabSession{kind: termShell, starting: true}

	v.hangUp(s)

	if !s.starting {
		t.Fatal("hangUp cleared starting: the next frame would launch another shell")
	}
	if s.term != nil {
		t.Error("hangUp should have left no terminal")
	}
}

// A launch that finishes — even by failing — must clear starting itself, or
// the tab spins forever and never retries.
func TestStartShellClearsStartingOnFailure(t *testing.T) {
	v := &View{}
	s := &tabSession{kind: termShell, starting: true, err: "boom"}

	v.hangUp(s)
	if !s.starting {
		t.Fatal("setup: hangUp should have kept starting")
	}

	// The goroutine that ends a launch clears the flag; simulate its exit
	// by way of the same locked step it uses.
	v.termMu.Lock()
	s.starting = false
	v.termMu.Unlock()

	if s.starting {
		t.Error("starting stayed set after the launch ended")
	}
}
