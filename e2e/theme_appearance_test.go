package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// TestFollowSystemShowsTheDesktopAppearance checks that the "跟随系统"
// choice uses the desktop's appearance rather than a fixed one.
func TestFollowSystemShowsTheDesktopAppearance(t *testing.T) {
	a := testApp(t)
	a.SetTheme(appt.ThemeSystem)

	for _, dark := range []bool{true, false} {
		tt := ui.NewTester(njui.NewView(a), 900, 600)
		tt.SetDark(dark)
		tt.Frame()
		// The frame paints the palette the tokens picked; the accent is
		// the same in both, so the background is what tells them apart.
		img := tt.Image()
		corner := img.RGBAAt(5, 5)
		if dark && corner.R > 40 {
			t.Errorf("a dark desktop should paint a dark background, got %v", corner)
		}
		if !dark && corner.R < 150 {
			t.Errorf("a light desktop should paint a light background, got %v", corner)
		}
	}
}

// TestForcedThemeIgnoresTheDesktop checks the two fixed choices.
func TestForcedThemeIgnoresTheDesktop(t *testing.T) {
	a := testApp(t)
	a.SetTheme(appt.ThemeDark)

	// Even a light desktop shows the dark palette.
	tt := ui.NewTester(njui.NewView(a), 900, 600)
	tt.SetDark(false)
	tt.Frame()
	if c := tt.Image().RGBAAt(5, 5); c.R > 40 {
		t.Errorf("a forced dark theme should stay dark on a light desktop, got %v", c)
	}

	a.SetTheme(appt.ThemeLight)
	tt = ui.NewTester(njui.NewView(a), 900, 600)
	tt.SetDark(true)
	tt.Frame()
	if c := tt.Image().RGBAAt(5, 5); c.R < 150 {
		t.Errorf("a forced light theme should stay light on a dark desktop, got %v", c)
	}
}

// TestForceDarkFromTheDesktop checks that a running app repaints when the
// desktop changes, which the toolkit reports.
func TestForceDarkFromTheDesktop(t *testing.T) {
	a := testApp(t)
	a.SetTheme(appt.ThemeSystem)

	tt := ui.NewTester(njui.NewView(a), 900, 600)
	tt.SetDark(true)
	tt.Frame()
	darkCorner := tt.Image().RGBAAt(5, 5)

	tt.SetDark(false)
	tt.Frame()
	lightCorner := tt.Image().RGBAAt(5, 5)

	if darkCorner.R >= lightCorner.R {
		t.Errorf("the layout should repaint with the new appearance: dark %v light %v", darkCorner, lightCorner)
	}
}
