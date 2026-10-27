package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// The hooks below exist for the end-to-end tests, like the app package's
// own test hooks: a file drop cannot be synthesized headlessly, so the
// drop zone's handler is exposed for a test to drive. NewView returns
// the view function rather than the view, so the hook builds its own.

// DropFilesForTest feeds the drop zone what a real drop would hand over.
func DropFilesForTest(a *appt.App, paths []string) {
	(&View{App: a}).importDropped(paths)
}

// NewViewForTest returns the view itself, for the tests that need the
// state NewView hides behind the draw function — the terminal page's
// sessions, whose screens are not text nodes a tester can read.
func NewViewForTest(a *appt.App) *View { return &View{App: a} }

// Draw is the view's frame; NewView returns it as a plain function.
func (v *View) Draw(c *ui.Context) { v.draw(c) }

// ShellTextForTest is the shell session's screen and scrollback, "" when
// no shell has started.
func (v *View) ShellTextForTest() string {
	v.termMu.Lock()
	shell := v.shell
	v.termMu.Unlock()
	if shell == nil {
		return ""
	}
	return shell.Text()
}

// ProbeTextForTest is the probe session's screen and scrollback, "" when
// no probe is connected.
func (v *View) ProbeTextForTest() string {
	v.termMu.Lock()
	probe := v.probe
	v.termMu.Unlock()
	if probe == nil {
		return ""
	}
	return probe.Text()
}
