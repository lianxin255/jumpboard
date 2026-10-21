package ui

import (
	"fmt"
	"strings"
	"time"

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

	query  string
	filter int
	table  ui.ListState

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

	// Last status shown as a toast, so that it is not added every frame.
	shownStatusAt   time.Time
	shownStatusText string
	// focusSearch asks the next frame to put the caret in the search box.
	focusSearch bool

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
	v.shortcuts(c)
}

// shortcuts wires the keys a desktop app is expected to answer.
func (v *View) shortcuts(c *ui.Context) {
	a := v.App
	c.OnShortcut(ui.Cmd, ui.KeyN, func() {
		if a.Page() == appt.PageMappings && !a.FormOpen() {
			a.OpenForm(nil)
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if a.Page() == appt.PageMappings {
			v.focusSearch = true
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyR, func() {
		if a.Page() == appt.PageMappings {
			a.RunBackground(a.Apply)
		}
	})
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
			if ui.Button(c, "停止").Tooltip("停止引擎，所有转发立即失效").Disabled(busy).Clicked() {
				a.Ask(&appt.Confirm{
					Title: "停止引擎",
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
			ui.Badge(c, "未应用").Background(t.Warning).TextColor(t.Base)
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
	rules := a.RuleStatus()
	listening, conns := 0, int64(0)
	for _, r := range rules {
		if r.Listening {
			listening++
			conns += r.Connections
		}
	}

	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(5, 10).Radius(3).
			Background(t.Surface3).Border(1, t.Border).Children(func() {
			dot := t.Success
			if engine != "running" {
				dot = t.Muted
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(dot)
			ui.Text(c, a.Eng.Mode()).FontSize(12).TextColor(t.FG2)
			ui.Text(c, engineLabel(engine)).FontSize(12).TextColor(dot)
		})
		if len(rules) > 0 {
			ui.Row(c).Padding(5, 10).Radius(3).Background(t.Surface3).Border(1, t.Border).Children(func() {
				ui.Textf(c, "监听 %d/%d 条 · 累计 %d 连接", listening, len(rules), conns).
					FontSize(12).TextColor(t.FG2)
			})
		}
		ui.Spacer(c)
		if busy {
			ui.Spinner(c).Label("正在处理")
		}
		if dirty {
			ui.Text(c, "配置已改动").FontSize(11).TextColor(t.Warning)
		}
		// The check binds each port for a moment and resolves the
		// upstreams: it answers "would applying work", without touching
		// what is running.
		if ui.Button(c, "预检").Disabled(busy).
			Tooltip("检查端口是否被占用、上游地址能否解析，不会改动正在跑的规则").Clicked() {
			a.RunBackground(a.TestConfig)
		}
		if ui.PrimaryButton(c, "应用配置").Disabled(busy).
			Tooltip("让改动生效（⌘R）").Clicked() {
			a.RunBackground(a.Apply)
		}
	})
	if msg := a.ApplyError(); msg != "" {
		ui.Box(c).FillWidth().Padding(8, 10).Radius(3).
			Background(ui.RGBA(181, 51, 51, 0.12)).Children(func() {
			ui.Text(c, msg).FontSize(11).TextColor(t.Danger).Selectable()
		})
	}
	if msg := a.ApplyWarning(); msg != "" {
		ui.Box(c).FillWidth().Padding(8, 10).Radius(3).
			Background(ui.RGBA(201, 162, 58, 0.12)).Children(func() {
			ui.Text(c, msg).FontSize(11).TextColor(t.Warning).Selectable()
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

	filtered := filterMappings(mappings, v.query, v.filter)

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
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Segmented(c, &v.filter, "全部", "已启用", "已停用").Label("筛选").Width(190)
				search := ui.SearchField(c, &v.query).Label("搜索规则").Width(200).
					Tooltip("按名称、域名、端口搜索（⌘F）")
				if v.focusSearch {
					search.AutoFocus()
					v.focusSearch = false
				}
				v.aiEntry(c, t)
				if ui.Button(c, "从模板新建").Disabled(busy).Clicked() {
					a.SetTemplatePicker(true)
				}
				if ui.PrimaryButton(c, "新建映射").Disabled(busy).
					Tooltip("新建一条映射（⌘N）").Clicked() {
					a.OpenForm(nil)
				}
			})
		})

		if len(mappings) == 0 {
			v.emptyState(c, t, "还没有映射",
				"按 ⌘N 新建，或者从模板开始：数据库、Redis、Web 站点的常用端口都填好了。")
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
					hint := "没有匹配的规则"
					if len(mappings) == 0 {
						hint = ""
					}
					if hint != "" {
						ui.Text(c, hint).FontSize(12).TextColor(t.Muted).Padding(14)
					}
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
	listening, problem := v.App.RuleState(m.ID)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Padding(2, 7).Radius(2).Border(1, color).Children(func() {
			ui.Text(c, m.Protocol.Label()).FontSize(10).Bold().TextColor(color)
		})
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, m.Name).FontSize(13).TextColor(t.FG).SingleLine()
			sub := strings.ToUpper(string(m.Protocol)) + " · " + displayHost(m)
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				// The dot says whether the rule is listening right now. It
				// is only drawn once the engine has been applied to: before
				// that, "not listening" is the expected state, not news.
				dot := t.Muted
				hint := ""
				if v.App.EngineApplied() {
					switch {
					case !m.Enabled:
						hint = "已停用"
					case listening:
						dot, hint = t.Success, "监听中"
					case problem == "" || problem == "未应用":
						hint = "改动还没应用"
					default:
						dot, hint = t.Danger, problem
					}
					ui.Box(c).Size(5, 5).Radius(3).Background(dot).Tooltip(hint)
				}
				ui.Text(c, sub).FontSize(11).TextColor(t.Muted).SingleLine()
				// A rule that cannot listen says why in the row itself:
				// a tooltip is no help when the port is already taken.
				if v.App.EngineApplied() && m.Enabled && !listening && problem != "" && problem != "未应用" {
					ui.Text(c, problem).FontSize(10).TextColor(t.Danger).SingleLine()
				}
			})
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
	info := v.App.RuleInfo(m.ID)
	a := v.App
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if m.ServerNames != "" {
			ui.Text(c, m.ServerNames).FontSize(11).TextColor(t.FG2).
				Padding(2, 6).Radius(2).Background(t.Surface3)
		} else {
			ui.Text(c, "外部").FontSize(11).TextColor(t.Muted).
				Padding(2, 6).Radius(2).Background(t.Surface3)
		}
		// The address is what someone shares, so a click puts it on the
		// clipboard rather than making them select the text by hand.
		if ui.Textf(c, ":%d", port).FontSize(12).TextColor(t.Accent).
			Label("复制地址-" + m.Name).Tooltip("点一下复制这个地址").Clicked() {
			mm := m
			a.RunBackground(func() error { return a.CopyAddress(mm) })
		}
		ui.Text(c, "→").FontSize(12).TextColor(t.Muted)
		ui.Textf(c, "%s:%d", m.UpstreamHost, m.UpstreamPort).FontSize(12).TextColor(t.FG2)
		if info.Connections > 0 {
			ui.Textf(c, "· %d 次", info.Connections).FontSize(10).TextColor(t.Muted).
				Tooltip("这条规则累计转发的连接数")
		}
		if info.Active > 0 {
			ui.Textf(c, "· %d 活跃", info.Active).FontSize(10).TextColor(t.Success)
		}
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
	// The label carries the mapping's name: several rows can show the same
	// status, and a control has to say which one it acts on.
	label := "检查-" + m.Name
	switch m.HealthStatus {
	case model.HealthHealthy:
		if ui.Text(c, "● 健康").FontSize(12).TextColor(t.Success).Label(label).
			Tooltip("点一下重新检查").Clicked() {
			mm := m
			v.App.RunBackground(func() error { return v.App.CheckNow(mm) })
		}
	case model.HealthUnhealthy:
		if ui.Text(c, "● 不可达").FontSize(12).TextColor(t.Danger).Label(label).
			Tooltip("点一下重新检查").Clicked() {
			mm := m
			v.App.RunBackground(func() error { return v.App.CheckNow(mm) })
		}
	default:
		if ui.Text(c, "○ 未检查").FontSize(12).TextColor(t.Muted).Label(label).
			Tooltip("点一下检查").Clicked() {
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

// filterMappings keeps the rules the search and the filter let through.
func filterMappings(all []model.Mapping, query string, filter int) []model.Mapping {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]model.Mapping, 0, len(all))
	for _, m := range all {
		switch filter {
		case 1:
			if !m.Enabled {
				continue
			}
		case 2:
			if m.Enabled {
				continue
			}
		}
		if q != "" {
			hay := strings.ToLower(fmt.Sprintf("%s %s %s %d %d %s",
				m.Name, m.ServerNames, m.UpstreamHost, m.ListenPort, m.UpstreamPort, m.Protocol))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, m)
	}
	return out
}
