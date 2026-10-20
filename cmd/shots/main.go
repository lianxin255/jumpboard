package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
	"github.com/lianxin255/nginx-jump-gui/internal/model"
	"github.com/lianxin255/nginx-jump-gui/internal/paths"
	"github.com/lianxin255/nginx-jump-gui/internal/services"
	njui "github.com/lianxin255/nginx-jump-gui/internal/ui"
)

// 出图脚本：真数据 + 真视图 + Tester，不依赖窗口。
func main() {
	os.Setenv("NGJ_DATA_ROOT", "/tmp/shots-data")
	os.Setenv("NGJ_RESOURCES", "../resources")
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
	_ = a.Reload()

	shoot := func(name string, page appt.Page, theme appt.ThemeChoice, dark bool, prep func()) {
		a.SetPage(page)
		a.SetTheme(theme)
		if prep != nil {
			prep()
		}
		tt := ui.NewTester(njui.NewView(a), 1280, 800)
		tt.SetDark(dark)
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
