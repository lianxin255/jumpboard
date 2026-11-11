package ui

import (
	"github.com/egoist/mygo/ui"
)

// The app's buttons, as physical objects.
//
// The toolkit's Button and PrimaryButton give the right behavior — pointer,
// keyboard, focus, disabled — and a flat face. These wrap them with the
// material stack from button_material.go, so a click has a shape: the face is
// lit from above, it sits on the surface, and it goes down when pressed.
//
// Nothing here adds a new control. The behavior stays the toolkit's; only the
// paint changes, which is what keeps the accessibility tree, the keyboard
// handling and the existing tests intact.

// PrimaryAction is the solid accent button: the one main action of a page.
//
// disabled is a parameter rather than a check on the element because the
// caller sets it *after* this returns — `PrimaryAction(...).Disabled(busy)` —
// so IsDisabled() inside would always be false and the disabled face would
// never draw. The element is still marked disabled for input and
// accessibility; this only decides which face to paint.
func PrimaryAction(c *ui.Context, t Tokens, label string, disabled bool) ui.Element {
	b := ui.PrimaryButton(c, label).TextColor(t.AccentText)
	if disabled {
		// A disabled face keeps the shape but loses the light: no gradient,
		// no highlight, and only the contact shadow, so it reads as flat
		// rather than as a button nobody can press.
		return b.Background(t.Surface2).Border(1, t.Border).
			TextColor(t.Meta).
			Shadow(0, 1, 1, 0, t.Shadow.Alpha(0.25))
	}
	m := primaryMaterial(t)
	if b.Pressed() {
		return applyButtonMaterial(b, m, true).Translate(0, PressOffset(BodySize))
	}
	if b.Hovered() {
		// Hover lifts: the gradient widens and the diffuse shadow grows, so
		// the face rises a little toward the pointer.
		m.top = m.top.Mix(ui.Hex("#FFFFFF"), 0.06)
		return applyButtonMaterial(b, m, false).
			Shadow(0, 3, 5, 0, m.diffuse)
	}
	return applyButtonMaterial(b, m, false)
}

// SecondaryAction is the raised plain button: 预检, 取消, 从模板新建 — a real
// affordance one step below the accent. disabled is a parameter for the same
// reason as PrimaryAction's.
func SecondaryAction(c *ui.Context, t Tokens, label string, disabled bool) ui.Element {
	b := ui.Button(c, label)
	if disabled {
		return b.Background(t.Surface3).Border(1, t.Border).
			TextColor(t.Meta).
			Shadow(0, 1, 1, 0, t.Shadow.Alpha(0.20))
	}
	m := secondaryMaterial(t)
	if b.Pressed() {
		return applyButtonMaterial(b, m, true).Translate(0, PressOffset(BodySize))
	}
	if b.Hovered() {
		m.top = m.top.Mix(ui.Hex("#FFFFFF"), 0.04)
		return applyButtonMaterial(b, m, false).
			Shadow(0, 3, 4, 0, m.diffuse)
	}
	return applyButtonMaterial(b, m, false)
}

// QuietAction is the flat button for a row or a toolbar: no gradient, no
// drop shadow, just the fill and the hairline. A page of rows must not turn
// into a page of raised keys — the emphasis belongs to the one action that
// matters, and everything else answers on hover.
func QuietAction(c *ui.Context, t Tokens, label string, disabled bool) ui.Element {
	b := ui.Button(c, label)
	switch {
	case disabled:
		return b.Background(ui.Transparent).TextColor(t.Meta)
	case b.Pressed():
		return b.Background(t.Surface2).Border(1, t.Border)
	case b.Hovered():
		return b.Background(t.Surface2).Border(1, t.Border)
	}
	return b.Background(ui.Transparent).Border(1, ui.Transparent)
}

// GhostAction is QuietAction with no border at all, for the tightest spots:
// a row's icon-sized text action, a card's 收起.
func GhostAction(c *ui.Context, t Tokens, label string, disabled bool) ui.Element {
	b := ui.Button(c, label)
	switch {
	case disabled:
		return b.Background(ui.Transparent).TextColor(t.Meta)
	case b.Pressed(), b.Hovered():
		return b.Background(t.Surface2)
	}
	return b.Background(ui.Transparent)
}
