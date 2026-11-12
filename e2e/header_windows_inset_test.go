package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"

	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// TestHeaderClearsTheWindowControls checks that the trailing actions in the
// window bar stay clear of the platform's own window controls.
//
// TitleBar reports the band those controls occupy: the traffic lights on the
// left on macOS, minimise/maximise/close on the right on Windows. The header
// only ever used its Height (to push the rail's brand down), so on Windows the
// last action sat underneath the close button. The assertion is geometric: the
// action's right edge must end before the band starts.
func TestHeaderClearsTheWindowControls(t *testing.T) {
	const windowW = 1280
	// Roughly what Windows reserves for three caption buttons at 100% DPI.
	bar := ui.TitleBar{Height: 32, Right: 138}

	a := testApp(t)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, windowW, 760)
	tt.SetTitleBar(bar)
	tt.Frame()

	r, ok := tt.Find("应用配置")
	if !ok {
		t.Fatalf("the apply action should be drawn: %v", tt.Texts())
	}

	limit := float32(windowW) - bar.Right
	if r.X+r.W > limit {
		t.Errorf("the trailing action ends at %v, past the window controls at %v — it would be covered",
			r.X+r.W, limit)
	}
}

// TestSidebarBrandClearsTheTrafficLights is the same check on the other edge.
//
// macOS puts its traffic lights top-left, over the rail, and the rail's brand
// lives in exactly that corner. TitleBar.Left is the width they take; the
// brand has to start past it. On Windows Left is zero and the brand keeps its
// usual inset, which the second half asserts.
func TestSidebarBrandClearsTheTrafficLights(t *testing.T) {
	a := testApp(t)
	v := njui.NewViewForTest(a)

	mac := ui.NewTester(v.Draw, 1280, 760)
	mac.SetTitleBar(ui.TitleBar{Height: 28, Left: 72})
	mac.Frame()
	r, ok := mac.Find("JumpBoard")
	if !ok {
		t.Fatalf("the brand should be drawn: %v", mac.Texts())
	}
	if r.X < 72 {
		t.Errorf("the brand starts at x=%v, under the traffic lights that end at 72", r.X)
	}

	// Windows: nothing on the left, so the brand sits at its normal inset.
	win := ui.NewTester(v.Draw, 1280, 760)
	win.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
	win.Frame()
	r2, ok := win.Find("JumpBoard")
	if !ok {
		t.Fatalf("the brand should be drawn: %v", win.Texts())
	}
	if r2.X >= 72 {
		t.Errorf("the brand moved to x=%v although the left edge is free", r2.X)
	}
}
