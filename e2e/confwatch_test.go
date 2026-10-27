package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestExternalConfEditShowsTheBar checks the watch end to end: an edit to
// the exported configuration that the app did not make raises the bar, and
// dismissing it clears it.
func TestExternalConfEditShowsTheBar(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(streamRule(t, "被监听的站")); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.Paths.ExportDir, "nginx.conf")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("apply should have written the export: %v", err)
	}

	// A foreign edit: rewrite the file with a later mtime.
	if err := os.WriteFile(path, []byte("server { listen 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	// The poll runs every two seconds; nudge it the way the loop does.
	a.PollConfForTest()
	if !a.ConfChanged() {
		t.Fatal("a foreign edit should raise the bar")
	}

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the external-edit bar", func() bool { return tt.HasText("配置文件被外部修改") })

	if err := tt.Click("忽略"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ConfChanged() {
		t.Error("dismissing should clear the change")
	}
}
