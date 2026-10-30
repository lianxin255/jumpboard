package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/ui/icons"
)

// timeAt converts milliseconds to a time, kept here so the views need no
// time import of their own.
func timeAt(ms int64) time.Time { return time.UnixMilli(ms) }

// logsPage shows the operation log and the engine's access log.
func (v *View) logsPage(c *ui.Context, t Tokens) {
	a := v.App
	_, logs, _, _, _, _, busy, _ := a.Snapshot()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		// The window bar carries this page's name and the engine's state;
		// what it leaves behind — the pending strip, an apply failure —
		// is the page's one conditional block.
		v.pageNotices(c, t, busy)

		// The page's first toolbar: the tab and level pickers, and the
		// row's own actions, wrapping on a narrow window. The search gets
		// its own row below, the plan's placement — a field that narrows
		// the stream reads as its own control, not as another picker.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			// No fixed width: the control sizes to its labels. Width(220)
			// was tighter than the CJK labels and clipped them.
			ui.Segmented(c, &v.logTab, "操作日志", "访问日志").Label("日志类型")
			if v.logTab == 0 {
				ui.Segmented(c, &v.logLevel, "全部", "错误", "警告", "信息").Label("级别")
			}
			ui.Spacer(c)
			if v.logTab == 0 {
				if ui.Button(c, "导出").Disabled(busy).
					Tooltip("把操作日志全量导出为文本文件").Clicked() {
					v.exportAppLogs()
				}
				if DangerQuietButton(c, t, "清空操作日志").Disabled(busy).Clicked() {
					a.Ask(&appt.Confirm{
						Title: "清空操作日志", Body: "所有操作记录都会被删除。",
						Label: "清空", Destructive: true,
						Run: a.ClearAppLogs,
					})
				}
			} else {
				if CompactSwitch(c, boolPtr(a.NginxLogRead())).Label("读取访问日志").Changed() {
					a.SetNginxLogRead(!a.NginxLogRead())
				}
				// The interpretation is offered only when there is a log to
				// read: a button that would answer "nothing to interpret"
				// is worse than none.
				if a.AIReady() && a.NginxLogRead() {
					if ui.Button(c, "解读访问日志").Disabled(busy).
						Tooltip("把当前 access.log 的尾部交给模型总结").Clicked() {
						a.RunBackground(a.InterpretAccessLog)
					}
				}
				// The picker reads a file the app itself did not write — an
				// external nginx's error.log, a hand-written conf — and its
				// answer lands on the card below, beside the access log's
				// own interpretation. It asks only for AI: the file it reads
				// is not the engine's log, so the read switch is not its gate.
				if a.AIReady() {
					if ui.Button(c, "解读文件…").Disabled(busy).
						Tooltip("选一份 .conf / .log / .txt 交给模型解读，比如外部 nginx 的 error.log").Clicked() {
						v.interpretFilePicker()
					}
				}
				if ui.Button(c, "导出").Disabled(busy).
					Tooltip("把 access.log 全量导出").Clicked() {
					v.exportAccessLog()
				}
				if DangerQuietButton(c, t, "清空访问日志").Disabled(busy).Clicked() {
					a.Ask(&appt.Confirm{
						Title: "清空访问日志", Body: "引擎写的 access.log 会被清空。",
						Label: "清空", Destructive: true,
						Run: a.ClearNginxLogs,
					})
				}
			}
		})

		// The search: its own row, under the pickers it narrows.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.SearchField(c, &v.logQuery).Label("搜索日志").Width(280)
		})

		if v.logTab == 1 {
			v.interpretSection(c, t, "logs")
		}

		panelShell(c, t).Children(func() {
			if v.logTab == 0 {
				v.appLogList(c, t, logs)
			} else {
				v.nginxLogList(c, t)
			}
		})
	})
}

// boolPtr returns a pointer to a copy, for a switch that only reports its
// change.
func boolPtr(b bool) *bool { return &b }

// appLogList draws the operation log.
func (v *View) appLogList(c *ui.Context, t Tokens, logs []model.LogEntry) {
	want := ""
	switch v.logLevel {
	case 1:
		want = "error"
	case 2:
		want = "warn"
	case 3:
		want = "info"
	}
	q := strings.ToLower(strings.TrimSpace(v.logQuery))

	shown := make([]model.LogEntry, 0, len(logs))
	for _, e := range logs {
		if want != "" && string(e.Level) != want {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Message), q) {
			continue
		}
		shown = append(shown, e)
	}

	if len(shown) == 0 {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, "没有日志").FontSize(NoteSize).TextColor(t.Muted)
		})
		return
	}

	// The state lives on the view, not in this frame: a fresh ListState
	// each frame forgets the scroll position, so a long log could never
	// be read past its first page.
	ls := &v.logList
	ls.Key = func(i int) any { return shown[i].ID }
	ui.List(c, ls, len(shown), func(i int) {
		e := shown[i]
		// The level is a chip with its color as a tint, not just colored
		// text: the eye scans a wall of messages by the chip, and error
		// and warn should be findable without reading.
		color, tint := t.Muted, t.Surface2
		switch e.Level {
		case model.LogError:
			color, tint = t.Danger, t.Danger.Alpha(0.14)
		case model.LogWarn:
			color, tint = t.Warning, t.Warning.Alpha(0.14)
		}
		ui.Row(c).Gap(10).Padding(8, 14).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).Children(func() {
			ui.Text(c, formatClock(e.Timestamp)).FontSize(CaptionSize).TextColor(t.Muted).Width(70)
			ui.Box(c).Width(52).Radius(RadiusBadge).Background(tint).Padding(2, 0).Children(func() {
				ui.Text(c, string(e.Level)).FontSize(CaptionSize).Bold().TextColor(color)
			})
			ui.Text(c, e.Message).FontSize(NoteSize).TextColor(t.FG).SingleLine().Grow(1)
		})
	}).Grow(1)
}

// nginxLogList draws the engine's access log.
func (v *View) nginxLogList(c *ui.Context, t Tokens) {
	a := v.App
	if !a.NginxLogRead() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, "访问日志读取已关闭").FontSize(NoteSize).TextColor(t.Muted)
		})
		return
	}
	// Reading the files is cheap and bounded, so it happens in the frame,
	// once every two seconds rather than on every frame.
	now := time.Now().UnixMilli()
	if now-v.nginxLoadedAt > 2000 {
		v.nginxAccess, v.nginxErrors = a.NginxLogs(400)
		v.nginxLoadedAt = now
		c.After(2 * time.Second)
	}

	q := strings.ToLower(strings.TrimSpace(v.logQuery))
	access, errs := v.nginxAccess, v.nginxErrors
	if q != "" {
		access = filterLines(access, q)
		errs = filterLines(errs, q)
	}

	ui.Column(c).Fill().Gap(0).Children(func() {
		if len(errs) > 0 {
			ui.Text(c, "错误").FontSize(CaptionSize).Bold().TextColor(t.Danger).Padding(6, 14)
			ui.List(c, nil, len(errs), func(i int) {
				ui.Text(c, errs[i]).Font(FontMono).FontSize(CaptionSize).TextColor(t.Danger).
					Padding(2, 14).SingleLine()
			}).MaxHeight(200)
		}
		ui.Text(c, "访问记录").FontSize(CaptionSize).Bold().TextColor(t.Muted).Padding(6, 14)
		if len(access) == 0 {
			ui.Text(c, "还没有访问记录").FontSize(NoteSize).TextColor(t.Muted).Padding(8, 14)
			return
		}
		ui.List(c, nil, len(access), func(i int) {
			ui.Text(c, access[i]).Font(FontMono).FontSize(CaptionSize).TextColor(t.FG2).
				Padding(2, 14).SingleLine()
		}).Grow(1)
	}).Grow(1)
}

// filterLines keeps the lines holding a substring.
func filterLines(lines []string, q string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), q) {
			out = append(out, l)
		}
	}
	return out
}

// formatClock renders a timestamp as a wall clock time.
func formatClock(ms int64) string {
	if ms == 0 {
		return "—"
	}
	return timeAt(ms).Format("15:04:05")
}

// historyPage holds the saved configurations as a master-detail frame:
// the snapshots on the left, the chosen one's content and diff summary on
// the right. Choosing by click replaces the old 查看 dialog hop — the
// reading matter sits beside the list it is about — while the diff
// dialog keeps the deep compare and 回滚 keeps its confirmation.
func (v *View) historyPage(c *ui.Context, t Tokens) {
	a := v.App
	_, _, snaps, _, _, _, busy, _ := a.Snapshot()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		// The window bar carries the page's name and the engine's state;
		// the pending strip below is the same one the mappings page
		// shows — one place 未应用 is said.
		v.pageNotices(c, t, busy)

		if len(snaps) == 0 {
			v.emptyState(c, t, "还没有快照", "成功应用一次配置后，这里会出现第一条记录。")
			return
		}

		// The page's quiet caption and its one export, above the frame:
		// the plan's placement, so the frame itself stays the reading.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "每次成功应用自动保存，可回看差异或回滚").
				FontSize(CaptionSize).TextColor(t.Muted)
			ui.Spacer(c)
			if ui.Button(c, "导出配置").FontSize(CaptionSize).
				Tooltip("导出当前生效的 nginx.conf").Clicked() {
				v.exportNginx()
			}
		})

		// The chosen snapshot rides the view's state, like the settings
		// group: the first snapshot is the opening choice, so the page
		// lands reading instead of asking.
		if v.historyChosenID == "" {
			v.historyChosenID = snaps[0].ID
		}
		chosenIdx := -1
		for i, sn := range snaps {
			if sn.ID == v.historyChosenID {
				chosenIdx = i
				break
			}
		}
		if chosenIdx < 0 {
			chosenIdx = 0
			v.historyChosenID = snaps[0].ID
		}
		chosen := snaps[chosenIdx]

		ui.Row(c).Fill().AlignItems(ui.Start).Gap(16).Children(func() {
			// The list: one panel of snapshots, the chosen one named by
			// the accent's wash.
			panelShell(c, t).Width(300).Shrink(0).Children(func() {
				ls := &v.historyList
				ls.Key = func(i int) any { return snaps[i].ID }
				ui.List(c, ls, len(snaps), func(i int) {
					sn := snaps[i]
					on := sn.ID == v.historyChosenID
					// The row carries its snapshot's id as the label: a test
					// — and a screen reader — reaches one snapshot rather
					// than the first.
					row := ui.Column(c).Gap(2).FillWidth().Padding(10, 14).
						BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).
						Label("快照 " + sn.ID)
					if on {
						row.Background(t.AccentSoft)
					}
						row.Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								// The row itself is the click: choosing here is
								// reading, so the whole row answers.
								ui.Text(c, formatTime(sn.CreatedAt)).
									FontSize(BodySize).FontWeight(func() int {
									if on {
										return 500
									}
									return 400
								}()).TextColor(t.FG).Grow(1)
							})
							// The row's second line: why the snapshot was taken
							// and how big it is — the plan's summary line, which
							// says more than a bare reason.
							ui.Textf(c, "%s · %d 行", sn.Reason, strings.Count(sn.Content, "\n")+1).
								FontSize(CaptionSize).TextColor(t.Muted).SingleLine()
							if on {
								ui.Text(c, "当前查看").FontSize(CaptionSize).TextColor(t.Accent)
							}
						})
					if row.Clicked() {
						v.historyChosenID = sn.ID
						a.Change()
					}
				}).Grow(1)
			})

			// The reading: the chosen snapshot's head and its diff against
			// the running configuration, in the panel the list leaves.
			panelShell(c, t).Grow(1).Children(func() {
				ui.Column(c).Fill().Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(12, 16).
						BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).Children(func() {
						ui.Text(c, "快照内容").FontSize(BodySize).Bold().TextColor(t.FG)
						ui.Text(c, formatTime(chosen.CreatedAt)).FontSize(CaptionSize).TextColor(t.Muted)
						ui.Spacer(c)
						// The 查看 dialog's deep compare stays one click
						// away; the label keeps the words the tests know.
						if ui.Button(c, "查看差异").FontSize(CaptionSize).Clicked() {
							s := chosen
							a.SetSnapshotView(&s)
						}
						// The label names the row: every row says 回滚, and a
						// test has to reach one snapshot rather than the first.
						if DangerQuietButton(c, t, "回滚").FontSize(CaptionSize).Label("回滚 " + chosen.ID).Disabled(busy).Clicked() {
							s := chosen
							a.Ask(&appt.Confirm{
								Title: "回滚配置",
								Body:  fmt.Sprintf("把 %s 的配置写回磁盘并校验。", formatTime(chosen.CreatedAt)),
								Label: "回滚",
								Run:   func() error { return a.RollbackSnapshot(s) },
							})
						}
					})
					ui.Scroll(c).Grow(1).Children(func() {
						ui.Text(c, chosen.Content).Font(FontMono).FontSize(CaptionSize).
							TextColor(t.FG2).Wrap().Selectable().Padding(12, 16)
					})
				})
			})
		})
	})
}

// hotkeyDraft is what the shortcut card holds between frames: the
// accelerator typed into its field, and why the last save was refused.
type hotkeyDraft struct {
	text string
	err  string
}

// hotkeyCards holds every app's shortcut-card draft.
//
// The natural home is a View field, like v.whitelistIP, but the View
// struct is declared in view.go and this slice leaves that file alone.
// Keyed by the app, a fresh view — the tests build one per app — starts
// from a clean card, exactly as a View field would. Frames run on one
// thread, so the map needs no lock.
var hotkeyCards = map[*appt.App]*hotkeyDraft{}

// hotkeyDraftFor returns the shortcut card's draft for the app.
func hotkeyDraftFor(a *appt.App) *hotkeyDraft {
	d, ok := hotkeyCards[a]
	if !ok {
		d = &hotkeyDraft{}
		hotkeyCards[a] = d
	}
	return d
}

// settingsCardMin is the least width one of the settings page's cards
// takes. The master-detail layout gives the content column the window
// minus the nav, so the min mostly guards the narrow-window stack.
const settingsCardMin = float32(360)

// settingsSection is one entry of the settings nav: an id the view state
// holds, the label the nav and the content header say, and the icon that
// marks the row.
type settingsSection struct {
	id    int
	label string
	icon  *ui.SVG
}

// settingsSections lists the page's groups in the order a user needs
// them: the AI service first — the chat page's 去设置 lands on it — then
// the desktop's looks and the app's own facts, who may connect, the data
// tools, the scenes, the keys, the update, and the danger zone last,
// apart from the cards it outclasses in consequence.
func settingsSections() []settingsSection {
	return []settingsSection{
		{0, "AI 配置", icons.Chat},
		{1, "通用与外观", icons.Settings},
		{2, "访问控制", icons.Check},
		{3, "备份与数据", icons.History},
		{4, "场景管理", icons.Relay},
		{5, "快捷键", icons.Terminal},
		{6, "更新", icons.Activity},
		{7, "危险操作", icons.Alert},
	}
}

// settingsPage holds every preference behind a master-detail frame: a
// quiet nav of groups on the left, the one chosen group's cards on the
// right. One group at a time replaces the old waterfall of paired cards,
// where the AI configuration's height decided where 外观 began; now each
// group reads as its own short page, and a group below the fold of the
// old scroll is one click away instead.
func (v *View) settingsPage(c *ui.Context, t Tokens) {
	a := v.App
	_, _, _, wl, _, _, busy, _ := a.Snapshot()

	if v.settingsSection < 0 || v.settingsSection > 7 {
		v.settingsSection = 0
	}

	// The page's own padding: the master-detail frame sits inside it, so
	// the content column ends PagePadX short of the window's edge.
	ui.Row(c).Fill().Padding(PagePadY, PagePadX).AlignItems(ui.Start).Gap(24).Children(func() {
		// The nav: the page's own list, sticky in the frame's left. The
		// chosen group carries the accent's wash; the rest stay quiet.
		ui.Column(c).Width(168).Shrink(0).Gap(2).Children(func() {
			for _, s := range settingsSections() {
				s := s
				on := s.id == v.settingsSection
				item := ui.Text(c, s.label).FontSize(BodySize).SingleLine().
					Label("设置分组 " + s.label).Padding(6, 10).Radius(RadiusButton)
				if on {
					item.FontWeight(500).TextColor(t.Accent).
						Background(t.AccentSoft).Border(1, t.AccentLine)
				} else {
					item.TextColor(t.FG2)
				}
				if item.Clicked() {
					v.settingsSection = s.id
					a.Change()
				}
			}
		})

		// The content: the chosen group's title and one line of what it is
		// for — the plan's master-detail head — then its cards in one
		// scroll, each full width of the column.
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(16).Children(func() {
				v.settingsGroupHeading(c, t)
				switch v.settingsSection {
				case 0:
					v.aiCard(c, t)
				case 1:
					v.appearanceCard(c, t)
					v.generalCard(c, t)
				case 2:
					v.accessCard(c, t, wl, busy)
				case 3:
					v.backupCard(c, t)
				case 4:
					v.workspaceCard(c, t, busy)
				case 5:
					v.hotkeyCard(c, t)
				case 6:
					v.updateCard(c, t)
				case 7:
					v.dangerCard(c, t, busy)
				}
			})
		})
	})
}

// settingsGroupHeading is the content column's head: the chosen group's
// name as a page-level title and one line saying what the group is for.
// The cards under it carry their own section titles; this says which group
// the reader is in, the one thing a master-detail page must never leave
// implicit.
func (v *View) settingsGroupHeading(c *ui.Context, t Tokens) {
	var title, note string
	switch v.settingsSection {
	case 0:
		title = "AI 配置"
		note = "配置兼容 OpenAI 的模型服务。只有主动使用 AI 功能时才发送对应内容。"
	case 1:
		title = "通用与外观"
		note = "应用名称、外观与显示偏好。"
	case 2:
		title = "访问控制"
		note = "限制可访问所有代理的客户端 IP。"
	case 3:
		title = "备份与数据"
		note = "备份、导入和查看实际生成的配置。"
	case 4:
		title = "场景管理"
		note = "每个场景一套独立映射：公司、家里各存各的规则。"
	case 5:
		title = "快捷键"
		note = "设置全局显示/隐藏窗口的快捷键。"
	case 6:
		title = "更新"
		note = "检查新版本，更新前保留当前配置。"
	case 7:
		title = "危险操作"
		note = "会删除现有数据的操作集中在这里。"
	}
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, title).FontSize(HeadSize).Bold().TextColor(t.FG)
		ui.Text(c, note).FontSize(CaptionSize).TextColor(t.Muted).Wrap()
	})
}

// dangerCard is the settings page's 危险操作 card: the wipe, alone, in
// the danger's own outline — a card apart from the tools it outclasses
// in consequence.
func (v *View) dangerCard(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	zoneFill, zoneLine := statusOutline(t.Danger)
	ui.Column(c).FillWidth().Gap(4).Padding(16).Radius(RadiusPanel).
		Background(zoneFill).Border(1, zoneLine).Children(func() {
		ui.Text(c, "危险操作").FontSize(BodySize).Bold().TextColor(t.Danger)
		ui.Text(c, "清空所有数据会删除映射、日志、快照和白名单，无法撤销。").
			FontSize(CaptionSize).TextColor(t.FG2).Wrap()
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if DangerButton(c, t, "清空所有数据").Disabled(busy).Clicked() {
				a.Ask(&appt.Confirm{
					Title: "清空所有数据",
					Body:  "映射、日志、快照和白名单都会被删除，无法撤销。",
					Label: "全部删除", Destructive: true,
					Run: a.Wipe,
				})
			}
		})
	})
}

// appearanceCard is the settings page's 外观与显示 card: the theme and
// the traffic display — how the app looks, and only that.
func (v *View) appearanceCard(c *ui.Context, t Tokens) {
	a := v.App
	themeIdx := 0
	switch a.Theme() {
	case appt.ThemeLight:
		themeIdx = 1
	case appt.ThemeDark:
		themeIdx = 2
	}

	card(c, t, "外观与显示", func() {
		ui.Field(c, "主题", func() {
			if ui.Segmented(c, &themeIdx, "跟随系统", "亮色", "暗色").Label("主题").Changed() {
				switch themeIdx {
				case 1:
					a.SetTheme(appt.ThemeLight)
				case 2:
					a.SetTheme(appt.ThemeDark)
				default:
					a.SetTheme(appt.ThemeSystem)
				}
			}
		})
		ui.Text(c, "亮色与暗色各有一套配色；跟随系统时随桌面的外观自动切换。").
			FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		// The traffic switch rides the same row as its words, the
		// whitelist switch's shape: a settings page whose cards
		// grow one line each would push the cards beside them
		// below the fold of the window's default size.
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "流量显示").FontSize(BodySize).TextColor(t.FG)
				ui.Text(c, "映射表与详情显示连接数和流量").FontSize(CaptionSize).TextColor(t.Muted)
			})
			on := a.ShowTraffic()
			if CompactSwitch(c, &on).Label("显示流量").
				Tooltip("关闭后映射表和详情不再显示连接数与流量，转发本身不受影响").Changed() {
				a.SetShowTraffic(on)
			}
		})
	})
}

// generalCard is the settings page's 通用 card: the gateway's own name
// and the login item — the app-level facts that are neither its looks
// nor its rules.
func (v *View) generalCard(c *ui.Context, t Tokens) {
	a := v.App
	if !v.nameLoaded {
		v.settingsName = a.HostName()
		v.nameLoaded = true
	}
	card(c, t, "通用", func() {
		ui.Field(c, "网关名称", func() {
			if ui.TextInput(c, &v.settingsName).Submitted() {
				a.SetHostName(v.settingsName)
			}
		})
		ui.Field(c, "开机自启", func() {
			on := a.OpenAtLogin()
			if CompactSwitch(c, &on).Label("开机自启动").Changed() {
				if err := a.SetOpenAtLogin(on); err != nil {
					a.Fail("设置开机自启失败：%v", err)
				}
			}
		})
		ui.Text(c, "登录系统后自动启动 JumpBoard；需要安装打包后的应用。").
			FontSize(CaptionSize).TextColor(t.Muted).Wrap()
	})
}

// accessCard is the settings page's 访问控制 card: the whitelist's
// master switch, its rows, and the field that adds to it.
func (v *View) accessCard(c *ui.Context, t Tokens, wl []model.WhitelistEntry, busy bool) {
	a := v.App
	card(c, t, "访问控制", func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "IP 白名单").FontSize(BodySize).TextColor(t.FG)
				ui.Text(c, "开启后只允许列表中的 IP 访问所有代理").FontSize(CaptionSize).TextColor(t.Muted)
			})
			// The switch's label names the action, not the heading
			// text beside it: two elements with one name would
			// make the click ambiguous.
			on := a.WhitelistOn()
			if CompactSwitch(c, &on).Label("启用白名单").Changed() {
				a.SetWhitelistOn(on)
			}
		})
		ui.Divider(c)
		if len(wl) == 0 {
			ui.Text(c, "还没有条目").FontSize(CaptionSize).TextColor(t.Muted)
		}
		for _, e := range wl {
			e := e
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				on := e.Enabled
				CompactSwitch(c, &on).Label("启用-" + e.IP).OnChange(func() {
					a.RunBackground(func() error { return a.ToggleWhitelist(e.ID, on) })
				})
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, e.IP).FontSize(NoteSize).TextColor(t.FG).Font(FontMono)
					if e.Note != "" {
						ui.Text(c, e.Note).FontSize(CaptionSize).TextColor(t.Muted)
					}
				})
				if DangerQuietButton(c, t, "移除").FontSize(CaptionSize).Clicked() {
					a.RunBackground(func() error { return a.DeleteWhitelist(e.ID) })
				}
			})
		}
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &v.whitelistIP).Placeholder("192.168.1.0/24").Grow(1).Label("白名单地址")
			ui.TextInput(c, &v.whitelistNote).Placeholder("备注").Width(90)
			if ui.Button(c, "添加").Disabled(busy).Clicked() {
				ip, note := v.whitelistIP, v.whitelistNote
				v.whitelistIP, v.whitelistNote = "", ""
				a.RunBackground(func() error { return a.AddWhitelist(ip, note) })
			}
		})
		if a.WhitelistOn() && len(wl) == 0 {
			ui.Text(c, "白名单开着但没有任何 IP，应用配置会被拒绝。").
				FontSize(CaptionSize).TextColor(t.Warning)
		}
	})
}

// backupCard is the settings page's 备份与快照 card: the backup and
// export tools over the data directory they act on.
func (v *View) backupCard(c *ui.Context, t Tokens) {
	a := v.App
	card(c, t, "备份与快照", func() {
		ui.Text(c, "备份整份配置，或回到「历史」查看每次应用留下的快照。").
			FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if ui.Button(c, "导出备份").Clicked() {
				v.exportBackup()
			}
			if ui.Button(c, "导入备份").Clicked() {
				v.importBackup()
			}
			if ui.Button(c, "导入为新场景").
				Tooltip("把备份里的映射放进一个新建的场景，不动当前场景").Clicked() {
				v.importBackupAsWorkspace()
			}
			if ui.Button(c, "配置历史").Clicked() {
				a.SetPage(appt.PageHistory)
			}
		})
		ui.Text(c, "导出不含密码；导入会全量替换现有映射与白名单。").
			FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		ui.Divider(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "导出 nginx 配置").Clicked() {
				v.exportNginx()
			}
			ui.Text(c, "导出成标准 nginx.conf，可在真正的 nginx 上运行。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		})
		ui.Divider(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "查看当前配置").Clicked() {
				a.SetConfViewOpen(true)
			}
			ui.Text(c, "只读查看应用实际生成的 nginx.conf。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		})
		ui.Divider(c)
		ui.Text(c, "数据目录："+a.Paths.DataDir).FontSize(CaptionSize).
			TextColor(t.Muted).Wrap().Selectable()
		if a.Paths.Fallback {
			ui.Text(c, "程序目录不可写，数据改存在用户目录。").
				FontSize(CaptionSize).TextColor(t.Warning)
		}
	})
}

// hotkeyCard is the settings page's 快捷键 card: the global binding, the
// field that changes it, and the in-app shortcuts that never change.
func (v *View) hotkeyCard(c *ui.Context, t Tokens) {
	a := v.App
	card(c, t, "快捷键", func() {
		// The global binding is a setting; the in-app
		// shortcuts below it are not — they are wired in the
		// view and stay as they are. The current accelerator
		// is read from the store each frame: frames are drawn
		// on demand, and the card must say what is stored, not
		// what it remembers.
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			kbdKey(c, t, a.Hotkey())
			ui.Text(c, "全局显示/隐藏主窗口").FontSize(NoteSize).TextColor(t.FG2)
		})
		// A typed accelerator, not a key recorder: the string
		// is validated by trying to register it, which is what
		// the save reports.
		d := hotkeyDraftFor(a)
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &d.text).Placeholder("CmdOrCtrl+Shift+Alt+K").Grow(1).
				Label("新快捷键").
				Tooltip("输入组合键文本（本版不录制按键）：修饰键写 CmdOrCtrl、Ctrl、Alt 或 Shift，用 + 连接，末尾是主键（字母、数字或 F1-F24 等功能键），例如 CmdOrCtrl+Shift+Alt+K。保存时尝试注册，组合被占用或格式不对会提示。")
			if ui.Button(c, "保存快捷键").Clicked() {
				if err := a.SetHotkey(d.text); err != nil {
					d.err = err.Error()
				} else {
					d.text, d.err = "", ""
				}
			}
		})
		if d.err != "" {
			ui.Text(c, d.err).FontSize(CaptionSize).TextColor(t.Danger).Wrap()
		}
		ui.Divider(c)
		rows := [][2]string{
			{"⌘N", "新建映射"},
			{"⌘F", "聚焦搜索"},
			{"⌘R", "应用配置"},
			{"Esc", "关闭当前对话框"},
		}
		for _, r := range rows {
			r := r
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				kbdKey(c, t, r[0])
				ui.Text(c, r[1]).FontSize(NoteSize).TextColor(t.FG2)
			})
		}
		ui.Text(c, "Windows 上 ⌘ 对应 Ctrl。").FontSize(CaptionSize).TextColor(t.Muted)
	})
}

// updateCard is the settings page's 更新 card: the version, the check,
// and what a found version offers.
func (v *View) updateCard(c *ui.Context, t Tokens) {
	a := v.App
	card(c, t, "更新", func() {
		ui.Text(c, "当前版本 "+a.AppVersion).FontSize(CaptionSize).TextColor(t.Muted)
		if !a.UpdatesEnabled() {
			ui.Text(c, "开发构建不支持自动更新，新版本请从 GitHub Releases 下载。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
			return
		}
		if up := a.UpdateAvail(); up != nil {
			ui.Text(c, "发现新版本 "+up.Version).FontSize(BodySize).Bold().TextColor(t.FG)
			markdown.View(c, up.Notes, &markdown.Options{})
			if a.UpdateBusy() {
				if p := a.UpdateProgress(); p >= 0 {
					ui.Text(c, fmt.Sprintf("正在下载更新…%d%%", int(p*100))).
						FontSize(CaptionSize).TextColor(t.Muted)
				} else {
					ui.Text(c, "正在下载更新…").FontSize(CaptionSize).TextColor(t.Muted)
				}
			}
			if ui.PrimaryButton(c, "安装并重启").Disabled(a.UpdateBusy()).Clicked() {
				a.InstallFoundUpdate()
			}
			ui.Text(c, "下载校验通过后自动重启到新版本。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
			return
		}
		if ui.Button(c, "检查更新").Disabled(a.UpdateBusy()).Clicked() {
			a.CheckForUpdates()
		}
		if a.UpdateBusy() {
			ui.Text(c, "正在检查…").FontSize(CaptionSize).TextColor(t.Muted)
		} else {
			ui.Text(c, "更新包经签名校验，只有检查后才会下载。").
				FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		}
	})
}

// workspaceCard is the settings page's scene manager: the list with
// switch, rename and delete, and the field that adds a scene. The
// one-click switch itself lives on the mappings page; this is where a
// scene is born, renamed and retired. It pairs with the data tools,
// which feed it.
func (v *View) workspaceCard(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	list, active := a.WorkspacesState()
	card(c, t, "场景", func() {
		ui.Text(c, "每个场景一套独立映射：公司、家里各存各的规则，切换后立即按新场景重新应用。").
			FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		for _, w := range list {
			w := w
			on := w.ID == active
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if v.wsRenameID == w.ID {
					// The rename is the row itself turned into a field,
					// prefilled with the name it would change.
					ui.TextInput(c, &v.wsRenameName).Grow(1).Label("重命名 " + w.Name)
					if ui.Button(c, "保存").FontSize(CaptionSize).Disabled(busy).Clicked() {
						id, name := w.ID, v.wsRenameName
						v.wsRenameID = ""
						a.RunBackground(func() error { return a.RenameWorkspaceAction(id, name) })
					}
					if ui.Button(c, "取消").FontSize(CaptionSize).Clicked() {
						v.wsRenameID = ""
					}
					return
				}
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, w.Name).FontSize(NoteSize).TextColor(t.FG)
					if on {
						ui.Text(c, "当前场景").FontSize(CaptionSize).TextColor(t.Accent)
					}
				})
				if !on {
					if ui.Button(c, "切换").FontSize(CaptionSize).Label("切换 " + w.Name).
						Disabled(busy).Tooltip("切换到这个场景，引擎立即按它重新应用").Clicked() {
						a.RequestSwitchWorkspace(w.ID)
					}
				}
				if ui.Button(c, "重命名").FontSize(CaptionSize).Label("重命名 " + w.Name).Disabled(busy).Clicked() {
					v.wsRenameID, v.wsRenameName = w.ID, w.Name
				}
				// The last scene and the active one have no delete: the
				// buttons stay rather than flicker with the list.
				del := DangerQuietButton(c, t, "删除").FontSize(CaptionSize).Label("删除场景 " + w.Name).
					Disabled(busy || on || len(list) <= 1).Tooltip("删除场景及其全部映射，删除前会确认")
				if del.Clicked() {
					name := w.Name
					a.Ask(&appt.Confirm{
						Title: "删除场景",
						Body:  fmt.Sprintf("场景「%s」和它的全部映射都会被删除，无法撤销。", name),
						Label: "删除场景", Destructive: true,
						Run: func() error { return a.DeleteWorkspaceAction(w.ID) },
					})
				}
			})
		}
		ui.Divider(c)
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &v.wsNewName).Placeholder("公司 / 家里 / 项目A").Grow(1).Label("新场景名称")
			if ui.Button(c, "新建场景").Disabled(busy).Clicked() {
				name := v.wsNewName
				v.wsNewName = ""
				a.RunBackground(func() error { return a.CreateWorkspaceAction(name) })
			}
		})
		ui.Text(c, "新场景从空白开始，不会影响正在转发的规则。").FontSize(CaptionSize).TextColor(t.Muted).Wrap()
	})
}

// card draws one of the settings page's paired cards: a titled panel
// whose Grow and MinWidth let a pair share a line while the page has room
// for the two of them, and take a full-width line of their own — the pair
// stacked — when it does not. Grow then splits the line with the partner,
// so the two are always equal in width.
func card(c *ui.Context, t Tokens, title string, fn func()) {
	ui.Column(c).Grow(1).MinWidth(settingsCardMin).Gap(10).Padding(16).Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).FontSize(BodySize).Bold().TextColor(t.FG)
		fn()
	})
}

// wideCard is the settings page's full-width card: the AI configuration,
// which owns a row of its own and does not pair.
func wideCard(c *ui.Context, t Tokens, title string, fn func()) {
	ui.Column(c).FillWidth().MinWidth(settingsCardMin).Gap(10).Padding(16).Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).FontSize(BodySize).Bold().TextColor(t.FG)
		fn()
	})
}
