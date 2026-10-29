package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/appt"
	njui "github.com/lianxin255/jumpboard/internal/ui"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
)

// dropAndAwait runs the drop the way the drop zone does — through the
// view, onto the background goroutine — and waits for the store to hold
// n mappings, bounded, because the import lands after the call returns.
func dropAndAwait(t *testing.T, a *appt.App, n int, paths ...string) {
	t.Helper()
	njui.DropFilesForTest(a, paths)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) == n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	t.Fatalf("store holds %d mappings, want %d", len(list), n)
}

// TestDropConfMergesMappings walks the .conf drop end to end: a file
// dropped on the page becomes new mappings beside the existing ones.
func TestDropConfMergesMappings(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "已有", Protocol: model.TCP, Enabled: true, ListenPort: 16001,
		UpstreamHost: "10.9.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	conf := nginx.Generate([]model.Mapping{
		{Name: "拖入的库", Protocol: model.TCP, Enabled: true, ListenPort: 16002,
			UpstreamHost: "10.9.0.2", UpstreamPort: 6379},
	}, nginx.GenOptions{})
	path := filepath.Join(t.TempDir(), "dropped.conf")
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}

	dropAndAwait(t, a, 2, path)

	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	names := map[string]bool{}
	for _, m := range list {
		names[m.Name] = true
	}
	if !names["已有"] || !names["拖入的库"] {
		t.Errorf("merge kept the old and added the new: %v", names)
	}
}

// TestDropRejectsUnknownExtension keeps a wrong file a status line, not
// a crash: nothing lands in the store, and the refusal is visible.
func TestDropRejectsUnknownExtension(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	njui.DropFilesForTest(a, []string{path})

	// The refusal lands as a toast once the background pass finishes.
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the refusal", func() bool { return tt.HasText("已跳过不支持的文件") })
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Errorf("nothing should have been imported: %d", len(list))
	}
}

// TestDropJSONAsksBeforeReplacing walks the .json drop end to end: the
// drop raises the same confirmation dialog a paste does, the old rules
// stand until the dialog is confirmed, and only the confirmation
// replaces them. A drop is a gesture — the replacement is a decision.
func TestDropJSONAsksBeforeReplacing(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "会被替换的旧规则", Protocol: model.TCP, Enabled: true, ListenPort: 16003,
		UpstreamHost: "10.9.0.3", UpstreamPort: 80,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	backup := `{"version":2,"exportedAt":1,"mappings":[{"name":"备份规则","protocol":"tcp","enabled":true,"listenPort":16004,"upstreamHost":"10.9.0.4","upstreamPort":443}]}`
	path := filepath.Join(t.TempDir(), "backup.json")
	if err := os.WriteFile(path, []byte(backup), 0o644); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	njui.DropFilesForTest(a, []string{path})

	// The dialog a paste raises is the one the drop raises now; it is
	// the only place that says what the backup is.
	waitFor(t, tt, "the backup preview", func() bool { return tt.HasText("JumpBoard 备份") })
	if !tt.HasText("替换") {
		t.Errorf("a dropped backup should warn about replacement: %v", tt.Texts())
	}
	// The dialog is up and the old rule is still the whole rule set.
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 1 || list[0].Name != "会被替换的旧规则" {
		t.Fatalf("a dropped backup must not replace before confirmation: %+v", list)
	}

	// The paste dialog's source-neutral confirm — the drop raised the
	// same dialog a paste does.
	if err := tt.Click("确认导入"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the preview gone", func() bool { return a.PastePreview() == nil })

	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 || list[0].Name != "备份规则" {
		t.Fatalf("a confirmed drop replaces the rules: %d %v", len(list), err)
	}
}
