package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFormSavesInSmallestWindow checks the smallest window the app allows:
// the dialog's scroll area now sizes itself to the window, so 保存 stays
// on screen instead of being pushed out where nothing can reach it.
func TestFormSavesInSmallestWindow(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)

	tt := tester(t, a, 900, 600)
	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "名称", "小窗口站")
	fillField(t, tt, "监听端口", itoa(listen))
	fillField(t, tt, "上游主机", "127.0.0.1")
	fillField(t, tt, "上游端口", itoa(upPort))
	if err := tt.Click("保存"); err != nil {
		t.Fatalf("the save button must be reachable in a 900x600 window: %v", err)
	}
	waitFor(t, tt, "the mapping in the table", func() bool {
		list, _ := a.Store.ListMappings()
		return len(list) == 1 && list[0].Name == "小窗口站"
	})
}

// TestSearchShortcutFilters checks ⌘F: the shortcut focuses the search
// box, so typing filters the table without a click first.
func TestSearchShortcutFilters(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if _, err := a.Store.CreateMapping(streamRule(t, "被搜的站")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.CreateMapping(streamRule(t, "别的站")); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	a.Change()
	waitFor(t, tt, "both rows", func() bool { return tt.HasText("被搜的站") && tt.HasText("别的站") })

	// ⌘F, then type: the caret must already be in the search box. The
	// focus request lands on the next frame, as it does for a user.
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("被搜")
	tt.Frame()
	if !tt.HasText("被搜的站") {
		t.Errorf("the match should stay listed: %v", tt.Texts())
	}
	if tt.HasText("别的站") {
		t.Errorf("the non-match should be filtered out: %v", tt.Texts())
	}
}

// TestLogListScrollsToOlderRows checks the log list keeps its scroll: the
// state used to be rebuilt every frame, so anything past the first page
// could never be read.
func TestLogListScrollsToOlderRows(t *testing.T) {
	a := testApp(t)
	// Enough rows that the earliest ones fall outside the first page.
	for i := 0; i < 80; i++ {
		a.Log.Info("行 %03d", i)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	a.Change()

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The newest rows show; the earliest do not.
	if !tt.HasText("行 079") {
		t.Errorf("the newest row should be listed: %v", tt.Texts())
	}
	if tt.HasText("行 000") {
		t.Fatalf("行 000 should be off the first page for this test to mean anything")
	}
	// Wheel-scroll the list; the scroll must survive the frame.
	for i := 0; i < 40 && !tt.HasText("行 000"); i++ {
		tt.Scroll(600, 400, 0, 240)
		tt.Frame()
	}
	if !tt.HasText("行 000") {
		t.Errorf("scrolling should reach the oldest rows: %v", tt.Texts())
	}
}

// TestAccessLogReadErrorIsShown checks that a log that cannot be read is
// reported as an error, not shown as "还没有访问记录": the log page is the
// place a broken engine gets noticed.
func TestAccessLogReadErrorIsShown(t *testing.T) {
	a := testApp(t)
	// New() applies an empty rule set and keeps access.log open. Windows
	// will not remove an open file, so release the engine handle first.
	// Replacing the log with a directory is then a portable read error:
	// TailFile rejects IsDir (a zero-size directory on Windows would
	// otherwise look like an empty log).
	if err := a.Eng.Stop(); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(a.Paths.LogsDir, "access.log")
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logPath, 0o755); err != nil {
		t.Fatal(err)
	}
	a.SetNginxLogRead(true)

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("访问日志"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the read error", func() bool {
		return tt.HasText("读取访问日志失败")
	})
}
