package ui

import (
	"github.com/egoist/mygo/ui"
)

// The destructive buttons. The toolkit has an accent PrimaryButton and a
// plain Button; the delete family — 删除、清空、回滚、重置、丢弃 — asks for
// the same two steps in the danger color instead, built the way the app
// already built them one button at a time: the toolkit's own button, its
// palette swapped, no new control.

// DangerButton is the solid danger button: the destructive main action of
// the dialog or the page it sits in — 清空所有数据, a confirm's 确认删除.
// It carries the confirmations it always did; only the color says danger.
func DangerButton(c *ui.Context, t Tokens, label string) ui.Element {
	// The toolkit styles a primary button's hover and pressed faces in a
	// styleFn that stands down once the face is not the accent, so the
	// steps are applied here instead: darkened on a light desktop,
	// lightened on a dark one, the way the accent's own steps go.
	_, hover, pressed := dangerSteps(t)
	b := ui.PrimaryButton(c, label).Background(t.Danger)
	if b.Pressed() {
		b.Background(pressed)
	} else if b.Hovered() {
		b.Background(hover)
	}
	return b
}

// DangerQuietButton is the low-emphasis danger button for a destructive
// action inside a row or a toolbar — a list's 删除, the history's 回滚,
// the logs' 清空. Its face stays the plain button's, so a page of rows
// does not light up red; the ink and the border carry the danger, and the
// toolkit's own hover and pressed faces still answer.
func DangerQuietButton(c *ui.Context, t Tokens, label string) ui.Element {
	return ui.Button(c, label).
		TextColor(t.Danger).Border(1, t.Danger.Alpha(0.35))
}

// dangerSteps is the danger color with its hover and pressed neighbors.
func dangerSteps(t Tokens) (base, hover, pressed ui.Color) {
	step := ui.Hex("#000000")
	if t.Dark {
		step = ui.Hex("#ffffff")
	}
	return t.Danger, t.Danger.Mix(step, 0.12), t.Danger.Mix(step, 0.24)
}
