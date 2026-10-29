package e2e

import (
	"fmt"
	"testing"
)

// TestPasteImportConf merges an nginx.conf from the clipboard: the preview
// says what will be added, and confirming adds the mapping.
func TestPasteImportConf(t *testing.T) {
	a := testApp(t)
	// A rule already there, so the toolbar with 粘贴导入 draws at all: the
	// button lives in the row that only appears once rules exist.
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "已有规则")); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	a.ReadClipboard = func() (string, error) {
		return fmt.Sprintf("server {\n  listen %d;\n  proxy_pass http://192.168.1.20:8080;\n}\n", port), nil
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("粘贴导入"); err != nil {
		t.Fatal(err)
	}
	// The button and the dialog carry the same words, so wait for the one
	// thing only the dialog says.
	waitFor(t, tt, "the paste preview", func() bool { return tt.HasText("剪贴板内容是 nginx 配置") })
	// The dialog's confirm button is source-neutral now — the same dialog
	// answers a paste and a dropped file.
	if err := tt.Click("确认导入"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the preview gone", func() bool { return a.PastePreview() == nil })

	// The address cell carries a 复制地址 label, so the address itself is
	// not a label the tester reads; the header's rule count is.
	waitFor(t, tt, "the second rule in the header", func() bool { return tt.HasText("2 条规则") })

	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 2 {
		t.Fatalf("after a conf paste: %d %v", len(list), err)
	}
	// The pasted one is the second, and carries the clipboard's upstream.
	got := list[1]
	if got.UpstreamHost != "192.168.1.20" || got.UpstreamPort != 8080 {
		t.Errorf("imported wrongly: %+v", got)
	}
}

// TestPasteImportBackupReplaces: a clipboard backup is a full replace, and
// the preview says so plainly before it runs.
func TestPasteImportBackupReplaces(t *testing.T) {
	a := testApp(t)
	// A rule already in the store, so the toolbar is shown at all: the
	// 粘贴导入 button sits in the row that only draws when rules exist.
	old := streamRule(t, "会被替换的旧规则")
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), old); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	// The backup is taken while only the old rule exists, so importing it
	// is the "go back to what I had" case.
	backup, n, err := a.ExportBackupJSON()
	if err != nil || n != 1 {
		t.Fatalf("export: %d %v", n, err)
	}
	a.ReadClipboard = func() (string, error) { return string(backup), nil }

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("粘贴导入"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the backup preview", func() bool { return tt.HasText("JumpBoard 备份") })
	if !tt.HasText("替换") {
		t.Errorf("a backup preview should warn about replacement: %v", tt.Texts())
	}
	if err := tt.Click("确认导入"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the preview gone", func() bool { return a.PastePreview() == nil })

	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 {
		t.Fatalf("after a backup paste: %d %v", len(list), err)
	}
	if list[0].Name != old.Name {
		t.Errorf("the backup should restore the rule it holds: %+v", list[0])
	}
}

// TestPasteImportRefusesGarbage: a clipboard of prose is refused, and no
// dialog is left behind.
func TestPasteImportRefusesGarbage(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "占位规则")); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	a.ReadClipboard = func() (string, error) { return "随便复制的一段话", nil }

	tt := tester(t, a, 1200, 760)
	tt.Frame()
	if !tt.HasText("粘贴导入") {
		t.Fatalf("the paste button should be on the page: %v", tt.Texts())
	}
	if err := tt.Click("粘贴导入"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.PastePreview() != nil {
		t.Error("prose should not raise a preview")
	}
}
