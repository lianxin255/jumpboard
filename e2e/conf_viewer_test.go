package e2e

import (
	"testing"
)

// TestConfViewerOpens: the settings page's 查看当前配置 opens a read-only
// viewer over the configuration the engine runs. The buffer's text is not a
// node a tester reads, so the dialog's presence and its copy action are
// what the e2e asserts; the text itself comes from NginxExport, which the
// appt tests cover.
func TestConfViewerOpens(t *testing.T) {
	a := testApp(t)
	m := streamRule(t, "被查看的站")
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1280, 800)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The viewer's button sits below the fold now that the AI card leads
	// the settings page; scroll to it the way a user would.
	clickScrolled(t, tt, "查看当前配置")
	waitFor(t, tt, "the configuration viewer", func() bool { return tt.HasText("当前配置") })
	for _, want := range []string{"复制全文", "只读", "关闭"} {
		if !tt.HasText(want) {
			t.Errorf("the viewer misses %q: %v", want, tt.Texts())
		}
	}
	// The export that is the viewer's source has the one rule in it.
	conf, n, err := a.NginxExport()
	if err != nil || n != 1 {
		t.Fatalf("export: %d %v", n, err)
	}
	if conf == "" {
		t.Error("a rule should export to a non-empty configuration")
	}
	// Closing returns to the settings page.
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ConfViewOpen() {
		t.Error("closing should clear the viewer state")
	}
}
