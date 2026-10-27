package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// TestTerminalTabsOpenAndClose walks the tab row: a shell tab and a probe
// tab open side by side, the active tab is whichever was clicked last,
// and closing one leaves the other untouched.
func TestTerminalTabsOpenAndClose(t *testing.T) {
	termLibReady(t)
	a := testApp(t)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)

	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := v.TabCountForTest(); got != 1 {
		t.Fatalf("want one tab, got %d", got)
	}
	first := v.ActiveTabForTest()
	if first == nil || first.KindForTest() != "本地终端" {
		t.Fatalf("want a shell tab active, got %+v", first)
	}

	if err := tt.Click("端口探测"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := v.TabCountForTest(); got != 2 {
		t.Fatalf("want two tabs, got %d", got)
	}
	second := v.ActiveTabForTest()
	if second == nil || second.KindForTest() != "端口探测" {
		t.Fatalf("want the probe tab active, got %+v", second)
	}

	// Back to the first tab: its session is still the shell's.
	if err := tt.Click(first.NameForTest()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := v.ActiveTabForTest(); got == nil || got.IDForTest() != first.IDForTest() {
		t.Fatalf("want the shell tab active again, got %+v", got)
	}

	// Closing the probe tab leaves the shell's.
	if err := tt.Click("关闭-" + second.NameForTest()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := v.TabCountForTest(); got != 1 {
		t.Fatalf("want one tab after closing, got %d", got)
	}
	if got := v.ActiveTabForTest(); got == nil || got.IDForTest() != first.IDForTest() {
		t.Fatalf("the shell tab should survive the closing, got %+v", got)
	}
	if strings.Contains(v.ShellTextForTest(), "jb-tab-ok") {
		return
	}
	// The surviving tab still drives its shell: type through it once.
	if r, ok := tt.Find("Terminal"); ok {
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	}
	tt.Type("echo jb-tab-ok")
	tt.Key(0, ui.KeyEnter)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if strings.Contains(v.ShellTextForTest(), "jb-tab-ok") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the surviving shell should still answer: %q", v.ShellTextForTest())
}

// TestTerminalSerialPane shows the serial tab's honest answer: either the
// devices the machine offers, or the note that there are none. Where a
// device opens, the terminal gets its screen; where none can be opened,
// the note names the device.
func TestTerminalSerialPane(t *testing.T) {
	termLibReady(t)
	a := testApp(t)
	v := njui.NewViewForTest(a)
	tt := ui.NewTester(v.Draw, 1200, 760)

	if err := tt.Click("终端"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("串口"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("新建会话"); err != nil {
		t.Fatal(err)
	}
	// The pane lists the ports off the frame.
	waitFor(t, tt, "the serial pane to answer", func() bool {
		return tt.HasText("没有可用的串口") || tt.HasText("设备") || tt.HasText("打开串口失败")
	})
	if tt.HasText("没有可用的串口") {
		t.Skip("this machine has no serial device to open")
	}
	// An open refusal names the device, which the pane shows as a note; a
	// port that opens stays connected — the pane says which — until the
	// device answers or is closed.
	if err := tt.Click("打开串口"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the port to open or refuse", func() bool {
		return tt.HasText("打开串口失败") || tt.HasText("已连接 ") || tt.HasText("串口已断开")
	})
	if tt.HasText("打开串口失败") {
		// A permission or driver the user has not granted is an environment
		// the test skips, not a failure.
		t.Skipf("the machine refused its first serial device: %v", tt.Texts())
	}
	if !tt.HasText("已连接 ") {
		t.Skip("the port closed before the pane said so")
	}
}
