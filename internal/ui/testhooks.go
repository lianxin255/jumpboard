package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
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

// firstTabText is the screen and scrollback of the first session tab of
// a kind, "" when there is none. A terminal's screen is drawn by the
// emulator's cells, not text nodes, so a tester cannot read it.
func (v *View) firstTabText(kind termKind) string {
	v.termMu.Lock()
	defer v.termMu.Unlock()
	for _, s := range v.termTabs {
		if s.kind == kind && s.term != nil {
			return s.term.Text()
		}
	}
	return ""
}

// ShellTextForTest is the shell session's screen and scrollback, "" when
// no shell has started.
func (v *View) ShellTextForTest() string { return v.firstTabText(termShell) }

// ProbeTextForTest is the probe session's screen and scrollback, "" when
// no probe is connected.
func (v *View) ProbeTextForTest() string { return v.firstTabText(termProbe) }

// SerialTextForTest is the serial session's screen and scrollback, "" when
// no serial port is open.
func (v *View) SerialTextForTest() string { return v.firstTabText(termSerial) }

// NewTabForTest opens a tab the way 新建会话 would, so a test can drive a
// session without walking the picker.
func (v *View) NewTabForTest(kind termKind, mappings []model.Mapping) {
	v.newTab(kind, mappings)
}

// ActiveTabForTest is the tab the page would draw.
func (v *View) ActiveTabForTest() *tabSession {
	v.termMu.Lock()
	defer v.termMu.Unlock()
	if v.termActive < 0 || v.termActive >= len(v.termTabs) {
		return nil
	}
	return v.termTabs[v.termActive]
}

// TabCountForTest is how many session tabs are open.
func (v *View) TabCountForTest() int {
	v.termMu.Lock()
	defer v.termMu.Unlock()
	return len(v.termTabs)
}

// KindForTest is the tab's kind, for the tests that need to say what it is.
func (s *tabSession) KindForTest() string { return s.kind.label() }

// IDForTest is the tab's id.
func (s *tabSession) IDForTest() int { return s.id }

// NameForTest is the tab's title.
func (s *tabSession) NameForTest() string { return s.name }
