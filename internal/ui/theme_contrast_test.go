package ui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

type contrastCase struct {
	name string
	fg   ui.Color
	bg   ui.Color
}

// TestThemeTextContrast holds every face that puts text on a color to
// WCAG AA for normal text: 4.5:1. The first group is the opaque pairs.
// The second is the translucent bands the pages draw — the statusTint
// and statusOutline fills behind status words, and the pending bar's
// amber strip — composited the way the renderer composites them: the
// fill's alpha is quantized by the toolkit's own Alpha and blended with
// its own source-over Over, so the ratio is against the exact color on
// screen, not the token.
//
// The bands sit in the page flow on the content wash (Canvas), but the
// composites are asserted across Canvas, Surface and Surface2, so a band
// that lands on a panel or a hovered row still reads.
//
// The Soft fills carry no contrast pair: the only text they carried was
// the mappings table's health pill, which draws on an opaque surface the
// opaque Success and Danger pairs above already cover, and no other face
// puts text on a soft fill. The protocol badges draw the same way — an
// opaque Surface chip, the protocol's hue in the border and the ink —
// so the opaque TCP, UDP, HTTP and HTTPS pairs are their composite on
// the canvas, on a hovered row and on a selected one alike; the protocol
// tokens are not asserted on the translucent protoTint, which only the
// form's reverse-proxy mark still uses, on the modal's plain Surface.
func TestThemeTextContrast(t *testing.T) {
	for _, tokens := range []Tokens{Light(), Dark()} {
		cases := []contrastCase{
			{"primary on canvas", tokens.FG, tokens.Canvas},
			{"secondary on canvas", tokens.Muted, tokens.Canvas},
			{"metadata on canvas", tokens.Meta, tokens.Canvas},
			{"primary on surface", tokens.FG, tokens.Surface},
			{"secondary on surface", tokens.Muted, tokens.Surface},
			{"metadata on surface", tokens.Meta, tokens.Surface},
			{"accent button", tokens.AccentText, tokens.Accent},
			{"success", tokens.Success, tokens.Surface},
			{"warning", tokens.Warning, tokens.Surface},
			{"danger", tokens.Danger, tokens.Surface},
			{"TCP", tokens.TCP, tokens.Surface},
			{"UDP", tokens.UDP, tokens.Surface},
			{"HTTP", tokens.HTTP, tokens.Surface},
			{"HTTPS", tokens.HTTPS, tokens.Surface},
		}

		// The status bands, with the ink each one actually carries: the
		// imported-config bar's word is plain ink; the warning and danger
		// bands say themselves in their own hue; the ended-session bands
		// and the danger zone's caption are secondary ink.
		successTint, _ := statusTint(tokens.Success)
		warningTint, _ := statusTint(tokens.Warning)
		dangerTint, _ := statusTint(tokens.Danger)
		warningOutline, _ := statusOutline(tokens.Warning)
		dangerOutline, _ := statusOutline(tokens.Danger)
		bands := []struct {
			name string
			fill ui.Color
			ink  ui.Color
		}{
			{"success tint", successTint, tokens.FG},
			{"warning tint", warningTint, tokens.Warning},
			{"danger tint", dangerTint, tokens.Danger},
			{"warning outline", warningOutline, tokens.FG2},
			{"danger outline", dangerOutline, tokens.Danger},
			{"danger outline caption", dangerOutline, tokens.FG2},
		}
		surfaces := []struct {
			name string
			col  ui.Color
		}{
			{"canvas", tokens.Canvas},
			{"surface", tokens.Surface},
			{"surface2", tokens.Surface2},
		}
		for _, b := range bands {
			for _, s := range surfaces {
				cases = append(cases, contrastCase{
					b.name + " over " + s.name, b.ink, b.fill.Over(s.col),
				})
			}
		}
		// The pending bar's amber strip sits on the page wash.
		cases = append(cases, contrastCase{
			"amber strip over canvas", tokens.AmberText, tokens.AmberSoft.Over(tokens.Canvas),
		})

		for _, tc := range cases {
			if got := colorContrast(tc.fg, tc.bg); got < 4.5 {
				t.Errorf("dark=%t %s contrast = %.2f:1, want at least 4.5:1", tokens.Dark, tc.name, got)
			}
		}
	}
}

func colorContrast(a, b ui.Color) float64 {
	la, lb := colorLuminance(a), colorLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func colorLuminance(c ui.Color) float64 {
	channels := [3]float64{
		float64(c.R) / 255,
		float64(c.G) / 255,
		float64(c.B) / 255,
	}
	for i, channel := range channels {
		if channel <= 0.04045 {
			channels[i] = channel / 12.92
		} else {
			channels[i] = math.Pow((channel+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2]
}
