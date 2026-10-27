package e2e

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// TestSettingsUpdateCardDevBuild checks what a development build says: the
// card explains updates are unavailable instead of offering the buttons.
func TestSettingsUpdateCardDevBuild(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("开发构建不支持自动更新") {
		t.Fatalf("the card should say why there are no buttons: %v", tt.Texts())
	}
	if tt.HasText("检查更新") {
		t.Error("a development build must not offer the check")
	}
}

// TestSettingsUpdateCheckAndInstall walks the whole card with a fake feed:
// the check finds a version, its notes render, and the install ends in the
// restart the app asked for.
func TestSettingsUpdateCheckAndInstall(t *testing.T) {
	a := testApp(t)
	a.AppVersion = "1.2.0"
	var relaunched atomic.Bool
	a.CheckUpdate = func(context.Context) (*appt.UpdateInfo, error) {
		return &appt.UpdateInfo{Version: "1.3.0", Notes: "### 摘要\n修复了导出的问题。"}, nil
	}
	a.InstallUpdate = func(_ context.Context, progress func(float64)) error {
		progress(0.5)
		progress(1)
		return nil
	}
	a.RelaunchApp = func() { relaunched.Store(true) }

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The card sits below the fold, so the test scrolls to it as a user
	// would before clicking.
	reveal(t, tt, "检查更新")
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the found version", func() bool { return tt.HasText("发现新版本 1.3.0") })
	if !tt.HasText("修复了导出的问题") {
		t.Errorf("the release notes should render: %v", tt.Texts())
	}
	if tt.HasText("已是最新版本") {
		t.Error("a find is not up to date")
	}

	// The notes push the button further down, so it is revealed too.
	reveal(t, tt, "安装并重启")
	if err := tt.Click("安装并重启"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the restart", func() bool { return relaunched.Load() })
	if p := a.UpdateProgress(); p != 1 {
		t.Errorf("progress = %v, want the last report", p)
	}
}
