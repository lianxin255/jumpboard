package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/store"
)

// streamRule is a plain TCP rule that applies cleanly without a live
// upstream: an apply opens the port, it does not dial the backend.
func streamRule(t *testing.T, name string) model.Mapping {
	t.Helper()
	return model.Mapping{
		Name: name, Protocol: model.TCP, Enabled: true,
		ListenPort: freePort(t), UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}
}

// TestWhitelistEntryToggle adds an address through the settings card and
// switches it off and on with its own switch.
func TestWhitelistEntryToggle(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "白名单地址", "10.1.2.3")
	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address row", func() bool { return tt.HasText("10.1.2.3") })

	enabled := func() bool {
		list, err := a.Store.ListWhitelist()
		if err != nil || len(list) != 1 {
			return false
		}
		return list[0].Enabled
	}
	if !enabled() {
		t.Fatal("a new address should start enabled")
	}

	// The row's switch is named for the address, so the click lands on it
	// and not on the master switch.
	if err := tt.Click("启用-10.1.2.3"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address disabled", func() bool { return !enabled() })
	if err := tt.Click("启用-10.1.2.3"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address enabled again", enabled)
}

// TestOpenAtLoginSwitch walks the settings switch: a flip reaches the
// store, with no desktop to register a login item with.
func TestOpenAtLoginSwitch(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开机自启动"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the login-item mirror on", func() bool {
		return a.Store.Setting(store.SettingOpenAtLogin, "") == "1"
	})
	if !a.OpenAtLogin() {
		t.Error("the switch should read on")
	}
}

// TestSnapshotViewDialog opens a snapshot read-only and shows the
// configuration it holds.
func TestSnapshotViewDialog(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(streamRule(t, "快照里的站")); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	// The real apply runs through RunBackground, which reloads afterwards;
	// a direct call does not, so the page needs one to see the snapshot.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("历史"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("还没有快照") {
		t.Fatalf("the snapshot is missing: %v", tt.Texts())
	}
	if err := tt.Click("查看"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("配置快照") || !tt.HasText("server {") || !tt.HasText("快照里的站") {
		t.Fatalf("the snapshot content should show: %v", tt.Texts())
	}
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.SnapshotView() != nil {
		t.Error("closing should clear the snapshot view")
	}
}

// TestExportActionsBuildTheFiles checks the app, not the view, owns the
// export: it builds the backup and the nginx config, and it writes the
// bytes to the path the dialog returned.
func TestExportActionsBuildTheFiles(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(streamRule(t, "导出动作的站")); err != nil {
		t.Fatal(err)
	}

	data, n, err := a.ExportBackupJSON()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.Contains(string(data), "导出动作的站") {
		t.Fatalf("backup = %d mappings, %s", n, data)
	}
	backup := filepath.Join(t.TempDir(), "backup.json")
	if err := a.WriteExportFile(backup, data); err != nil {
		t.Fatal(err)
	}
	if got, err := a.ReadImportFile(backup); err != nil || string(got) != string(data) {
		t.Fatalf("reading the backup back: %v %q", err, got)
	}

	conf, rules, err := a.NginxExport()
	if err != nil {
		t.Fatal(err)
	}
	if rules != 1 || !strings.Contains(conf, "server {") {
		t.Fatalf("nginx export = %d rules:\n%s", rules, conf)
	}
	out := filepath.Join(t.TempDir(), "out.conf")
	if err := a.WriteExportFile(out, []byte(conf)); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(out); err != nil || string(got) != conf {
		t.Fatalf("the written config: %v %q", err, got)
	}
}

// TestExportNginxButton walks the settings button: the save dialog answers
// with a path, and the file holds the rules.
func TestExportNginxButton(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(streamRule(t, "导出按钮的站")); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.conf")
	a.SaveDialog = func(mygo.SaveDialogOptions) (string, error) { return out, nil }

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("导出 nginx 配置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the export should exist at %s: %v", out, err)
	}
	if !strings.Contains(string(data), "server {") {
		t.Errorf("the export should hold the rules:\n%s", data)
	}
	if !tt.HasText("已导出 1 条规则") {
		t.Errorf("the outcome should be announced: %v", tt.Texts())
	}
}

// TestImportBackupButton walks the import button: a backup file replaces
// everything the app holds.
func TestImportBackupButton(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(streamRule(t, "要被替换的站")); err != nil {
		t.Fatal(err)
	}
	// The backup to import: what the app exports, with the rule renamed so
	// that landing is visible.
	payload, err := a.Export()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := a.Store.ListMappings()
	list[0].Name = "备份里的站"
	payload.Mappings = list
	file := filepath.Join(t.TempDir(), "backup.json")
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	a.OpenDialog = func(mygo.OpenDialogOptions) ([]string, error) { return []string{file}, nil }

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("导入备份"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the import to land", func() bool {
		got, err := a.Store.ListMappings()
		return err == nil && len(got) == 1 && got[0].Name == "备份里的站"
	})
}
