package appt

import (
	"os"
	"path/filepath"
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

// TestImportDroppedJSON walks the backup path of a drop: a .json goes
// through the ordinary backup import, which replaces the rule set.
func TestImportDroppedJSON(t *testing.T) {
	a := droppedApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "旧规则", Protocol: model.TCP, Enabled: true, ListenPort: 17003,
		UpstreamHost: "10.8.0.3", UpstreamPort: 80,
	}); err != nil {
		t.Fatal(err)
	}
	backup := `{"version":2,"exportedAt":1,"mappings":[{"name":"备份规则","protocol":"tcp","enabled":true,"listenPort":17004,"upstreamHost":"10.8.0.4","upstreamPort":443}]}`
	path := filepath.Join(t.TempDir(), "backup.json")
	if err := os.WriteFile(path, []byte(backup), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.ImportDroppedFiles([]string{path}); err != nil {
		t.Fatal(err)
	}
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	if len(list) != 1 || list[0].Name != "备份规则" {
		t.Fatalf("a dropped backup replaces the rules: %+v", list)
	}
}
