package e2e

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// TestSidebarPaintsTheThemeToken checks the rail itself, not just the
// content wash: a forced theme has to win over the desktop's appearance.
func TestSidebarPaintsTheThemeToken(t *testing.T) {
	a := testApp(t)
	cases := []struct {
		name   string
		choice appt.ThemeChoice
		dark   bool
		side   ui.Color
		canvas ui.Color
	}{
		{"light on a dark desktop", appt.ThemeLight, true, njui.Light().Sidebar, njui.Light().Canvas},
		{"dark on a light desktop", appt.ThemeDark, false, njui.Dark().Sidebar, njui.Dark().Canvas},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a.SetTheme(tc.choice)
			tt := ui.NewTester(njui.NewView(a), 900, 600)
			tt.SetDark(tc.dark)
			tt.Frame()
			img := tt.Image()
			// Mid-rail, clear of the brand mark and the engine card.
			for _, p := range [][2]int{{40, 220}, {40, 340}, {120, 300}} {
				if got := img.RGBAAt(p[0], p[1]); !sameColor(got, tc.side) {
					t.Errorf("sidebar at %v: got %v, want %v", p, got, tc.side)
				}
			}
			// Inside the page padding, above the first card.
			for _, p := range [][2]int{{250, 8}, {860, 8}} {
				if got := img.RGBAAt(p[0], p[1]); !sameColor(got, tc.canvas) {
					t.Errorf("canvas at %v: got %v, want %v", p, got, tc.canvas)
				}
			}
		})
	}
}

// TestSidebarStaysClearOverTheMaterial checks that a window with a material
// still leaves the rail unpainted, so the material shows, while the content
// keeps its own wash.
func TestSidebarStaysClearOverTheMaterial(t *testing.T) {
	a := testApp(t)
	a.SetTheme(appt.ThemeLight)
	tt := ui.NewTester(njui.NewView(a), 900, 600)
	tt.SetVibrancy(true)
	tt.SetDark(true)
	tt.Frame()
	img := tt.Image()
	if got := img.RGBAAt(40, 300); got.A != 0 {
		t.Errorf("sidebar over a material should be clear, got %v", got)
	}
	if got := img.RGBAAt(860, 8); !sameColor(got, njui.Light().Canvas) {
		t.Errorf("content should still paint the canvas, got %v", got)
	}
}

// TestSetThemeReachesTheMaterial checks that a choice is handed to whoever
// paints the window's material, which is what the rail shows through.
func TestSetThemeReachesTheMaterial(t *testing.T) {
	a := testApp(t)
	var got []appt.ThemeChoice
	a.OnTheme = func(c appt.ThemeChoice) { got = append(got, c) }
	for _, c := range []appt.ThemeChoice{appt.ThemeLight, appt.ThemeDark, appt.ThemeSystem} {
		a.SetTheme(c)
	}
	if len(got) != 3 || got[0] != appt.ThemeLight || got[1] != appt.ThemeDark || got[2] != appt.ThemeSystem {
		t.Errorf("material heard %v", got)
	}
}

func sameColor(got color.RGBA, want ui.Color) bool {
	return got.R == want.R && got.G == want.G && got.B == want.B && got.A == want.A
}
