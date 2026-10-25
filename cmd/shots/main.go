package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/services"
	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// 出图脚本：真数据 + 真视图 + Tester，不依赖窗口。
func main() {
	// The app declares Chinese before its first window; the shots do too, so
	// that the toolkit's own strings (Search, Clear…) match what ships.
	mygo.App.SetLanguages("zh-Hans", "en")
	mygo.App.SetLocale("zh-Hans")
	os.Setenv("JUMPBOARD_DATA_ROOT", "/tmp/shots-data")
	os.Setenv("JUMPBOARD_RESOURCES", "../resources")
	os.RemoveAll("/tmp/shots-data")

	p, err := paths.Resolve()
	if err != nil {
		panic(err)
	}
	a, err := appt.New(p)
	if err != nil {
		panic(err)
	}
	defer a.Close()

	files := map[string][]byte{}
	dir := "../internal/services/builtin"
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		files[e.Name()] = data
	}
	builtin, _ := services.LoadBuiltin(files)
	a.Tpl = services.NewTemplates(builtin, p.DataDir+"/templates/custom")

	rows := []model.Mapping{
		{Name: "MySQL 内网", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
			UpstreamHost: "192.168.1.10", UpstreamPort: 3306, HealthCheckEnabled: true, HealthStatus: model.HealthHealthy},
		{Name: "GitLab Web", Protocol: model.HTTP, Enabled: true, ListenPort: 8080,
			ServerNames: "git.example.com", ListenPath: "/", UpstreamHost: "192.168.1.20", UpstreamPort: 80,
			ProxyMode: model.ProxyStandard, HealthCheckEnabled: true, HealthStatus: model.HealthOff},
		{Name: "Redis 缓存", Protocol: model.TCP, Enabled: false, ListenPort: 9100,
			UpstreamHost: "192.168.1.30", UpstreamPort: 6379, HealthCheckEnabled: true, HealthStatus: model.HealthUnhealthy},
		{Name: "MQTT Broker", Protocol: model.UDP, Enabled: true, ListenPort: 1883,
			UpstreamHost: "192.168.1.40", UpstreamPort: 1883, HealthCheckEnabled: true, HealthStatus: model.HealthHealthy},
	}
	for _, m := range rows {
		if _, err := a.Store.CreateMapping(m); err != nil {
			panic(err)
		}
	}
	_ = a.Store.AddSnapshot("# 快照\nworker_processes 1;\n", "reload")
	a.Log.Info("配置应用成功，4 条映射")
	a.Log.Warn("配置校验失败: nginx: [emerg] invalid port in \"0\"")
	a.Log.Error("引擎启动失败 · nginx exited with code 1")

	// The acceptance shots (impl-01..03) want the pending bar, so one rule
	// is saved after the last apply: the configuration reads as dirty,
	// which the mappings page answers with the amber strip.
	if err := a.Apply(); err != nil {
		panic(err)
	}
	a.OpenForm(nil)
	a.SetForm(appt.FormValues{
		Name: "日志收集", Protocol: appt.ProtocolChoices[1],
		ListenPort: "5140", UpstreamHost: "10.0.1.50", UpstreamPort: "514",
		HealthCheckEnabled:  true,
		HealthCheckInterval: "30",
	})
	if err := a.SaveForm(); err != nil {
		panic(err)
	}
	if err := a.Reload(); err != nil {
		panic(err)
	}

	shoot := func(name string, page appt.Page, theme appt.ThemeChoice, dark bool, prep func()) {
		// One app, many shots: overlays from a previous shot must close,
		// or the next frame draws a dialog nobody asked for.
		a.CloseForm()
		a.SetTemplatePicker(false)
		a.SetSnapshotView(nil)
		a.SetPage(page)
		a.SetTheme(theme)
		if prep != nil {
			prep()
		}
		tt := ui.NewTester(njui.NewView(a), 1280, 800)
		tt.SetDark(dark)
		// A Tester takes its language from the app's, which the shots set
		// above; without this the toolkit's own strings stay English.
		tt.SetLocale("zh-Hans")
		tt.Frame()
		// The access-log shot shows the tab, not the pending bar: the dirty
		// state was for impl-01, and by now it has been applied away.
		if name == "impl-03" {
			if err := tt.Click("访问日志"); err != nil {
				panic(err)
			}
			tt.Frame()
		}
		img := tt.Image()
		f, err := os.Create("/tmp/ui-shots/" + name + ".png")
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
		f.Close()
		fmt.Println("wrote", name)
	}

	os.MkdirAll("/tmp/ui-shots", 0o755)
	// impl-01..03 mirror the design exports: rules with the pending bar,
	// the new-mapping dialog, and the access log. The dirty state is for
	// the first two; the third applies it away first.
	shoot("impl-01", appt.PageMappings, appt.ThemeLight, false, nil)
	shoot("impl-02", appt.PageMappings, appt.ThemeLight, false, func() {
		a.OpenForm(nil)
	})
	if err := a.Apply(); err != nil {
		panic(err)
	}
	shoot("impl-03", appt.PageLogs, appt.ThemeLight, false, func() {
		a.SetNginxLogRead(true)
	})
	shoot("mappings-dark", appt.PageMappings, appt.ThemeDark, true, nil)
	shoot("mappings-light", appt.PageMappings, appt.ThemeLight, false, nil)
	shoot("logs-dark", appt.PageLogs, appt.ThemeDark, true, nil)
	shoot("history-light", appt.PageHistory, appt.ThemeLight, false, nil)
	shoot("settings-dark", appt.PageSettings, appt.ThemeDark, true, nil)
	shoot("settings-light", appt.PageSettings, appt.ThemeLight, false, nil)
	shoot("form-dark", appt.PageMappings, appt.ThemeDark, true, func() {
		a.OpenForm(nil)
	})
}
