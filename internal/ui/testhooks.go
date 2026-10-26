package ui

import (
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
