package e2e

import (
	"encoding/json"
	"errors"
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
	// The AI card now leads the settings page, so the cards these walks
	// touch sit below the fold of the default window; they are reached
	// the way a user reaches them, by scrolling to them.
	fillFieldScrolled(t, tt, "白名单地址", "10.1.2.3")
	clickScrolled(t, tt, "添加")
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

// TestOpenAtLoginSwitchFailureShowsError refuses the desktop registration and
// checks that the switch reports the failure instead of staying silent.
func TestOpenAtLoginSwitchFailureShowsError(t *testing.T) {
	a := testApp(t)
	a.OnOpenAtLogin = func(bool) error { return errors.New("desktop unavailable") }
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开机自启动"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("设置开机自启失败：desktop unavailable") {
		t.Fatalf("the login-item failure should be visible: %v", tt.Texts())
	}
	if a.OpenAtLogin() {
		t.Error("a failed registration should leave the switch off")
	}
}

// TestSnapshotViewDialog opens a snapshot as a diff against the rules as
// they stand, and checks that rolling back from it asks first. The diff
// itself is drawn rather than laid out as text, so what the test can read
// is the summary line; the line counts behind it are checked in the ui
// package.
func TestSnapshotViewDialog(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "快照里的站")); err != nil {
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
	// The dialog names itself and shows how the snapshot differs from the
	// rules now. The snapshot was just taken, so nothing differs.
	if !tt.HasText("配置快照") || !tt.HasText("差异 +0") {
		t.Fatalf("the snapshot dialog should show its diff summary: %v", tt.Texts())
	}
	// Rolling back asks before it acts.
	if err := tt.Click("回滚到此配置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ConfirmState() == nil || !tt.HasText("回滚配置") {
		t.Fatalf("rollback should ask first: %v", tt.Texts())
	}
	a.ResolveConfirm(false)
	tt.Frame()
	if a.SnapshotView() != nil {
		t.Error("confirming closed the snapshot view already")
	}
}

// TestExportActionsBuildTheFiles checks the app, not the view, owns the
// export: it builds the backup and the nginx config, and it writes the
// bytes to the path the dialog returned.
func TestExportActionsBuildTheFiles(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "导出动作的站")); err != nil {
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
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "导出按钮的站")); err != nil {
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
	clickScrolled(t, tt, "导出 nginx 配置")
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
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "要被替换的站")); err != nil {
		t.Fatal(err)
	}
	// The backup to import: what the app exports, with the rule renamed so
	// that landing is visible.
	payload, err := a.Export()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
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
	// The caption size the redesign raised sits the second row of cards
	// lower, so the import button rests at this window's fold: the test
	// scrolls to it, as the user would.
	reveal(t, tt, "导入备份")
	if err := tt.Click("导入备份"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the import to land", func() bool {
		got, err := a.Store.ListMappings(a.ActiveWorkspace())
		return err == nil && len(got) == 1 && got[0].Name == "备份里的站"
	})
}
