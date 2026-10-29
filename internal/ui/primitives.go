package ui

import (
	"github.com/egoist/mygo/ui"
)

// The small reusable faces the pages share: a dialog's chrome, a page
// panel, a status band, a filter chip, an empty-state card. Each helper
// is only the styling the call sites repeated — the width, the gap and
// the children stay at the call site, so a page keeps its own layout and
// its own labels while the faces it draws stay one shape.

// IconButton is an icon-only control: a quiet square that carries one
// glyph and answers a click, a focus and Enter or Space — ButtonBase's
// own keyboard behavior, not a plain box's. The label is the caller's —
// an icon without words has to say itself to assistive technology and to
// the tests — and the face stays a quiet one, with the hover fill the
// plain button's row takes and none of it while disabled. ink is the
// glyph's color, the danger red of a stop or the accent of a start.
func IconButton(c *ui.Context, t Tokens, icon *ui.SVG, ink ui.Color, disabled bool) ui.Element {
	btn := ui.ButtonBase(c).Size(28, 28).Radius(RadiusButton)
	switch {
	case disabled:
		ink = t.Meta
	case btn.Hovered():
		btn.Background(t.Surface2)
		ink = t.FG
	}
	btn.Children(func() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Icon(c, icon).Size(14, 14).TextColor(ink)
		})
	})
	return btn
}

// modalCard is a dialog's chrome: the surface, the border, the radius and
// the drop every modal carries. The width and the gap are the dialog's
// own; the padding, the shadow and the rest are the family's.
func modalCard(c *ui.Context, t Tokens, width, gap float32) ui.Element {
	return ui.Column(c).Width(width).Gap(gap).Padding(24).Radius(RadiusModal).
		Background(t.Surface).Border(1, t.Border).
		Shadow(0, 12, 40, 0, t.Shadow)
}

// panelShell is the page-level shell around a list or a pane: the surface
// panel that fills its page and clips what scrolls inside it.
func panelShell(c *ui.Context, t Tokens) ui.Element {
	return ui.Column(c).Fill().Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border).Clip()
}

// statusTint is the quiet band an inline status draws: the status color's
// own hue as a low fill, with a line of the same hue a step stronger, so
// the band reads by color as well as by its word.
func statusTint(col ui.Color) (fill, line ui.Color) {
	return col.Alpha(0.12), col.Alpha(0.30)
}

// statusOutline is the louder band a section-wide notice draws — a
// session that ended, the danger zone: less fill, more line.
func statusOutline(col ui.Color) (fill, line ui.Color) {
	return col.Alpha(0.08), col.Alpha(0.45)
}

// chipQuiet is an unselected chip's face: the glass fill and the plain
// border, the ink the caller already gave it.
func chipQuiet(e ui.Element, t Tokens) ui.Element {
	return e.Background(t.Chip).Border(1, t.Border)
}

// chipStyle is the filter chips' selected pair: a quiet chip when it is
// off, and when it is on the accent's ink on the accent's own tint, so
// the choice says itself in color as well as in weight.
func chipStyle(e ui.Element, t Tokens, on bool) ui.Element {
	if on {
		return e.Bold().TextColor(t.Accent).
			Background(t.Accent.Alpha(0.12)).Border(1, t.Accent.Alpha(0.45))
	}
	return chipQuiet(e.TextColor(t.FG2), t)
}

// emptyCard is the centered card an empty page shows: surface, border,
// one radius, generous padding. The width is the page's own.
func emptyCard(c *ui.Context, t Tokens, width float32) ui.Element {
	return ui.Box(c).Width(width).Padding(32, 24).Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border)
}

// emptyGlyph is the round accent badge an empty-state card leads with:
// one glyph in the accent's tint, the card's own hello.
func emptyGlyph(c *ui.Context, t Tokens, glyph string) {
	ui.Box(c).Size(36, 36).Radius(RadiusButton).Background(t.Accent.Alpha(0.14)).
		Border(1, t.Accent.Alpha(0.28)).Children(func() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, glyph).FontSize(HeadSize).TextColor(t.Accent)
		})
	})
}

// emptyGlyphIcon is emptyGlyph's mark: the SVG the empty state is about —
// the Relay, a chat bubble — in the same badge, the product's own symbol
// where a text glyph used to stand in for it.
func emptyGlyphIcon(c *ui.Context, t Tokens, icon *ui.SVG) {
	ui.Box(c).Size(36, 36).Radius(RadiusButton).Background(t.Accent.Alpha(0.14)).
		Border(1, t.Accent.Alpha(0.28)).Children(func() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Icon(c, icon).Size(18, 18).TextColor(t.Accent)
		})
	})
}

// draftCardShell is one proposed mapping's card: the sunken surface the
// AI panel and the chat page's preview section both draw a draft on.
func draftCardShell(c *ui.Context, t Tokens) ui.Element {
	return ui.Column(c).Gap(6).Padding(10).Radius(RadiusChip).
		Background(t.Surface3).Border(1, t.Border)
}

// resultCard is the read-only card an AI answer lands on — a diagnosis or
// an interpretation: the surface, the border, the radius, and room to
// scroll inside.
func resultCard(c *ui.Context, t Tokens) ui.Element {
	return ui.Box(c).FillWidth().Padding(10, 12).Radius(RadiusChip).
		Background(t.Surface).Border(1, t.Border)
}

// kbdKey is one keyboard shortcut as it is shown on a card: a sunken
// keycap in the mono face. The label around it is the caller's.
func kbdKey(c *ui.Context, t Tokens, key string) {
	ui.Box(c).Padding(2, 8).Radius(RadiusBadge).Background(t.Surface3).
		Border(1, t.Border).Children(func() {
		ui.Text(c, key).FontSize(CaptionSize).Bold().TextColor(t.FG).Font(FontMono)
	})
}

// protoTint is a protocol badge's pale fill: the protocol's own hue at a
// low alpha, the ink the caller draws on it.
func protoTint(col ui.Color) ui.Color { return col.Alpha(0.12) }
