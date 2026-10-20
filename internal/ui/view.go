package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// View is the window's content: it draws the app from its state.
//
// The fields below the app are the view's own: the text in the search
// box, the table's scroll and choice, and the renderers' caches. They are
// only touched inside a frame, on the main thread.
type View struct {
	App *appt.App

	query string
	table ui.ListState

	// The forms' own state, which the app's state does not need to know
	// about until it is saved.
	whitelistIP   string
	whitelistNote string
	logTab        int
	logLevel      int
	logQuery      string
	nginxAccess   []string
	nginxErrors   []string
	nginxLoadedAt int64
	settingsName  string
	nameLoaded    bool

	ai aiState
	md markdownState

	// AI settings, loaded once from the store.
	aiSettingsLoaded bool
	aiBaseURL        string
	aiModel          string
	aiKey            string
	aiKeySet         bool
}

// NewView returns the window's content for an app.
func NewView(a *appt.App) func(c *ui.Context) {
	v := &View{App: a}
	return v.draw
}

// draw builds one frame.
func (v *View) draw(c *ui.Context) {
	// The toolkit's theme is reset every frame, so the palette is applied
	// at the start of each one. Following the desktop needs the appearance
	// it reports, which is the theme the frame started with.
	systemDark := c.Theme().Dark
	t := TokensFor(v.App.Theme(), systemDark)
	c.SetTheme(theme(t))

	ui.Column(c).Fill().Background(t.Base).Children(func() {
		v.topbar(c, t)
		v.body(c, t)
	})
	v.dialogs(c, t)
	v.toast(c, t)
}

// topbar draws the header: the app's name, the pages, the engine state and
// the buttons that act on it.
func (v *View) topbar(c *ui.Context, t Tokens) {
	a := v.App
	_, _, _, _, engine, dirty, busy, _ := a.Snapshot()
	page := int(a.Page())

	ui.Row(c).FillWidth().Height(56).Padding(0, 24).
		Background(t.Topbar).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).
		AlignItems(ui.Center).Gap(10).Children(func() {

		ui.Box(c).Size(26, 26).Radius(4).Background(t.Accent).Children(func() {
			ui.Text(c, "跳").FontSize(13).Bold().TextColor(t.AccentText)
		})
		ui.Text(c, "NginxJump").FontSize(14).Bold().TextColor(t.FG)
		ui.Text(c, "v1.1").FontSize(11).TextColor(t.Muted)

		ui.Spacer(c)

		if ui.Segmented(c, &page, "映射", "日志", "历史", "设置").Label("页面").Width(340).Changed() {
			a.SetPage(appt.Page(page))
		}

		ui.Spacer(c)

		// The engine state: a dot, a word, and the ability to start and
		// stop it. A dirty configuration shows as a hint next to the
		// apply button in the page body.
		dot := t.Success
		label := "运行中"
		if engine != "running" {
			dot, label = t.Muted, "已停止"
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(7, 7).Radius(4).Background(dot)
			ui.Text(c, label).FontSize(12).TextColor(dot)
		})
		if engine == "running" {
			if ui.Button(c, "停止").Tooltip("停止 nginx，所有转发立即失效").Disabled(busy).Clicked() {
				a.Ask(&appt.Confirm{
					Title: "停止 nginx",
					Body:  "停止后所有转发规则立即失效，外部无法访问。",
					Label: "停止", Destructive: true,
					Run: a.StopEngine,
				})
			}
		} else {
			if ui.PrimaryButton(c, "启动").Disabled(busy).Clicked() {
				a.RunBackground(a.StartEngine)
			}
		}
		if dirty {
			ui.Badge(c, "未应用").Background(t.Warning).TextColor(ui.Hex("#141413"))
		}
	})
}

// body draws the page shown.
func (v *View) body(c *ui.Context, t Tokens) {
	switch v.App.Page() {
	case appt.PageLogs:
		v.logsPage(c, t)
	case appt.PageHistory:
		v.historyPage(c, t)
	case appt.PageSettings:
		v.settingsPage(c, t)
	default:
		v.mappingsPage(c, t)
	}
}

// actionRow draws the engine state line and the buttons that change the
// configuration.
func (v *View) actionRow(c *ui.Context, t Tokens) {
	a := v.App
	_, _, _, _, engine, dirty, busy, _ := a.Snapshot()
	pid := a.Eng.PID()

	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(5, 10).Radius(3).
			Background(t.Surface3).Border(1, t.Border).Children(func() {
			dot := t.Success
			if engine != "running" {
				dot = t.Muted
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(dot)
			ui.Text(c, "nginx").FontSize(12).TextColor(t.FG2)
			ui.Text(c, engineLabel(engine)).FontSize(12).TextColor(dot)
		})
		if pid > 0 {
			ui.Row(c).Padding(5, 10).Radius(3).Background(t.Surface3).Border(1, t.Border).Children(func() {
				ui.Textf(c, "PID %d", pid).FontSize(12).TextColor(t.FG2)
			})
		}
		ui.Spacer(c)
		if busy {
			ui.Spinner(c).Label("正在处理")
		}
		if dirty {
			ui.Text(c, "配置已改动").FontSize(11).TextColor(t.Warning)
		}
		if ui.Button(c, "校验配置").Disabled(busy).Clicked() {
			a.RunBackground(a.TestConfig)
		}
		if ui.PrimaryButton(c, "应用配置").Disabled(busy).Clicked() {
			a.RunBackground(a.Apply)
		}
	})
	if msg := a.ApplyError(); msg != "" {
		ui.Box(c).FillWidth().Padding(8, 10).Radius(3).
			Background(ui.RGBA(181, 51, 51, 0.12)).Children(func() {
			ui.Text(c, msg).FontSize(11).TextColor(t.Danger)
		})
	}
}

func engineLabel(s string) string {
	if s == "running" {
		return "运行中"
	}
	return "已停止"
}

// mappingsPage draws the forwarding rules.
func (v *View) mappingsPage(c *ui.Context, t Tokens) {
	a := v.App
	mappings, _, _, _, _, _, busy, _ := a.Snapshot()

	filtered := filterMappings(mappings, v.query)

	ui.Column(c).Fill().Padding(20, 28).Gap(14).Children(func() {
		v.actionRow(c, t)

		ui.Row(c).AlignItems(ui.End).Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, "端口映射").FontSize(26).Bold().TextColor(t.FG)
				enabled := 0
				for _, m := range mappings {
					if m.Enabled {
						enabled++
					}
				}
				ui.Textf(c, "%d 条规则 · %d 启用 · %d 停用", len(mappings), enabled, len(mappings)-enabled).
					FontSize(12).TextColor(t.Muted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &v.query).Label("搜索规则").Width(240)
			v.aiEntry(c, t)
			if ui.Button(c, "从模板新建").Disabled(busy).Clicked() {
				a.SetTemplatePicker(true)
			}
			if ui.PrimaryButton(c, "新建映射").Disabled(busy).Clicked() {
				a.OpenForm(nil)
			}
		})

		if len(mappings) == 0 {
			v.emptyState(c, t, "还没有映射", "点“新建映射”手动添加，或从模板开始。")
			return
		}

		// The rows keep their state by ID as the list changes. No row is
		// chosen: the table's highlight paints over the badges and status
		// colors a row carries, and nothing here acts on a chosen row.
		v.table.Key = func(i int) any { return filtered[i].ID }
		v.table.Label = func(i int) string { return filtered[i].Name }

		ui.Column(c).Fill().Radius(4).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
			cols := []ui.TableColumn{
				{Title: "规则 / 协议", Width: 240},
				{Title: "转发关系"},
				{Title: "健康检查", Width: 100},
				{Title: "启用", Width: 70},
				{Title: "操作", Width: 210},
			}
			ui.Table(c, &v.table, cols, len(filtered), func(row, col int) {
				m := filtered[row]
				switch col {
				case 0:
					v.nameCell(c, t, m)
				case 1:
					v.forwardCell(c, t, m)
				case 2:
					v.healthCell(c, t, m)
				case 3:
					on := m.Enabled
					if ui.Switch(c, &on).Label("启用-" + m.Name).Changed() {
						id := m.ID
						a.RunBackground(func() error { return a.SetMappingEnabled(id, on) })
					}
				case 4:
					v.rowActions(c, t, m, busy)
				}
			}).Grow(1).Children(func() {
				if len(filtered) == 0 {
					ui.Text(c, "没有匹配的规则").FontSize(12).TextColor(t.Muted).Padding(14)
				}
			})
		})
	})
}

// nameCell shows the protocol badge and the mapping's name.
func (v *View) nameCell(c *ui.Context, t Tokens, m model.Mapping) {
	color := t.TCP
	switch m.Protocol {
	case model.UDP:
		color = t.UDP
	case model.HTTP:
		color = t.HTTP
	case model.HTTPS:
		color = t.HTTPS
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Padding(2, 7).Radius(2).Border(1, color).Children(func() {
			ui.Text(c, m.Protocol.Label()).FontSize(10).Bold().TextColor(color)
		})
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, m.Name).FontSize(13).TextColor(t.FG).SingleLine()
			sub := strings.ToUpper(string(m.Protocol)) + " · " + displayHost(m)
			ui.Text(c, sub).FontSize(11).TextColor(t.Muted).SingleLine()
		})
	})
}

// forwardCell shows what the mapping listens on and where it goes.
func (v *View) forwardCell(c *ui.Context, t Tokens, m model.Mapping) {
	port := m.ListenPort
	if port == 0 {
		if m.Protocol == model.HTTPS {
			port = 443
		} else {
			port = 80
		}
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if m.ServerNames != "" {
			ui.Text(c, m.ServerNames).FontSize(11).TextColor(t.FG2).
				Padding(2, 6).Radius(2).Background(t.Surface3)
		} else {
			ui.Text(c, "外部").FontSize(11).TextColor(t.Muted).
				Padding(2, 6).Radius(2).Background(t.Surface3)
		}
		ui.Textf(c, ":%d", port).FontSize(12).TextColor(t.Accent)
		ui.Text(c, "→").FontSize(12).TextColor(t.Muted)
		ui.Textf(c, "%s:%d", m.UpstreamHost, m.UpstreamPort).FontSize(12).TextColor(t.FG2)
	})
}

// displayHost is the host part shown under a mapping's name.
func displayHost(m model.Mapping) string {
	if m.ServerNames != "" {
		return m.ServerNames
	}
	return m.UpstreamHost
}

// healthCell shows the last check's outcome.
func (v *View) healthCell(c *ui.Context, t Tokens, m model.Mapping) {
	if !m.HealthCheckEnabled {
		ui.Text(c, "○ 未启用").FontSize(12).TextColor(t.Muted)
		return
	}
	switch m.HealthStatus {
	case model.HealthHealthy:
		ui.Text(c, "● 健康").FontSize(12).TextColor(t.Success)
	case model.HealthUnhealthy:
		if ui.Text(c, "● 不可达").FontSize(12).TextColor(t.Danger).Tooltip("点一下重新检查").Clicked() {
			mm := m
			v.App.RunBackground(func() error { return v.App.CheckNow(mm) })
		}
	default:
		if ui.Text(c, "○ 未检查").FontSize(12).TextColor(t.Muted).Clicked() {
			mm := m
			v.App.RunBackground(func() error { return v.App.CheckNow(mm) })
		}
	}
}

// rowActions draws the per-row buttons.
func (v *View) rowActions(c *ui.Context, t Tokens, m model.Mapping, busy bool) {
	a := v.App
	id := m.ID
	ui.Row(c).Gap(4).Children(func() {
		if ui.Button(c, "编辑").FontSize(11).Disabled(busy).Clicked() {
			mm := m
			a.OpenForm(&mm)
		}
		if ui.Button(c, "克隆").FontSize(11).Disabled(busy).Clicked() {
			a.RunBackground(func() error { return a.CloneMapping(id) })
		}
		if ui.Button(c, "删除").FontSize(11).Disabled(busy).Clicked() {
			name := m.Name
			a.Ask(&appt.Confirm{
				Title:       "删除映射",
				Body:        fmt.Sprintf("「%s」将被删除，需要重新创建。", name),
				Label:       "确认删除",
				Destructive: true,
				Run:         func() error { return a.DeleteMapping(id) },
			})
		}
	})
}

// emptyState draws the placeholder a page shows with nothing in it.
func (v *View) emptyState(c *ui.Context, t Tokens, title, hint string) {
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		ui.Text(c, title).FontSize(15).Bold().TextColor(t.FG2)
		ui.Text(c, hint).FontSize(12).TextColor(t.Muted)
	})
}

// filterMappings keeps the rules whose name, host or port matches the
// query.
func filterMappings(all []model.Mapping, query string) []model.Mapping {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return all
	}
	out := make([]model.Mapping, 0, len(all))
	for _, m := range all {
		hay := strings.ToLower(fmt.Sprintf("%s %s %s %d %d %s",
			m.Name, m.ServerNames, m.UpstreamHost, m.ListenPort, m.UpstreamPort, m.Protocol))
		if strings.Contains(hay, q) {
			out = append(out, m)
		}
	}
	return out
}
