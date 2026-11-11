package ui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/lianxin255/jumpboard/internal/appt"
)

// A disabled button must paint the flat face, not the raised accent one.
//
// This is a regression test for a real bug: the wrapper used to ask
// `b.IsDisabled()` inside itself, but the caller sets Disabled *after* the
// wrapper returns — `PrimaryAction(...).Disabled(busy)` — so the check was
// always false and the disabled face never drew. The flag is a parameter now.
func TestDisabledButtonPaintsFlatFace(t *testing.T) {
	tk := TokensFor(appt.ThemeLight, false)
	shot := func(disabled bool) *image.RGBA {
		view := func(c *ui.Context) {
			ui.Row(c).Padding(20).Children(func() {
				PrimaryAction(c, tk, "应用配置", disabled)
			})
		}
		tt := ui.NewTester(view, 300, 120)
		tt.Frame()
		return tt.Image()
	}
	on, off := shot(false), shot(true)
	if on.RGBAAt(60, 30) == off.RGBAAt(60, 30) {
		t.Errorf("the disabled face paints the same as the enabled one: %v", on.RGBAAt(60, 30))
	}
}

// PressOffset is the physical travel of a press: the same 1 point at every
// type size, because a chip that moves 2px reads as falling over.
func TestPressOffsetIsConstant(t *testing.T) {
	for _, size := range []float32{11, 13, 15, 22} {
		if got := PressOffset(size); got != 1 {
			t.Errorf("PressOffset(%v) = %v, want 1", size, got)
		}
	}
}
