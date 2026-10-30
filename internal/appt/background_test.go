package appt

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestRunActionReportsAndSerializes verifies that the action label is
// shared state, and a second task cannot race the first while it runs.
func TestRunActionReportsAndSerializes(t *testing.T) {
	a := newTestApp(t)
	started := make(chan struct{})
	finish := make(chan struct{})
	var calls atomic.Int32
	a.RunAction("check", func() error {
		calls.Add(1)
		close(started)
		<-finish
		return nil
	})
	<-started
	if got := a.BusyAction(); got != "check" {
		t.Fatalf("the view should hear the running action, got %q", got)
	}
	a.RunAction("apply", func() error {
		calls.Add(1)
		return nil
	})
	if got := calls.Load(); got != 1 {
		t.Fatalf("the second action must wait, calls = %d", got)
	}
	close(finish)
	deadline := time.Now().Add(2 * time.Second)
	for a.BusyAction() != "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := a.BusyAction(); got != "" {
		t.Fatalf("the action label should clear, got %q", got)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("only the first action should have run, calls = %d", got)
	}
}

// TestDestructiveConfirmRunsOnce verifies a confirmed stop executes its
// action inside the existing background task, not a rejected second one.
func TestDestructiveConfirmRunsOnce(t *testing.T) {
	a := newTestApp(t)
	var calls atomic.Int32
	a.Ask(&Confirm{Title: "停止引擎", Label: "停止", Destructive: true, Run: func() error {
		calls.Add(1)
		return nil
	}})
	a.ResolveConfirm(true)
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("the confirmed action should execute exactly once, calls = %d", got)
	}
}
