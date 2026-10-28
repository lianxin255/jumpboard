package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
)

// timeAt converts milliseconds to a time, kept here so the views need no
// time import of their own.
func timeAt(ms int64) time.Time { return time.UnixMilli(ms) }

// logsPage shows the operation log and the engine's access log.
func (v *View) logsPage(c *ui.Context, t Tokens) {
	a := v.App
	_, logs, _, _, _, _, busy, _ := a.Snapshot()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		v.actionRow(c, t)

		ui.Row(c).AlignItems(ui.End).Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, "日志").FontSize(TitleSize).Bold().TextColor(t.FG)
				ui.Text(c, "操作记录与内置引擎的访问日志").FontSize(SubtitleSz).TextColor(t.Muted)
			})
			ui.Spacer(c)
			// No fixed width: the control sizes to its labels. Width(220)
			// was tighter than the CJK labels and clipped them.
			ui.Segmented(c, &v.logTab, "操作日志", "访问日志").Label("日志类型")
			if v.logTab == 0 {
				ui.Segmented(c, &v.logLevel, "全部", "错误", "警告", "信息").Label("级别")
			}
		})

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &v.logQuery).Label("搜索日志").Width(280)
			ui.Spacer(c)
			if v.logTab == 0 {
				if ui.Button(c, "导出").Disabled(busy).
					Tooltip("把操作日志全量导出为文本文件").Clicked() {
					v.exportAppLogs()
				}
				if ui.Button(c, "清空操作日志").Disabled(busy).Clicked() {
					a.Ask(&appt.Confirm{
						Title: "清空操作日志", Body: "所有操作记录都会被删除。",
						Label: "清空", Destructive: true,
						Run: a.ClearAppLogs,
					})
				}
			} else {
				if ui.Switch(c, boolPtr(a.NginxLogRead())).Label("读取访问日志").Changed() {
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
				if ui.Button(c, "导出").Disabled(busy).
					Tooltip("把 access.log 全量导出").Clicked() {
					v.exportAccessLog()
				}
				if ui.Button(c, "清空访问日志").Disabled(busy).Clicked() {
					a.Ask(&appt.Confirm{
						Title: "清空访问日志", Body: "引擎写的 access.log 会被清空。",
						Label: "清空", Destructive: true,
						Run: a.ClearNginxLogs,
					})
				}
			}
		})

		if v.logTab == 1 {
			v.interpretSection(c, t, "logs")
		}

		ui.Column(c).Fill().Radius(RadiusPanel).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
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
			ui.Text(c, "没有日志").FontSize(12).TextColor(t.Muted)
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
			ui.Text(c, formatClock(e.Timestamp)).FontSize(11).TextColor(t.Muted).Width(70)
			ui.Box(c).Width(52).Radius(RadiusBadge).Background(tint).Padding(2, 0).Children(func() {
				ui.Text(c, string(e.Level)).FontSize(10).Bold().TextColor(color)
			})
			ui.Text(c, e.Message).FontSize(12).TextColor(t.FG).SingleLine().Grow(1)
		})
	}).Grow(1)
}

// nginxLogList draws the engine's access log.
func (v *View) nginxLogList(c *ui.Context, t Tokens) {
	a := v.App
	if !a.NginxLogRead() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, "访问日志读取已关闭").FontSize(12).TextColor(t.Muted)
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
			ui.Text(c, "错误").FontSize(11).Bold().TextColor(t.Danger).Padding(6, 14)
			ui.List(c, nil, len(errs), func(i int) {
				ui.Text(c, errs[i]).Font("monospace").FontSize(11).TextColor(t.Danger).
					Padding(2, 14).SingleLine()
			}).MaxHeight(200)
		}
		ui.Text(c, "访问记录").FontSize(11).Bold().TextColor(t.Muted).Padding(6, 14)
		if len(access) == 0 {
			ui.Text(c, "还没有访问记录").FontSize(12).TextColor(t.Muted).Padding(8, 14)
			return
		}
		ui.List(c, nil, len(access), func(i int) {
			ui.Text(c, access[i]).Font("monospace").FontSize(11).TextColor(t.FG2).
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

// historyPage lists the saved configurations.
func (v *View) historyPage(c *ui.Context, t Tokens) {
	a := v.App
	_, _, snaps, _, _, dirty, busy, _ := a.Snapshot()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		v.actionRow(c, t)

		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "配置历史").FontSize(TitleSize).Bold().TextColor(t.FG)
			ui.Text(c, "每次成功应用都会存一份快照，最多保留 50 份。回滚会把当时的规则恢复回来并立即生效。").
				FontSize(SubtitleSz).TextColor(t.Muted).Wrap()
		})
		if dirty {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(8, 12).Radius(RadiusChip).
				Background(t.Warning.Alpha(0.12)).Border(1, t.Warning.Alpha(0.32)).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(t.Warning)
				ui.Text(c, "有改动还没应用；映射页的状态点会显示哪些规则还没生效。").
					FontSize(11).TextColor(t.Warning)
			})
		}

		if len(snaps) == 0 {
			v.emptyState(c, t, "还没有快照", "成功应用一次配置后，这里会出现第一条记录。")
			return
		}

		ui.Column(c).Fill().Radius(RadiusPanel).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
			ls := &v.historyList
			ls.Key = func(i int) any { return snaps[i].ID }
			ui.List(c, ls, len(snaps), func(i int) {
				sn := snaps[i]
				ui.Row(c).Gap(12).Padding(10, 14).AlignItems(ui.Center).
					BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).Children(func() {
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Textf(c, "%s · %d 行", formatTime(sn.CreatedAt), strings.Count(sn.Content, "\n")+1).
							FontSize(13).TextColor(t.FG)
						ui.Text(c, sn.Reason).FontSize(11).TextColor(t.Muted)
					})
					if ui.Button(c, "查看").FontSize(11).Clicked() {
						s := sn
						a.SetSnapshotView(&s)
					}
					// The label names the row: every row says 回滚, and a
					// test has to reach one snapshot rather than the first.
					if ui.Button(c, "回滚").FontSize(11).Label("回滚 " + sn.ID).Disabled(busy).Clicked() {
						s := sn
						a.Ask(&appt.Confirm{
							Title: "回滚配置",
							Body:  fmt.Sprintf("把 %s 的配置写回磁盘并校验。", formatTime(sn.CreatedAt)),
							Label: "回滚",
							Run:   func() error { return a.RollbackSnapshot(s) },
						})
					}
				})
			}).Grow(1)
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

// hotkeyCard returns the shortcut card's draft for the app.
func hotkeyCard(a *appt.App) *hotkeyDraft {
	d, ok := hotkeyCards[a]
	if !ok {
		d = &hotkeyDraft{}
		hotkeyCards[a] = d
	}
	return d
}

// settingsPage holds the appearance, the access control and the data tools.
func (v *View) settingsPage(c *ui.Context, t Tokens) {
	a := v.App
	_, _, _, wl, _, _, busy, _ := a.Snapshot()

	if !v.nameLoaded {
		v.settingsName = a.HostName()
		v.nameLoaded = true
	}

	themeIdx := 0
	switch a.Theme() {
	case appt.ThemeLight:
		themeIdx = 1
	case appt.ThemeDark:
		themeIdx = 2
	}

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(PageGap).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "设置").FontSize(TitleSize).Bold().TextColor(t.FG)
			ui.Text(c, "AI 配置、外观、访问控制与数据管理").FontSize(SubtitleSz).TextColor(t.Muted)
		})

		ui.Scroll(c).Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Wrap().Children(func() {
				// The AI card leads the page: the chat page's 去设置 lands
				// here for it, and a card below the fold would need the
				// user to scroll to the one thing they came for.
				v.aiCard(c, t)

				v.card(c, t, "外观", func() {
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
						FontSize(11).TextColor(t.Muted).Wrap()
					ui.Field(c, "网关名称", func() {
						if ui.TextInput(c, &v.settingsName).Submitted() {
							a.SetHostName(v.settingsName)
						}
					})
					ui.Field(c, "开机自启", func() {
						on := a.OpenAtLogin()
						if ui.Switch(c, &on).Label("开机自启动").Changed() {
							_ = a.SetOpenAtLogin(on)
						}
					})
					ui.Text(c, "登录系统后自动启动 JumpBoard；需要安装打包后的应用。").
						FontSize(11).TextColor(t.Muted).Wrap()
					// The traffic switch rides the same row as its words, the
					// whitelist switch's shape: a settings page whose cards
					// grow one line each would push the cards beside them
					// below the fold of the window's default size.
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Gap(2).Grow(1).Children(func() {
							ui.Text(c, "流量显示").FontSize(13).TextColor(t.FG)
							ui.Text(c, "映射表与详情显示连接数和流量").FontSize(11).TextColor(t.Muted)
						})
						on := a.ShowTraffic()
						if ui.Switch(c, &on).Label("显示流量").
							Tooltip("关闭后映射表和详情不再显示连接数与流量，转发本身不受影响").Changed() {
							a.SetShowTraffic(on)
						}
					})
				})

				v.card(c, t, "访问控制", func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Gap(2).Grow(1).Children(func() {
							ui.Text(c, "IP 白名单").FontSize(13).TextColor(t.FG)
							ui.Text(c, "开启后只允许列表中的 IP 访问所有代理").FontSize(11).TextColor(t.Muted)
						})
						// The switch's label names the action, not the heading
						// text beside it: two elements with one name would
						// make the click ambiguous.
						on := a.WhitelistOn()
						if ui.Switch(c, &on).Label("启用白名单").Changed() {
							a.SetWhitelistOn(on)
						}
					})
					ui.Divider(c)
					if len(wl) == 0 {
						ui.Text(c, "还没有条目").FontSize(11).TextColor(t.Muted)
					}
					for _, e := range wl {
						e := e
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							on := e.Enabled
							ui.Switch(c, &on).Label("启用-" + e.IP).OnChange(func() {
								a.RunBackground(func() error { return a.ToggleWhitelist(e.ID, on) })
							})
							ui.Column(c).Gap(1).Grow(1).Children(func() {
								ui.Text(c, e.IP).FontSize(12).TextColor(t.FG).Font("monospace")
								if e.Note != "" {
									ui.Text(c, e.Note).FontSize(10).TextColor(t.Muted)
								}
							})
							if ui.Button(c, "移除").FontSize(11).Clicked() {
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
							FontSize(11).TextColor(t.Warning)
					}
				})

				v.card(c, t, "备份与快照", func() {
					ui.Text(c, "备份整份配置，或回到「历史」查看每次应用留下的快照。").
						FontSize(11).TextColor(t.Muted).Wrap()
					ui.Row(c).Gap(8).Children(func() {
						if ui.PrimaryButton(c, "导出备份").Clicked() {
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
						FontSize(11).TextColor(t.Muted).Wrap()
					ui.Divider(c)
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "导出 nginx 配置").Clicked() {
							v.exportNginx()
						}
						ui.Text(c, "导出成标准 nginx.conf，可在真正的 nginx 上运行。").
							FontSize(11).TextColor(t.Muted).Wrap()
					})
					ui.Divider(c)
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "查看当前配置").Clicked() {
							a.SetConfViewOpen(true)
						}
						ui.Text(c, "只读查看应用实际生成的 nginx.conf。").
							FontSize(11).TextColor(t.Muted).Wrap()
					})
					ui.Divider(c)
					ui.Text(c, "数据目录："+a.Paths.DataDir).FontSize(10).
						TextColor(t.Muted).Wrap().Selectable()
					if a.Paths.Fallback {
						ui.Text(c, "程序目录不可写，数据改存在用户目录。").
							FontSize(10).TextColor(t.Warning)
					}
				})

				v.card(c, t, "快捷键", func() {
					// The global binding is a setting; the in-app
					// shortcuts below it are not — they are wired in the
					// view and stay as they are. The current accelerator
					// is read from the store each frame: frames are drawn
					// on demand, and the card must say what is stored, not
					// what it remembers.
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Padding(2, 8).Radius(RadiusBadge).Background(t.Surface3).
							Border(1, t.Border).Children(func() {
							ui.Text(c, a.Hotkey()).FontSize(11).Bold().TextColor(t.FG).Font("monospace")
						})
						ui.Text(c, "全局显示/隐藏主窗口").FontSize(12).TextColor(t.FG2)
					})
					// A typed accelerator, not a key recorder: the string
					// is validated by trying to register it, which is what
					// the save reports.
					d := hotkeyCard(a)
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
						ui.Text(c, d.err).FontSize(11).TextColor(t.Danger).Wrap()
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
							ui.Box(c).Padding(2, 8).Radius(RadiusBadge).Background(t.Surface3).
								Border(1, t.Border).Children(func() {
								ui.Text(c, r[0]).FontSize(11).Bold().TextColor(t.FG).Font("monospace")
							})
							ui.Text(c, r[1]).FontSize(12).TextColor(t.FG2)
						})
					}
					ui.Text(c, "Windows 上 ⌘ 对应 Ctrl。").FontSize(11).TextColor(t.Muted)
				})

				v.card(c, t, "更新", func() {
					ui.Text(c, "当前版本 "+a.AppVersion).FontSize(11).TextColor(t.Muted)
					if !a.UpdatesEnabled() {
						ui.Text(c, "开发构建不支持自动更新，新版本请从 GitHub Releases 下载。").
							FontSize(11).TextColor(t.Muted).Wrap()
						return
					}
					if up := a.UpdateAvail(); up != nil {
						ui.Text(c, "发现新版本 "+up.Version).FontSize(13).Bold().TextColor(t.FG)
						markdown.View(c, up.Notes, &markdown.Options{})
						if a.UpdateBusy() {
							if p := a.UpdateProgress(); p >= 0 {
								ui.Text(c, fmt.Sprintf("正在下载更新…%d%%", int(p*100))).
									FontSize(11).TextColor(t.Muted)
							} else {
								ui.Text(c, "正在下载更新…").FontSize(11).TextColor(t.Muted)
							}
						}
						if ui.PrimaryButton(c, "安装并重启").Disabled(a.UpdateBusy()).Clicked() {
							a.InstallFoundUpdate()
						}
						ui.Text(c, "下载校验通过后自动重启到新版本。").
							FontSize(11).TextColor(t.Muted).Wrap()
						return
					}
					if ui.Button(c, "检查更新").Disabled(a.UpdateBusy()).Clicked() {
						a.CheckForUpdates()
					}
					if a.UpdateBusy() {
						ui.Text(c, "正在检查…").FontSize(11).TextColor(t.Muted)
					} else {
						ui.Text(c, "更新包经签名校验，只有检查后才会下载。").
							FontSize(11).TextColor(t.Muted).Wrap()
					}
				})
			})

			// The scene card comes last: a scene is renamed and retired
			// rarely enough that the tools around it keep the first rows,
			// and the page scrolls. The one-click switch is on the
			// mappings page, where it belongs.
			v.workspaceCard(c, t, busy)

			// The danger zone is a band across the page, not a button
			// among the tools: wiping everything deserves to look
			// different from exporting a backup.
			ui.Row(c).Gap(12).AlignItems(ui.Center).FillWidth().
				Padding(14, 16).Radius(RadiusPanel).
				Background(t.Danger.Alpha(0.08)).Border(1, t.Danger.Alpha(0.45)).
				Children(func() {
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Text(c, "危险操作").FontSize(13).Bold().TextColor(t.Danger)
						ui.Text(c, "清空所有数据会删除映射、日志、快照和白名单，无法撤销。").
							FontSize(11).TextColor(t.FG2)
					})
					btn := ui.PrimaryButton(c, "清空所有数据").Disabled(busy)
					btn.Background(t.Danger)
					if btn.Clicked() {
						a.Ask(&appt.Confirm{
							Title: "清空所有数据",
							Body:  "映射、日志、快照和白名单都会被删除，无法撤销。",
							Label: "全部删除", Destructive: true,
							Run: a.Wipe,
						})
					}
				})
		}).Grow(1)
	})
}

// workspaceCard is the settings page's scene manager: the list with
// switch, rename and delete, and the field that adds a scene. The
// one-click switch itself lives on the mappings page; this is where a
// scene is born, renamed and retired.
func (v *View) workspaceCard(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	list, active := a.WorkspacesState()
	v.card(c, t, "场景", func() {
		ui.Text(c, "每个场景一套独立映射：公司、家里各存各的规则，切换后立即按新场景重新应用。").
			FontSize(11).TextColor(t.Muted).Wrap()
		for _, w := range list {
			w := w
			on := w.ID == active
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if v.wsRenameID == w.ID {
					// The rename is the row itself turned into a field,
					// prefilled with the name it would change.
					ui.TextInput(c, &v.wsRenameName).Grow(1).Label("重命名 " + w.Name)
					if ui.Button(c, "保存").FontSize(11).Disabled(busy).Clicked() {
						id, name := w.ID, v.wsRenameName
						v.wsRenameID = ""
						a.RunBackground(func() error { return a.RenameWorkspaceAction(id, name) })
					}
					if ui.Button(c, "取消").FontSize(11).Clicked() {
						v.wsRenameID = ""
					}
					return
				}
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, w.Name).FontSize(12).TextColor(t.FG)
					if on {
						ui.Text(c, "当前场景").FontSize(10).TextColor(t.Accent)
					}
				})
				if !on {
					if ui.Button(c, "切换").FontSize(11).Label("切换 " + w.Name).
						Disabled(busy).Tooltip("切换到这个场景，引擎立即按它重新应用").Clicked() {
						a.RequestSwitchWorkspace(w.ID)
					}
				}
				if ui.Button(c, "重命名").FontSize(11).Label("重命名 " + w.Name).Disabled(busy).Clicked() {
					v.wsRenameID, v.wsRenameName = w.ID, w.Name
				}
				// The last scene and the active one have no delete: the
				// buttons stay rather than flicker with the list.
				del := ui.Button(c, "删除").FontSize(11).Label("删除场景 " + w.Name).
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
		ui.Text(c, "新场景从空白开始，不会影响正在转发的规则。").FontSize(11).TextColor(t.Muted).Wrap()
	})
}

// card draws a titled panel of settings. The width lets two fit a row in
// the window's default size, the way the design lays them out.
func (v *View) card(c *ui.Context, t Tokens, title string, fn func()) {
	ui.Column(c).Grow(1).MinWidth(320).MaxWidth(560).Gap(10).Padding(16).Radius(RadiusPanel).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(t.FG)
		fn()
	})
}
