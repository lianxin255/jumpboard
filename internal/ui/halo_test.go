package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lianxin255/jumpboard/internal/appt"
)

// The sweep must travel, and it must stop.
//
// Both halves were broken at first. A box built fresh each frame has no
// identity, so AnimateTo created a channel every pass and returned its
// target at once — the sweep jumped straight to the end and was never seen.
// Binding the box to a Handle fixed the identity, but AnimateTo's first call
// on a new channel also returns the target, so the value had to be Jumped to
// 0 once before the animation could travel from there. And a Jump on every
// frame holds it at 0 forever, which is the other way to draw nothing.
//
// The second assertion matters as much as the first: an animation that never
// ends keeps the window from going idle, which burns battery and stalls
// anything waiting for a settled frame.
func TestHaloSweepTravelsAndStops(t *testing.T) {
	var s haloSweep
	var painted []float32
	view := func(c *ui.Context) {
		s.draw(c, TokensFor(appt.ThemeLight, false), false, 800, 600)
		painted = append(painted, s.progress)
	}
	tt := ui.NewTester(view, 900, 700)
	s.progress, s.live = 0, true

	for i := 0; i < 14; i++ {
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
	}

	distinct := map[float32]bool{}
	for _, v := range painted {
		distinct[v] = true
	}
	if len(distinct) < 3 {
		t.Errorf("the sweep painted %d distinct values, so it did not travel: %v", len(distinct), painted)
	}
	if s.live {
		t.Error("the sweep never terminated; the window would never go idle")
	}
}

// A sweep that has already finished must not restart on a rebuild.
func TestHaloSweepStaysFinished(t *testing.T) {
	var s haloSweep
	view := func(c *ui.Context) {
		s.draw(c, TokensFor(appt.ThemeLight, false), false, 800, 600)
	}
	tt := ui.NewTester(view, 900, 700)
	s.progress, s.live = 0, true
	for i := 0; i < 14; i++ {
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
	}
	if s.live {
		t.Fatal("setup: the sweep should have finished")
	}
	tt.Frame()
	if s.live {
		t.Error("a rebuild restarted a finished sweep")
	}
}

// The first frame of a window is not an arrival: nothing was replaced, so
// nothing should sweep.
func TestHaloDoesNotSweepOnFirstFrame(t *testing.T) {
	var h haloState
	if h.haloStart(appt.PageMappings) {
		t.Error("the first page drew a sweep")
	}
	if h.haloStart(appt.PageMappings) {
		t.Error("rebuilding the same page drew a sweep")
	}
	if !h.haloStart(appt.PageLogs) {
		t.Error("changing the page did not draw a sweep")
	}
}
