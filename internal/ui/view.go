package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
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
	// detailID is the mapping whose detail popover is open, "" for none.
	// One popover at a time: opening another row's 详情 closes the first.
	detailID string

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
	// aiDiagModel is the optional 诊断/解读模型 field; empty means the
	// main model, which is what its placeholder says.
	aiDiagModel string
	aiKey       string
	aiKeySet    bool
	aiMu        sync.Mutex
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

	// The sidebar material shows wherever nothing is painted. The content
	// column always paints, so the frost stays in the rail. Without a
	// material the rail paints its own frost color and the layout is the
	// same.
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	} else {
		c.Root().Background(t.Canvas)
	}

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		v.sidebar(c, t)
		ui.Column(c).Grow(1).MinWidth(0).Background(t.Canvas).Children(func() {
			v.body(c, t)
		})
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

// sidebar is the frosted rail: brand, pages, and the engine. It draws no
// background when the window has a material, so the frost shows through;
// otherwise it paints the sidebar color. The pages keep the names the
// tests click: 映射, 日志, 历史, 设置.
func (v *View) sidebar(c *ui.Context, t Tokens) {
	a := v.App
	_, _, _, _, engine, _, busy, _ := a.Snapshot()
	page := int(a.Page())
	bar := c.TitleBar()

	side := ui.Column(c).Width(SidebarW).Shrink(0)
	if c.Vibrancy() {
		side.Background(ui.Transparent)
	} else {
		side.Background(t.Sidebar)
	}
	side.BorderWidth(0, 1, 0, 0).BorderColor(t.Hair).Children(func() {
		// Room for the traffic lights, then the brand. Dragging the brand
		// moves the window; the nav items below do not.
		top := bar.Height
		if top < 16 {
			top = 16
		}
		ui.Box(c).Height(top).Shrink(0).FillWidth().DragWindow()

		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(4, 16, 16, 16).DragWindow().Children(func() {
			ui.Box(c).Size(30, 30).Radius(8).
				Background(t.Accent).Children(func() {
				ui.Column(c).Fill().Center().Children(func() {
					ui.Text(c, "跳").FontSize(16).Bold().TextColor(t.AccentText)
				})
			})
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "JumpBoard").FontSize(14).Bold().TextColor(t.FG)
				ui.Text(c, "内网端口转发").FontSize(10).TextColor(t.Muted)
			})
		})

		labels := []string{"映射", "日志", "历史", "设置"}
		tabs := ui.TabsBase(c, &page, len(labels))
		ui.Column(c).Gap(2).Padding(0, 12).Children(func() {
			for i, label := range labels {
				on := i == page
				tab := tabs.Tab(i).FillWidth().Height(34).Radius(9).Padding(0, 10).
					AlignItems(ui.Center).Gap(10)
				if on {
					tab.Background(t.Accent.Alpha(0.08))
				}
				tab.Children(func() {
					mark := ui.Color{}
					if on {
						mark = t.Accent
					}
					ui.Box(c).Size(3, 14).Radius(2).Background(mark)
					text := ui.Text(c, label).FontSize(13)
					if on {
						text.Bold().TextColor(t.Accent)
					} else {
						text.TextColor(t.FG2)
					}
				})
			}
		})
		if tabs.List.Changed() {
			a.SetPage(appt.Page(page))
		}

		ui.Spacer(c)

		v.engineCard(c, t, engine, busy)
	})
}

// engineCard sits at the foot of the rail: a dot, the engine word, and
// start or stop. The words stay the ones the tests click.
func (v *View) engineCard(c *ui.Context, t Tokens, engine string, busy bool) {
	a := v.App
	dot := t.Success
	label := "运行中"
	if engine != "running" {
		dot, label = t.Muted, "已停止"
	}
	ui.Column(c).Gap(8).Padding(12, 14, 16, 14).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10).Radius(RadiusChip).
			Background(t.Chip).Border(1, t.Border).Children(func() {
			ui.Box(c).Size(7, 7).Radius(4).Background(dot)
			ui.Text(c, label).FontSize(12).Bold().TextColor(t.FG).Grow(1)
			if engine == "running" {
				ui.Text(c, a.Eng.Mode()).FontSize(10).TextColor(t.Meta)
			}
		})
		if engine == "running" {
			if ui.Button(c, "停止").FillWidth().Tooltip("停止引擎，所有转发立即失效").Disabled(busy).Clicked() {
				a.Ask(&appt.Confirm{
					Title: "停止引擎",
					Body:  "停止后所有转发规则立即失效，外部无法访问。",
					Label: "停止", Destructive: true,
					Run: a.StopEngine,
				})
			}
		} else if ui.PrimaryButton(c, "启动").FillWidth().Disabled(busy).Clicked() {
			a.RunBackground(a.StartEngine)
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
	_, _, _, _, _, dirty, busy, _ := a.Snapshot()
	rules := a.RuleStatus()
	listening, conns := 0, int64(0)
	for _, r := range rules {
		if r.Listening {
			listening++
			conns += r.Connections
		}
	}

	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		if len(rules) > 0 {
			ui.Textf(c, "监听 %d/%d · %d 连接", listening, len(rules), conns).
				FontSize(12).TextColor(t.Meta)
		}
		ui.Spacer(c)
		if busy {
			ui.Spinner(c).Label("正在处理")
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
	if dirty {
		v.pendingBar(c, t, busy)
	}
	if msg := a.ApplyError(); msg != "" {
		ui.Box(c).FillWidth().Padding(8, 12).Radius(RadiusChip).
			Background(t.Danger.Alpha(0.12)).Border(1, t.Danger.Alpha(0.28)).Children(func() {
			ui.Text(c, msg).FontSize(11).TextColor(t.Danger).Selectable()
		})
		v.diagSection(c, t, busy)
	}
	if msg := a.ApplyWarning(); msg != "" {
		ui.Box(c).FillWidth().Padding(8, 12).Radius(RadiusChip).
			Background(t.Warning.Alpha(0.12)).Border(1, t.Warning.Alpha(0.28)).Children(func() {
			ui.Text(c, msg).FontSize(11).TextColor(t.Warning).Selectable()
		})
	}
}

// pendingBar is the amber strip for a configuration that has not been
// applied yet. It stays in the page flow so a short window does not cover
// the rows, and it says 未应用 because that is the word the tests look for.
func (v *View) pendingBar(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MaxWidth(840).Height(52).Gap(12).AlignItems(ui.Center).
			Padding(0, 10, 0, 16).Radius(RadiusFloat).
			Background(t.AmberSoft).Border(1, t.AmberLine).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(t.Warning)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "有未生效的改动").FontSize(13).Bold().TextColor(t.AmberText)
				ui.Text(c, "未应用 · 重新加载后对外生效").FontSize(11).TextColor(t.AmberText)
			})
			if ui.PrimaryButton(c, "立即应用").Disabled(busy).
				Tooltip("应用配置（⌘R）").Clicked() {
				a.RunBackground(a.Apply)
			}
		})
	})
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
	enabled := 0
	for _, m := range mappings {
		if m.Enabled {
			enabled++
		}
	}

	// The whole page is a drop target: a backup or an nginx.conf dragged
	// anywhere on the rules imports, with the border lighting up while a
	// file hangs over it. DroppedFiles hands over the paths; what each
	// one becomes is importDropped's decision.
	page := ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(12)
	if paths := page.DroppedFiles(); paths != nil {
		v.importDropped(paths)
	}
	if page.FileDragOver() {
		page.Border(2, t.Accent)
	}
	page.Children(func() {

		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "转发规则").FontSize(TitleSize).Bold().TextColor(t.FG)
				if len(mappings) == 0 {
					ui.Text(c, "把内网服务端口从这台机器转出去").FontSize(12).TextColor(t.Meta)
				} else {
					ui.Textf(c, "%d 条规则 · %d 启用 · %d 停用", len(mappings), enabled, len(mappings)-enabled).
						FontSize(12).TextColor(t.Meta)
				}
			})
			ui.Spacer(c)
			// Search and the primary action sit on the title row. Filter,
			// templates and AI stay one row down so the title is not a toolbar.
			if len(mappings) > 0 {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					v.searchBox(c, t)
					if ui.PrimaryButton(c, "新建映射").Disabled(busy).
						Tooltip("新建一条映射（⌘N）").Clicked() {
						a.OpenForm(nil)
					}
				})
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if len(mappings) > 0 {
				// No fixed width: the control sizes to its labels. A width
				// narrower than the CJK labels overlapped the button beside it.
				ui.Segmented(c, &v.filter, "全部", "已启用", "已停用").Label("筛选")
				if ui.Button(c, "从模板新建").Disabled(busy).Clicked() {
					a.SetTemplatePicker(true)
				}
			}
			v.aiEntry(c, t)
			ui.Spacer(c)
		})
		v.actionRow(c, t)
		// A dropped conf leaves a bar offering the interpretation, and its
		// card lands right below: the answer sits where the drop did.
		v.confImportBar(c, t, busy)
		v.interpretSection(c, t, "import")

		if len(mappings) == 0 {
			v.mappingsEmpty(c, t, busy)
			return
		}

		v.batchBar(c, t, filtered, busy)

		if len(filtered) == 0 {
			v.filterEmpty(c, t)
			return
		}

		// The rows keep their state by ID as the list changes. No row is
		// chosen: the table's highlight paints over the badges and status
		// colors a row carries, and nothing here acts on a chosen row.
		v.table.Key = func(i int) any { return filtered[i].ID }
		v.table.Label = func(i int) string { return filtered[i].Name }

		ui.Column(c).Fill().Children(func() {
			cols := []ui.TableColumn{
				{Title: "规则", Width: 200},
				{Title: "协议", Width: 72},
				{Title: "转发关系"},
				{Title: "健康", Width: 100},
				{Title: "启用", Width: 64},
				{Title: "修改", Width: 200},
			}
			ui.Table(c, &v.table, cols, len(filtered), func(row, col int) {
				m := filtered[row]
				switch col {
				case 0:
					v.nameCell(c, t, m)
				case 1:
					v.protoCell(c, t, m)
				case 2:
					v.forwardCell(c, t, m)
				case 3:
					v.healthCell(c, t, m)
				case 4:
					on := m.Enabled
					if ui.Switch(c, &on).Label("启用-" + m.Name).Changed() {
						id := m.ID
						a.RunBackground(func() error { return a.SetMappingEnabled(id, on) })
					}
				case 5:
					v.rowActions(c, t, m, busy)
				}
			}).Grow(1)
		})
	})
}

// protoColor is the tint of a protocol chip.
func protoColor(t Tokens, p model.Protocol) ui.Color {
	switch p {
	case model.UDP:
		return t.UDP
	case model.HTTP:
		return t.HTTP
	case model.HTTPS:
		return t.HTTPS
	default:
		return t.TCP
	}
}

// protoCell is the protocol chip: a pale fill and ink of the same hue.
func (v *View) protoCell(c *ui.Context, t Tokens, m model.Mapping) {
	color := protoColor(t, m.Protocol)
	ui.Box(c).Padding(3, 8).Radius(RadiusBadge).
		Background(color.Alpha(0.12)).Children(func() {
		ui.Text(c, m.Protocol.Label()).FontSize(11).Bold().TextColor(color)
	})
}

// nameCell shows the mapping's name and the host under it. The protocol
// has its own column, so it is not repeated here. The cell carries the
// row's context menu: everything the 修改 column does as buttons, plus
// copy, one right-click away.
func (v *View) nameCell(c *ui.Context, t Tokens, m model.Mapping) {
	a := v.App
	id := m.ID
	listening, problem := v.App.RuleState(m.ID)
	cell := ui.Column(c).Gap(2)
	cell.ContextMenu(func(menu *ui.Menu) {
		if menu.Item("编辑").Chosen() {
			mm := m
			a.OpenForm(&mm)
		}
		if menu.Item("已启用").Checked(m.Enabled).Chosen() {
			mm := m
			a.RunBackground(func() error { return a.SetMappingEnabled(mm.ID, !mm.Enabled) })
		}
		menu.Separator()
		if menu.Item("克隆").Chosen() {
			a.RunBackground(func() error { return a.CloneMapping(id) })
		}
		if menu.Item("复制转发地址").Chosen() {
			mm := m
			a.RunBackground(func() error { return a.CopyAddress(mm) })
		}
		menu.Separator()
		if menu.Item("删除…").Chosen() {
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
	cell.Children(func() {
		// The name opens the detail popover; the anchor is the name
		// itself, so the panel points at what it is about. open is
		// recomputed from detailID every frame, which keeps the popover
		// honest when another row's name is clicked instead.
		name := ui.Text(c, m.Name).FontSize(13).Bold().TextColor(t.FG).SingleLine().
			Tooltip("点一下看详情，右键更多操作")
		if name.Clicked() {
			if v.detailID == m.ID {
				v.detailID = ""
			} else {
				v.detailID = m.ID
			}
		}
		open := v.detailID == m.ID
		ui.Popover(c, name, &open, func() { v.detailPopover(c, t, m) })
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			// The dot says whether the rule is listening right now. It is
			// only drawn once the engine has been applied to: before that,
			// "not listening" is the expected state, not news.
			if v.App.EngineApplied() {
				dot := t.Muted
				hint := ""
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
			ui.Text(c, displayHost(m)).FontSize(11).TextColor(t.Meta).SingleLine()
			// A rule that cannot listen says why in the row itself.
			if v.App.EngineApplied() && m.Enabled && !listening && problem != "" && problem != "未应用" {
				ui.Text(c, problem).FontSize(10).TextColor(t.Danger).SingleLine()
			}
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
	localLabel := fmt.Sprintf("127.0.0.1:%d", port)
	if m.ServerNames != "" {
		localLabel = fmt.Sprintf("%s:%d", m.ServerNames, port)
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		// The address is what someone shares, so a click puts it on the
		// clipboard rather than making them select the text by hand.
		if ui.Text(c, localLabel).Font("monospace").FontSize(12).TextColor(t.FG2).
			Label("复制地址-"+m.Name).Tooltip("点一下复制本地地址").
			Padding(4, 8).Radius(RadiusChip).Background(t.Chip).Border(1, t.Border).Clicked() {
			mm := m
			a.RunBackground(func() error { return a.CopyAddress(mm) })
		}
		ui.Text(c, "→").FontSize(12).TextColor(t.Meta)
		ui.Textf(c, "%s:%d", m.UpstreamHost, m.UpstreamPort).Font("monospace").FontSize(12).TextColor(t.FG2).
			Tooltip("上游目标")
		if info.Connections > 0 {
			ui.Textf(c, "· %d 次", info.Connections).FontSize(10).TextColor(t.Meta).
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

// healthCell shows the last check's outcome as a pill.
func (v *View) healthCell(c *ui.Context, t Tokens, m model.Mapping) {
	if !m.HealthCheckEnabled {
		ui.Box(c).Padding(3, 8).Radius(RadiusPill).Background(t.Chip).Children(func() {
			ui.Text(c, "未启用").FontSize(11).TextColor(t.Meta)
		})
		return
	}
	// The label carries the mapping's name: several rows can show the same
	// status, and a control has to say which one it acts on. The pill is
	// one text — the dot is a glyph — so the whole thing is clickable.
	label := "检查-" + m.Name
	word, color, fill, tip := "○ 未检查", t.Meta, t.Chip, "点一下检查"
	switch m.HealthStatus {
	case model.HealthHealthy:
		word, color, fill, tip = "● 健康", t.Success, t.SuccessSoft, "点一下重新检查"
	case model.HealthUnhealthy:
		word, color, fill, tip = "● 异常", t.Danger, t.DangerSoft, "点一下重新检查"
	}
	if ui.Text(c, word).FontSize(11).Bold().TextColor(color).Label(label).
		Tooltip(tip).Padding(3, 10).Radius(RadiusPill).
		Background(fill).Clicked() {
		mm := m
		v.App.RunBackground(func() error { return v.App.CheckNow(mm) })
	}
}

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

// detailPopover is the mapping's summary panel: what it forwards, how it
// is doing, and the edit action. Small by design — the form is where
// editing happens; this is the look-before-you-click view.
func (v *View) detailPopover(c *ui.Context, t Tokens, m model.Mapping) {
	a := v.App
	ui.Column(c).Width(260).Padding(14).Gap(8).
		Background(t.Surface).Border(1, t.Border).Radius(RadiusChip).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			v.protoCell(c, t, m)
			if m.Enabled {
				ui.Text(c, "已启用").FontSize(11).TextColor(t.Meta)
			} else {
				ui.Text(c, "已停用").FontSize(11).TextColor(t.Meta)
			}
			ui.Spacer(c)
			if ui.Text(c, "×").FontSize(12).TextColor(t.Meta).Label("关闭详情").Clicked() {
				v.detailID = ""
			}
		})
		ui.Text(c, m.Name).FontSize(14).Bold().TextColor(t.FG)
		ui.Text(c, forwardLine(m)).Font("monospace").FontSize(12).TextColor(t.FG2).
			Tooltip("本地地址 → 上游目标")
		if m.ServerNames != "" {
			ui.Textf(c, "域名 %s", m.ServerNames).FontSize(11).TextColor(t.Meta)
		}
		if m.HealthCheckEnabled {
			word, color := "未检查", t.Meta
			switch m.HealthStatus {
			case model.HealthHealthy:
				word, color = "健康检查：正常", t.Success
			case model.HealthUnhealthy:
				word, color = "健康检查：异常", t.Danger
			}
			ui.Text(c, word).FontSize(11).TextColor(color)
		} else {
			ui.Text(c, "健康检查未启用").FontSize(11).TextColor(t.Meta)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "编辑").Clicked() {
				v.detailID = ""
				mm := m
				a.OpenForm(&mm)
			}
			if ui.Button(c, "关闭").Clicked() {
				v.detailID = ""
			}
		})
	})
}

// forwardLine is the mapping's forwarding relation as one line, for the
// detail popover.
func forwardLine(m model.Mapping) string {
	port := m.ListenPort
	if port == 0 {
		if m.Protocol == model.HTTPS {
			port = 443
		} else {
			port = 80
		}
	}
	return fmt.Sprintf("127.0.0.1:%d → %s:%d", port, m.UpstreamHost, m.UpstreamPort)
}

// searchBox is the mappings page's filter: a plain text input, so that
// ⌘F can focus it — the input itself carries the AutoFocus, and changing
// its Key on the shortcut rebirths it, which is the one moment AutoFocus
// takes the caret. (The toolkit's SearchField hides its input inside a
// wrapper that neither Handle.Focus nor AutoFocus can reach.)
func (v *View) searchBox(c *ui.Context, t Tokens) {
	ui.Box(c).Width(240).Padding(0, 10).Radius(RadiusButton).
		Background(t.Chip).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "⌕").FontSize(13).TextColor(t.Meta)
			// The key goes on the context before the input is built: a
			// TextInput owns its buffer in state, so its identity has to
			// be fixed up front, not changed afterwards.
			ic := c.Key("search-" + strconv.Itoa(v.searchBorn))
			ui.TextInput(ic, &v.query).Placeholder("搜索规则 / 端口 / IP").Grow(1).
				Label("搜索规则").
				Tooltip("按名称、域名、端口搜索（⌘F）").
				AutoFocus()
			if v.query != "" {
				if ui.Text(c, "×").FontSize(12).TextColor(t.Meta).
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
		ui.Box(c).Width(460).Padding(32, 28).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Column(c).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(36, 36).Radius(10).Background(t.Accent.Alpha(0.14)).
					Border(1, t.Accent.Alpha(0.28)).Children(func() {
					ui.Column(c).Fill().Center().Children(func() {
						ui.Text(c, "⇄").FontSize(16).TextColor(t.Accent)
					})
				})
				ui.Text(c, "还没有转发规则").FontSize(18).Bold().TextColor(t.FG)
				ui.Text(c, "把内网 MySQL、SSH、Web 等服务转到这台机器的端口上。").
					FontSize(13).TextColor(t.Muted)
				ui.Text(c, "建好后点「应用配置」或 ⌘R，立刻对外生效。").FontSize(13).TextColor(t.Muted)
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
	ui.Column(c).Fill().Center().Gap(10).Children(func() {
		ui.Box(c).Width(360).Padding(24, 20).Radius(RadiusPanel).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(15).Bold().TextColor(t.FG)
				ui.Text(c, hint).FontSize(12).TextColor(t.Muted)
			})
		})
	})
}

// batchBar offers enable/disable for the rules currently on screen.
func (v *View) batchBar(c *ui.Context, t Tokens, filtered []model.Mapping, busy bool) {
	a := v.App
	var onIDs, offIDs []string
	for _, m := range filtered {
		if m.Enabled {
			onIDs = append(onIDs, m.ID)
		} else {
			offIDs = append(offIDs, m.ID)
		}
	}
	scope := "当前列表"
	if v.query != "" || v.filter != 0 {
		scope = "筛选结果"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, "%s %d 条 · 启用 %d · 停用 %d", scope, len(filtered), len(onIDs), len(offIDs)).
			FontSize(11).TextColor(t.Muted)
		ui.Spacer(c)
		if len(offIDs) > 0 {
			if ui.Button(c, "全部启用").FontSize(11).Disabled(busy).
				Tooltip("启用当前列表中停用的规则，改完后需应用配置").Clicked() {
				ids := append([]string(nil), offIDs...)
				a.Ask(&appt.Confirm{
					Title: "全部启用",
					Body:  fmt.Sprintf("将启用 %s 中目前停用的 %d 条规则。", scope, len(ids)),
					Label: "确认启用",
					Run:   func() error { return a.SetMappingsEnabled(ids, true) },
				})
			}
		}
		if len(onIDs) > 0 {
			if ui.Button(c, "全部停用").FontSize(11).Disabled(busy).
				Tooltip("停用当前列表中启用的规则，改完后需应用配置").Clicked() {
				ids := append([]string(nil), onIDs...)
				a.Ask(&appt.Confirm{
					Title: "全部停用",
					Body:  fmt.Sprintf("将停用 %s 中目前启用的 %d 条规则。对外转发会在应用后失效。", scope, len(ids)),
					Label: "确认停用", Destructive: true,
					Run: func() error { return a.SetMappingsEnabled(ids, false) },
				})
			}
		}
	})
}

// filterEmpty is the card shown when search/filter matches nothing.
func (v *View) filterEmpty(c *ui.Context, t Tokens) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Box(c).Width(420).Padding(28, 24).Radius(RadiusPanel).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Column(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "没有匹配的规则").FontSize(16).Bold().TextColor(t.FG)
				hint := "试试别的关键词，或清掉筛选条件。"
				if v.query != "" {
					hint = fmt.Sprintf("没有名称、域名或端口匹配「%s」的规则。", v.query)
				}
				ui.Text(c, hint).FontSize(13).TextColor(t.Muted)
				ui.Row(c).Gap(8).Children(func() {
					if ui.PrimaryButton(c, "清除筛选").Clicked() {
						v.query = ""
						v.filter = 0
					}
					if v.filter != 0 {
						if ui.Button(c, "显示全部").Clicked() {
							v.filter = 0
						}
					}
				})
			})
		})
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
