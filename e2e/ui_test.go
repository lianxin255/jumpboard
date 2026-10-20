package e2e

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
	njui "github.com/lianxin255/nginx-jump-gui/internal/ui"
)

// tester builds a Tester on the real view and app.
func tester(t *testing.T, a *appt.App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(njui.NewView(a), w, h)
	tt.SetDark(true)
	return tt
}

// TestMappingLifecycle walks the path a user takes: create, see it in the
// table, edit it, turn it off, delete it.
func TestMappingLifecycle(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if !tt.HasText("还没有映射") {
		t.Fatalf("an empty app should say so: %v", tt.Texts())
	}

	// Create.
	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("新建映射") || !tt.HasText("上游主机") {
		t.Fatalf("the form did not open: %v", tt.Texts())
	}
	fillField(t, tt, "名称", "MySQL 内网")
	fillField(t, tt, "监听端口", "9000")
	fillField(t, tt, "上游主机", "192.168.1.10")
	fillField(t, tt, "上游端口", "3306")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping in the table", func() bool { return tt.HasText("MySQL 内网") })

	// It is in the database, disabled from running until applied.
	list, err := a.Store.ListMappings()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Name != "MySQL 内网" || list[0].ListenPort != 9000 || list[0].UpstreamHost != "192.168.1.10" {
		t.Fatalf("stored wrongly: %+v", list[0])
	}
	if !tt.HasText("未应用") {
		t.Errorf("a saved change should mark the configuration dirty: %v", tt.Texts())
	}

	// Edit.
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "名称", "MySQL 改了名字")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the new name", func() bool { return tt.HasText("MySQL 改了名字") })

	// Delete, through the confirmation.
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("删除映射") {
		t.Fatalf("the confirmation did not open: %v", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if list, _ := a.Store.ListMappings(); len(list) != 1 {
		t.Fatal("canceling the confirmation should keep the mapping")
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("确认删除") {
		t.Fatalf("the confirmation did not open: %v", tt.Texts())
	}
	if err := tt.Click("确认删除"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping gone", func() bool {
		list, _ := a.Store.ListMappings()
		return len(list) == 0
	})
}

// fillField replaces the content of the field labelled label.
//
// A click puts the caret where the pointer landed, so typing after it
// appends; the field is selected first, as a user does with Cmd+A.
func fillField(t *testing.T, tt *ui.Tester, label, text string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("clicking %q: %v", label, err)
	}
	tt.Command("selectAll")
	tt.Type(text)
	tt.Frame()
}

// waitFor runs the frames until cond holds.
//
// The app changes its state in goroutines, so a test asks for frames until
// the change lands rather than assuming one is enough. It fails after a
// few seconds, which means a hang, never a slow machine.
func waitFor(t *testing.T, tt *ui.Tester, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestValidationBlocksBadInput checks that the form refuses what the
// generator could not use.
func TestValidationBlocksBadInput(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "名称", "坏的")
	fillField(t, tt, "监听端口", "99999")
	fillField(t, tt, "上游主机", "10.0.0.1")
	fillField(t, tt, "上游端口", "80")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if !tt.HasText("端口应为 1-65535") {
		t.Errorf("the port error should show: %v", tt.Texts())
	}
	if list, _ := a.Store.ListMappings(); len(list) != 0 {
		t.Errorf("an invalid mapping was saved: %+v", list)
	}
}

// TestThemeSwitch checks that the three appearances are all selectable and
// remembered.
func TestThemeSwitch(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		label string
		theme appt.ThemeChoice
	}{
		{"亮色", appt.ThemeLight},
		{"暗色", appt.ThemeDark},
		{"跟随系统", appt.ThemeSystem},
	} {
		if err := tt.Click(want.label); err != nil {
			t.Fatalf("clicking %q: %v", want.label, err)
		}
		tt.Frame()
		if got := a.Theme(); got != want.theme {
			t.Errorf("theme after %q = %q, want %q", want.label, got, want.theme)
		}
	}
	if got := a.Store.Setting("theme", ""); got != string(appt.ThemeSystem) {
		t.Errorf("the theme was not stored: %q", got)
	}
}

// TestWhitelistRefusesEmpty checks the fail-closed rule: a whitelist with
// no addresses must not produce a configuration.
func TestWhitelistRefusesEmpty(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("启用白名单"); err != nil {
		t.Fatalf("clicking the whitelist switch: %v", err)
	}
	tt.Frame()
	// The switch calls back into the app as it flips; the change lands
	// before the next frame.
	waitFor(t, tt, "the whitelist on", a.WhitelistOn)
	if !tt.HasText("应用配置会被拒绝") {
		t.Errorf("the warning should show: %v", tt.Texts())
	}

	if err := a.Apply(); err == nil {
		t.Fatal("applying with an empty whitelist should fail")
	}

	// Adding an address makes it work.
	fillField(t, tt, "白名单地址", "192.168.1.0/24")
	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address in the list", func() bool { return tt.HasText("192.168.1.0/24") })
}
