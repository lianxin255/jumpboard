package appt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
	"github.com/lianxin255/jumpboard/internal/paths"
)

// TestImportKind pins what a dropped file is taken for: the two formats
// the app can read, case-insensitively, and nothing else.
func TestImportKind(t *testing.T) {
	for _, tc := range []struct{ ext, want string }{
		{".json", "json"},
		{".JSON", "json"},
		{".conf", "conf"},
		{".Conf", "conf"},
		{".txt", ""},
		{".yaml", ""},
		{"", ""},
		{".json.bak", ""},
	} {
		if got := importKind(tc.ext); got != tc.want {
			t.Errorf("importKind(%q) = %q, want %q", tc.ext, got, tc.want)
		}
	}
}

// droppedApp builds an app on a throwaway data directory, the same
// harness the notification tests use, minus the recorder.
func droppedApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("JUMPBOARD_DATA_ROOT", t.TempDir())
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// writeDropFile writes one file a drop can carry and returns its path.
func writeDropFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// dropBackupJSON is a backup of one mapping, the shape Export writes.
const dropBackupJSON = `{"version":2,"exportedAt":1,"mappings":[{"name":"备份规则","protocol":"tcp","enabled":true,"listenPort":17004,"upstreamHost":"10.8.0.4","upstreamPort":443}]}`

// dropOldRule seeds the app with one mapping a replacement would destroy.
func dropOldRule(t *testing.T, a *App) {
	t.Helper()
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "旧规则", Protocol: model.TCP, Enabled: true, ListenPort: 17003,
		UpstreamHost: "10.8.0.3", UpstreamPort: 80,
	}); err != nil {
		t.Fatal(err)
	}
}

// dropOnlyOldRule fails unless the seeded rule is the only one left,
// which is what "the drop changed nothing" means here.
func dropOnlyOldRule(t *testing.T, a *App) {
	t.Helper()
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "旧规则" {
		t.Fatalf("the old rules should be untouched, have %+v", list)
	}
}

// dropStatus fails unless a status is showing, of the kind asked for.
func dropStatus(t *testing.T, a *App, wantErr bool) string {
	t.Helper()
	st, ok := a.Status()
	if !ok || st.Text == "" {
		t.Fatalf("the drop should leave a visible status, have %+v", st)
	}
	if st.IsErr != wantErr {
		t.Fatalf("status %+v, want IsErr = %v", st, wantErr)
	}
	return st.Text
}

// TestImportDroppedFiles walks what a drop on the window does: a .conf
// merges beside the existing rules, an unknown file is refused by name,
// and neither stops the other.
func TestImportDroppedFiles(t *testing.T) {
	a := droppedApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "已有", Protocol: model.TCP, Enabled: true, ListenPort: 17001,
		UpstreamHost: "10.8.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	conf := nginx.Generate([]model.Mapping{
		{Name: "拖入的库", Protocol: model.TCP, Enabled: true, ListenPort: 17002,
			UpstreamHost: "10.8.0.2", UpstreamPort: 6379},
	}, nginx.GenOptions{})
	dir := t.TempDir()
	confPath := filepath.Join(dir, "dropped.conf")
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	txtPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.ImportDroppedFiles([]string{confPath, txtPath}); err != nil {
		t.Fatal(err)
	}

	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("the conf should merge beside the existing rule, have %d", len(list))
	}
	names := map[string]bool{}
	for _, m := range list {
		names[m.Name] = true
	}
	if !names["已有"] || !names["拖入的库"] {
		t.Errorf("merge kept the old and added the new: %v", names)
	}
	// The refusal is a status, not a failure of the whole drop.
	st, ok := a.Status()
	if !ok || !st.IsErr || st.Text == "" {
		t.Errorf("the skipped file should be a visible status: %+v", st)
	}
}

// TestImportDroppedJSONAsksFirst is the import-safety pin: a dropped
// .json must not replace the rules by itself. It raises the same
// preview a paste does, cancel leaves everything as it was, and only a
// confirm performs the replacement — and reloads what the table shows.
func TestImportDroppedJSONAsksFirst(t *testing.T) {
	a := droppedApp(t)
	dropOldRule(t, a)
	path := writeDropFile(t, "backup.json", dropBackupJSON)

	if err := a.ImportDroppedFiles([]string{path}); err != nil {
		t.Fatal(err)
	}

	// Nothing was imported yet: the old rules stand, and what the drop
	// left is a pending preview that says it would replace them.
	dropOnlyOldRule(t, a)
	pv := a.PastePreview()
	if pv == nil {
		t.Fatal("a dropped backup should raise the paste preview")
	}
	if pv.Kind != pasteBackup || !pv.Replace {
		t.Errorf("preview should be a replacing backup: %+v", pv)
	}
	if pv.Count != 1 {
		t.Errorf("preview count = %d, want 1", pv.Count)
	}
	if !pv.Replace || pv.Detail == "" {
		t.Errorf("preview should explain the replacement: %+v", pv)
	}
	dropStatus(t, a, false)

	// Cancel folds the preview away and changes nothing.
	a.CancelPasteImport()
	if a.PastePreview() != nil {
		t.Error("cancel should clear the preview")
	}
	dropOnlyOldRule(t, a)

	// Confirm is what replaces the rules — and what reloads the table.
	if err := a.ImportDroppedFiles([]string{path}); err != nil {
		t.Fatal(err)
	}
	if err := a.ConfirmPasteImport(); err != nil {
		t.Fatal(err)
	}
	if a.PastePreview() != nil {
		t.Error("confirm should clear the preview")
	}
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "备份规则" {
		t.Fatalf("a confirmed backup replaces the rules: %+v", list)
	}
	rows, _, _, _, _, dirty, _, _ := a.Snapshot()
	if len(rows) != 1 || rows[0].Name != "备份规则" {
		t.Errorf("confirm should reload what the table shows: %+v", rows)
	}
	if !dirty {
		t.Error("confirm should mark the configuration dirty")
	}
}

// TestImportDroppedJSONMixedRefused: a backup beside anything else is
// refused as a whole. Importing the conf but not the backup would do
// half of a gesture the user cannot split, so nothing is imported, and
// a preview that was already pending keeps its place.
func TestImportDroppedJSONMixedRefused(t *testing.T) {
	a := droppedApp(t)
	dropOldRule(t, a)
	conf := nginx.Generate([]model.Mapping{
		{Name: "拖入的库", Protocol: model.TCP, Enabled: true, ListenPort: 17005,
			UpstreamHost: "10.8.0.5", UpstreamPort: 6379},
	}, nginx.GenOptions{})
	// A paste is already waiting for its confirmation.
	if err := a.StartPasteFromText(conf); err != nil {
		t.Fatal(err)
	}

	jsonPath := writeDropFile(t, "backup.json", dropBackupJSON)
	confPath := writeDropFile(t, "dropped.conf", conf)
	if err := a.ImportDroppedFiles([]string{jsonPath, confPath}); err != nil {
		t.Fatal(err)
	}

	// Neither file was imported.
	dropOnlyOldRule(t, a)
	// The pending preview is the paste's, not the dropped backup's.
	pv := a.PastePreview()
	if pv == nil || pv.Kind != pasteConf || pv.Count != 1 {
		t.Fatalf("the pending preview should keep the paste's, have %+v", pv)
	}
	dropStatus(t, a, true)
}

// TestImportDroppedMultipleJSONRefused: two backups in one drop would
// replace the rules twice over, so the drop is refused outright.
func TestImportDroppedMultipleJSONRefused(t *testing.T) {
	a := droppedApp(t)
	dropOldRule(t, a)
	first := writeDropFile(t, "first.json", dropBackupJSON)
	second := writeDropFile(t, "second.json", `{"version":2,"exportedAt":1,"mappings":[{"name":"另一份","protocol":"tcp","enabled":true,"listenPort":17006,"upstreamHost":"10.8.0.6","upstreamPort":80}]}`)

	if err := a.ImportDroppedFiles([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	dropOnlyOldRule(t, a)
	if a.PastePreview() != nil {
		t.Error("a refused drop should not raise a preview")
	}
	dropStatus(t, a, true)
}

// TestImportDroppedJSONAllInvalidKeepsRules: a backup that parses and
// carries the version, but whose mappings are all unusable, must not
// delete the rules it previewed a replacement of. The preview says what
// the backup contains and that invalid entries are skipped; the confirm
// refuses the whole replace and leaves everything as it was.
func TestImportDroppedJSONAllInvalidKeepsRules(t *testing.T) {
	a := droppedApp(t)
	dropOldRule(t, a)
	// The table holds the old rule, so "nothing changed" is visible in
	// the rows too, not only in the store.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	// Structurally a backup; nothing in it survives the sanitizer.
	backup := `{"version":2,"exportedAt":1,"mappings":[{"name":"坏规则","protocol":"tcp","enabled":true,"listenPort":17007}]}`
	path := writeDropFile(t, "backup.json", backup)

	if err := a.ImportDroppedFiles([]string{path}); err != nil {
		t.Fatal(err)
	}
	pv := a.PastePreview()
	if pv == nil {
		t.Fatal("the drop should raise the preview")
	}
	// The preview counts what the backup holds and warns that invalid
	// entries are skipped — it does not promise all of them will land.
	if !strings.Contains(pv.Detail, "包含 1 条映射") || !strings.Contains(pv.Detail, "跳过") {
		t.Errorf("preview detail should be truthful about skips: %s", pv.Detail)
	}

	if err := a.ConfirmPasteImport(); err == nil {
		t.Fatal("confirming an all-invalid backup should be refused")
	}
	if a.PastePreview() != nil {
		t.Error("the refused confirm should still clear the preview")
	}
	dropOnlyOldRule(t, a)
	rows, _, _, _, _, dirty, _, _ := a.Snapshot()
	if len(rows) != 1 || rows[0].Name != "旧规则" {
		t.Errorf("the table should still show the old rules: %+v", rows)
	}
	if dirty {
		t.Error("a refused import should not mark the configuration dirty")
	}
}

// TestImportDroppedInvalidJSON: a .json that is not a backup — whether
// it is not JSON at all, or JSON that is not ours — imports nothing and
// leaves no preview behind.
func TestImportDroppedInvalidJSON(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"not json", "显然不是 JSON"},
		{"json but not a backup", `{"hello":"world"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := droppedApp(t)
			dropOldRule(t, a)
			path := writeDropFile(t, "backup.json", tc.content)
			if err := a.ImportDroppedFiles([]string{path}); err != nil {
				t.Fatal(err)
			}
			dropOnlyOldRule(t, a)
			if a.PastePreview() != nil {
				t.Error("an unrecognized backup should not raise a preview")
			}
			dropStatus(t, a, true)
		})
	}
}
