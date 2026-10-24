package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	// The log and history lists keep their scroll position in the view:
	// a ListState built inside a frame forgets it every frame, so the
	// list could never be scrolled past its first page.
	logList       ui.ListState
	historyList   ui.ListState
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
	// searchBorn counts ⌘F presses. AutoFocus only works on the frame an
	// element is born, so the shortcut changes the search box's Key: the
	// field is reborn and takes the caret, no clicking needed.
	searchBorn int

	// AI settings, loaded once from the store. The model list is filled by
	// 拉取模型, whose goroutine writes it, so it sits behind aiMu and the
	// frame reads a snapshot.
	aiSettingsLoaded bool
	aiBaseURL        string
	aiModel          string
	aiKey            string
	aiKeySet         bool
	aiMu             sync.Mutex
	aiModels         []string
	aiFetching       bool
	aiListErr        string
	// modelPicker shows the fetched list under the model field.
	modelPicker bool
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
			v.searchBorn++
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
//
// The pages are tabs with an underline on the chosen one, centered in the
// bar, the way the design asks: the segmented pill made the navigation
// shout when the page title below already says where you are.
//
// The bar is the window's title bar: the window is created with
// TitleBarHiddenInset, so this row reaches the top of the window, pads
// itself clear of the traffic lights, and drags the window from anywhere
// that is not a control.
func (v *View) topbar(c *ui.Context, t Tokens) {
	a := v.App
	_, _, _, _, engine, dirty, busy, _ := a.Snapshot()
	page := int(a.Page())
	bar := c.TitleBar()

	left := bar.Left + 14
	if bar.Left == 0 {
		left = 24
	}
	right := bar.Right + 14
	if bar.Right == 0 {
		right = 24
	}

	ui.Row(c).FillWidth().Height(56).Padding(0, right, 0, left).
		Background(t.Topbar).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).
		AlignItems(ui.Center).Gap(10).DragWindow().Children(func() {

		// The wordmark, not a square logo: the blue box beside red, amber
		// and green read as a fourth window control. macOS puts the app
		// name here, like any title bar; the icon lives in the Dock.
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "JumpBoard").FontSize(14).Bold().TextColor(t.FG)
			ui.Text(c, "内网转发").FontSize(10).TextColor(t.Muted)
		})

		ui.Spacer(c)

		labels := []string{"映射", "日志", "历史", "设置"}
		tabs := ui.TabsBase(c, &page, len(labels))
		tabs.List.Gap(4).AlignItems(ui.Center).Children(func() {
			for i, label := range labels {
				on := i == page
				tabs.Tab(i).Padding(18, 14).Children(func() {
					ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
						text := ui.Text(c, label).FontSize(13)
						if on {
							text.Bold().TextColor(t.FG)
						} else {
							text.TextColor(t.Muted)
						}
						bar := ui.Color{}
						if on {
							bar = t.Accent
						}
						ui.Box(c).Size(22, 2).Radius(2).Background(bar)
					})
				})
			}
		})
		if tabs.List.Changed() {
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
		// One pill for the engine, one for what it is doing: two borders
		// around two short phrases read as clutter.
		dot := t.Success
		if engine != "running" {
			dot = t.Muted
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(5, 10).Radius(4).
			Background(t.Surface3).Border(1, t.Border).Children(func() {
			ui.Box(c).Size(7, 7).Radius(4).Background(dot)
			ui.Text(c, a.Eng.Mode()).FontSize(12).TextColor(t.FG2)
			ui.Text(c, "·").FontSize(12).TextColor(t.Muted)
			ui.Text(c, engineLabel(engine)).FontSize(12).TextColor(dot)
		})
		if len(rules) > 0 {
			ui.Row(c).Gap(4).Padding(5, 10).Radius(4).Background(t.Surface3).Border(1, t.Border).Children(func() {
				ui.Textf(c, "监听 %d/%d 条 · 累计 %d 连接", listening, len(rules), conns).
					FontSize(12).TextColor(t.FG2)
			})
		}
		ui.Spacer(c)
		if busy {
			ui.Spinner(c).Label("正在处理")
		}
		if dirty {
			ui.Text(c, "有未生效的改动").FontSize(11).TextColor(t.Warning)
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
			Background(t.Danger.Alpha(0.12)).Children(func() {
			ui.Text(c, msg).FontSize(11).TextColor(t.Danger).Selectable()
		})
	}
	if msg := a.ApplyWarning(); msg != "" {
		ui.Box(c).FillWidth().Padding(8, 10).Radius(3).
			Background(t.Warning.Alpha(0.12)).Children(func() {
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
				ui.Text(c, "转发规则").FontSize(24).Bold().TextColor(t.FG)
				enabled := 0
				for _, m := range mappings {
					if m.Enabled {
						enabled++
					}
				}
				if len(mappings) == 0 {
					ui.Text(c, "把内网服务端口从这台机器转出去").FontSize(12).TextColor(t.Muted)
				} else {
					ui.Textf(c, "本地端口对外 · %d 条规则 · %d 启用", len(mappings), enabled).
						FontSize(12).TextColor(t.Muted)
				}
			})
			ui.Spacer(c)
			// Empty state keeps one clear CTA in the card below; the toolbar
			// filter/search/new row only appears once there are rules. AI stays
			// reachable on the empty screen so a first rule can still be drafted.
			if len(mappings) > 0 {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Segmented(c, &v.filter, "全部", "已启用", "已停用").Label("筛选").Width(190)
					v.searchBox(c, t)
					v.aiEntry(c, t)
					if ui.Button(c, "从模板新建").Disabled(busy).Clicked() {
						a.SetTemplatePicker(true)
					}
					if ui.PrimaryButton(c, "新建映射").Disabled(busy).
						Tooltip("新建一条映射（⌘N）").Clicked() {
						a.OpenForm(nil)
					}
				})
			} else {
				v.aiEntry(c, t)
			}
		})

		if len(mappings) == 0 {
			v.mappingsEmpty(c, t, busy)
			return
		}

		// The rows keep their state by ID as the list changes. No row is
		// chosen: the table's highlight paints over the badges and status
		// colors a row carries, and nothing here acts on a chosen row.
		v.table.Key = func(i int) any { return filtered[i].ID }
		v.table.Label = func(i int) string { return filtered[i].Name }

		ui.Column(c).Fill().Radius(10).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
			cols := []ui.TableColumn{
				{Title: "规则", Width: 240},
				{Title: "本地 → 上游"},
				{Title: "健康", Width: 100},
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
//
// The badge is a filled chip, and the live state is a dot on the line
// under the name: the row's second line already says the protocol, so
// repeating it was noise, and a dot beside the name competed with it.
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
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Padding(3, 8).Radius(4).
			Background(color.Alpha(0.14)).Children(func() {
			ui.Text(c, m.Protocol.Label()).FontSize(10).Bold().TextColor(color)
		})
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, m.Name).FontSize(13).Bold().TextColor(t.FG).SingleLine()
			sub := displayHost(m)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
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
					ui.Box(c).Size(6, 6).Radius(3).Background(dot).Tooltip(hint)
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

// searchBox is the mappings page's filter: a plain text input, so that
// ⌘F can focus it — the input itself carries the AutoFocus, and changing
// its Key on the shortcut rebirths it, which is the one moment AutoFocus
// takes the caret. (The toolkit's SearchField hides its input inside a
// wrapper that neither Handle.Focus nor AutoFocus can reach.)
func (v *View) searchBox(c *ui.Context, t Tokens) {
	ui.Box(c).Width(220).Padding(0, 10).Radius(6).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "⌕").FontSize(13).TextColor(t.Muted)
			// The key goes on the context before the input is built: a
			// TextInput owns its buffer in state, so its identity has to
			// be fixed up front, not changed afterwards.
			ic := c.Key("search-" + strconv.Itoa(v.searchBorn))
			ui.TextInput(ic, &v.query).Placeholder("搜索规则").Grow(1).
				Label("搜索规则").
				Tooltip("按名称、域名、端口搜索（⌘F）").
				AutoFocus()
			if v.query != "" {
				if ui.Text(c, "×").FontSize(12).TextColor(t.Muted).
					Label("清空搜索").Clicked() {
					v.query = ""
				}
			}
		})
	})
}

// mappingsEmpty is the first-run card: one clear path to add a rule.
func (v *View) mappingsEmpty(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	ui.Column(c).Fill().Center().Gap(14).Children(func() {
		ui.Box(c).Width(440).Padding(28, 28).Radius(12).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Column(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "还没有转发规则").FontSize(18).Bold().TextColor(t.FG)
				ui.Text(c, "例如把内网 MySQL、SSH、Web 转到这台机器的端口上。").
					FontSize(13).TextColor(t.Muted)
				ui.Text(c, "建好后点「应用配置」，立刻对外生效。").FontSize(13).TextColor(t.Muted)
				ui.Row(c).Gap(10).Children(func() {
					if ui.PrimaryButton(c, "新建映射").Disabled(busy).
						Tooltip("新建一条映射（⌘N）").Clicked() {
						a.OpenForm(nil)
					}
					if ui.Button(c, "从模板新建").Disabled(busy).Clicked() {
						a.SetTemplatePicker(true)
					}
				})
				ui.Text(c, "常用模板：SSH · RDP · MySQL · Redis · Web").FontSize(11).TextColor(t.Muted)
			})
		})
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
