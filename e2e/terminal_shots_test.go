package e2e

import (
	"net"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// The assertions below are the terminal page's visual ones. A terminal's
// screen is drawn by the emulator's cells, not text nodes, so Tester.Texts
// never sees what a session shows — the tests read the grid two other ways:
// through the test hooks that hand over a session's own screen, and through
// the pixels, where the grid paints the terminal's background over whatever
// the page washes behind it.

// TestTerminalPageEmptyStateRenders walks the page with no session open:
// the card that says why and how, the kind picker, and the create button.
// There is no 全部关闭 without a tab to close.
func TestTerminalPageEmptyStateRenders(t *testing.T) {
	a := testApp(t)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)
	tt.SetDark(true)
	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"本地终端", "端口探测", "串口", "新建会话", "还没有会话"} {
		if !tt.HasText(want) {
			t.Fatalf("the empty terminal page should show %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText("全部关闭") {
		t.Fatalf("no tab means no 全部关闭: %v", tt.Texts())
	}
	if got := v.TabCountForTest(); got != 0 {
		t.Fatalf("want no tabs, got %d", got)
	}
}

// TestTerminalShellGridPaintsItsOwnBackground is the visual proof that a
// session's grid really draws: at points in the grid's lower half — clear
// of the prompt at the top — the pixels are the terminal's own dark
// background, not the page's canvas. The tab row is there too, with the
// way to close and to close everything.
func TestTerminalShellGridPaintsItsOwnBackground(t *testing.T) {
	termLibReady(t)
	a := testApp(t)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)
	tt.SetDark(true)
	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the shell session to start", func() bool {
		return !tt.HasText("终端创建中")
	})
	if tt.HasText("终端不可用") {
		t.Skipf("the emulator library refused: %v", tt.Texts())
	}
	r, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("the terminal grid should be drawn: %v", tt.Texts())
	}
	canvas := njui.Dark().Canvas
	img := tt.Image()
	for _, p := range [][2]int{
		{int(r.X + r.W/2), int(r.Y + r.H - 24)},
		{int(r.X + 20), int(r.Y + r.H - 24)},
		{int(r.X + r.W - 20), int(r.Y + r.H - 40)},
	} {
		if p[0] < 0 || p[1] < 0 || p[0] >= 1200 || p[1] >= 760 {
			t.Fatalf("sample point %v outside the window", p)
		}
		got := img.RGBAAt(p[0], p[1])
		if sameColor(got, canvas) {
			t.Errorf("the grid should paint its own background at %v, got the canvas %v", p, got)
		}
		if got.R > 128 {
			t.Errorf("the grid should be dark at %v, got %v", p, got)
		}
	}
	for _, want := range []string{"全部关闭", "关闭-"} {
		if !tt.HasText(want) {
			t.Errorf("the tab row should offer %q: %v", want, tt.Texts())
		}
	}
	if got := v.TabCountForTest(); got != 1 {
		t.Fatalf("want one tab, got %d", got)
	}
}

// TestTerminalShellGridPaintsInLightThemeToo runs the same grid check with
// the page in its light theme. The terminal follows the appearance itself —
// a light page gets a light grid, a dark one a dark grid — so the point of
// the assertion is not "dark either way" but "the grid's own background,
// not the page's": the two are different pixels.
func TestTerminalShellGridPaintsInLightThemeToo(t *testing.T) {
	termLibReady(t)
	a := testApp(t)
	a.SetTheme(appt.ThemeLight)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)
	tt.SetDark(false)
	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the shell session to start", func() bool {
		return !tt.HasText("终端创建中")
	})
	if tt.HasText("终端不可用") {
		t.Skipf("the emulator library refused: %v", tt.Texts())
	}
	r, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("the terminal grid should be drawn: %v", tt.Texts())
	}
	img := tt.Image()
	canvas := njui.Light().Canvas
	for _, p := range [][2]int{
		{int(r.X + r.W/2), int(r.Y + r.H - 24)},
		{int(r.X + 20), int(r.Y + r.H - 24)},
	} {
		if got := img.RGBAAt(p[0], p[1]); sameColor(got, canvas) {
			t.Errorf("the grid should paint its own background at %v, got the canvas %v", p, got)
		}
	}
}

// TestTerminalProbeAndSerialPanesRender drives the two panes that are not a
// shell: the probe picker offers the forwarded rule and its port, the serial
// pane offers a device and a rate — or says honestly that there are none.
// Both can be open at once, two tabs, each with its own pick.
func TestTerminalProbeAndSerialPanesRender(t *testing.T) {
	termLibReady(t)
	a := testApp(t)
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	m := streamRule(t, "探测目标")
	m.UpstreamHost, m.UpstreamPort = "127.0.0.1", upstream.Addr().(*net.TCPAddr).Port
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)
	tt.SetDark(true)
	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("端口探测"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the probe pane", func() bool {
		return tt.HasText("目标端口") || tt.HasText("没有可探测的端口")
	})
	if !tt.HasText("目标端口") {
		t.Fatalf("the probe pane should offer its picker: %v", tt.Texts())
	}
	if !tt.HasText("探测目标 — 127.0.0.1:" + strconv.Itoa(m.ListenPort)) {
		t.Fatalf("the probe picker should offer the rule: %v", tt.Texts())
	}
	if !tt.HasText("连接") {
		t.Fatalf("the probe pane should offer 连接: %v", tt.Texts())
	}

	// A serial tab beside it: the device picker and the rate, or the note
	// that the machine has none.
	if err := tt.Click("串口"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the serial pane", func() bool {
		return tt.HasText("波特率") || tt.HasText("没有可用的串口")
	})
	if !tt.HasText("波特率") && !tt.HasText("没有可用的串口") {
		t.Fatalf("the serial pane should answer: %v", tt.Texts())
	}
	if got := v.TabCountForTest(); got != 2 {
		t.Fatalf("want two tabs, got %d", got)
	}
}
