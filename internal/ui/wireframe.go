package ui

import (
	"github.com/egoist/mygo/ui"
)

// Wireframe empty states.
//
// An empty page is the one place an interface has nothing to say, so what it
// draws there is pure voice. A large abstract illustration says "a designer
// was here"; a generic icon says nothing at all. What works is a miniature of
// the real thing, drawn in outline: the page the user is about to fill in,
// at 1:4 scale, with the one control that fills it picked out in the accent.
//
// The eye reads a wireframe as "this page, but empty" rather than as
// decoration, which is exactly the message. It also stays honest: the shape
// changes when the page's shape changes, because both are drawn from the same
// description.
//
// Everything here is drawn from boxes and hairlines, so it inherits the
// tokens and needs no assets, no dark-mode variant and no scaling.

// wireframe draws a miniature page: a body with a header line, a few rows,
// and one accented control where the real page's main action sits.
//
// width and height are the miniature's own size in points; the caller picks
// them, but 160×120 is the proportion these are tuned for.
func wireframe(c *ui.Context, t Tokens, width, height float32, rows int) ui.Element {
	// The scale factor: every dimension inside is expressed against the
	// miniature's width so the drawing holds together at any size.
	u := width / 160

	return ui.Box(c).Width(width).Height(height).
		Radius(RadiusPanel*u).Background(t.Surface3).
		Border(1, t.Hair).Clip().Children(func() {
		ui.Column(c).Fill().Padding(10 * u).Gap(6 * u).Children(func() {
			// The header: a mark and a title rule, the accent control on
			// the right where the page's own action lives.
			ui.Row(c).Gap(5 * u).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(9*u, 9*u).Radius(2*u).
					Background(t.Accent.Alpha(0.18)).
					Border(1, t.Accent.Alpha(0.30))
				ui.Box(c).Width(34 * u).Height(4 * u).Radius(2 * u).
					Background(t.Muted.Alpha(0.28))
				ui.Spacer(c)
				// The one accented element: what the user is meant to do.
				ui.Box(c).Width(26 * u).Height(11 * u).Radius(3 * u).
					Background(t.Accent.Alpha(0.85))
			})
			ui.Box(c).FillWidth().Height(1).Background(t.Hair)
			// The rows: each a label rule and a value rule, so the
			// miniature reads as a table rather than as a list of bars.
			for i := 0; i < rows; i++ {
				// The last row fades out: it says "and more of these",
				// which is what an empty state is promising.
				alpha := float32(0.26)
				if i == rows-1 {
					alpha = 0.12
				}
				ui.Row(c).Gap(6 * u).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(30 * u).Height(3.5 * u).Radius(2 * u).
						Background(t.Muted.Alpha(alpha))
					ui.Spacer(c)
					ui.Box(c).Width(20 * u).Height(3.5 * u).Radius(2 * u).
						Background(t.Muted.Alpha(alpha * 0.7))
					ui.Box(c).Size(7*u, 7*u).Radius(7 * u).
						Background(t.Muted.Alpha(alpha * 0.5))
				})
			}
		})
	})
}

// emptyStateCard is the full empty-state card: the miniature, a title, one
// line of explanation, and the page's own actions.
//
// The card is the caller's width; the content is centered and the gap is
// generous, because an empty page has room and using it is the point.
func emptyStateCard(c *ui.Context, t Tokens, width float32, title, hint string, rows int, actions func()) {
	emptyCard(c, t, width).Children(func() {
		ui.Column(c).Gap(14).AlignItems(ui.Center).Children(func() {
			wireframe(c, t, 160, 120, rows)
			ui.Column(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(HeadSize).Bold().TextColor(t.FG)
				ui.Text(c, hint).FontSize(BodySize).TextColor(t.Muted).Wrap().Pretty()
			})
			if actions != nil {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(actions)
			}
		})
	})
}
