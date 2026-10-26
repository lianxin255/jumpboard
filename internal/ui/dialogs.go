package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
)

// dialogs draws whatever overlay is open.
func (v *View) dialogs(c *ui.Context, t Tokens) {
	a := v.App

	if open, editing, f, errs := a.FormState(); open {
		v.formDialog(c, t, editing, f, errs)
	}
	if a.TemplatePickerOpen() {
		v.templateDialog(c, t)
	}
	if q := a.ConfirmState(); q != nil {
		v.confirmDialog(c, q, t)
	}
	if sn := a.SnapshotView(); sn != nil {
		v.snapshotDialog(c, t, sn)
	}
	v.aiDialog(c, t)
}

// formDialog is the mapping editor.
//
// The inputs bind straight to the form value the app holds: binding them
// to a copy is the classic way to lose every keystroke, and the e2e tests
// check they stick.
//
// The layout follows the design: labels sit above their fields, and the
// fields form a two-column grid, so a rule reads as a form rather than a
// checklist. Groups (转发 / 反向代理 / 健康检查) carry a small heading;
// the protocol is a segmented control, and the extra-directives box gets
// quick-insert chips for the directives people ask for most.
func (v *View) formDialog(c *ui.Context, t Tokens, editing *model.Mapping, f appt.FormValues, errs map[string]string) {
	a := v.App
	open := true
	title := "新建映射"
	if editing != nil {
		title = "编辑映射"
	}

	// fld is one labelled field: the label above the control, the error
	// under it. Outside a ui.Form, Field draws exactly that.
	fld := func(label, errKey string, fn func()) {
		e := ui.Field(c, label, fn)
		if msg := errs[errKey]; msg != "" {
			e.Error(msg)
		}
	}
	// groupHead is the small heading above a set of fields.
	groupHead := func(label string) {
		ui.Text(c, label).FontSize(12).Bold().TextColor(t.FG)
	}

	ui.Modal(c, &open, func() {
		ui.Column(c).Width(640).Gap(16).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {

				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, title).FontSize(16).Bold().TextColor(t.FG)
					if isWeb(f.Protocol) {
						ui.Text(c, "反代").FontSize(10).Bold().TextColor(t.HTTP).
							Padding(2, 8).Radius(RadiusBadge).Background(t.HTTP.Alpha(0.14))
					}
					ui.Spacer(c)
					if ui.Text(c, "×").FontSize(16).TextColor(t.Muted).Label("关闭").Clicked() {
						a.CloseForm()
					}
				})
				// Live forward preview so the rule reads as an address path while editing.
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(RadiusChip).
					Background(t.Surface3).Border(1, t.Border).Children(func() {
					ui.Text(c, "转发").FontSize(10).Bold().TextColor(t.Muted)
					ui.Text(c, formPreviewLocal(f)).FontSize(12).Bold().TextColor(t.Accent)
					ui.Text(c, "→").FontSize(12).TextColor(t.Muted)
					ui.Text(c, formPreviewUpstream(f)).FontSize(12).TextColor(t.FG2)
					ui.Spacer(c)
					ui.Text(c, f.Protocol).FontSize(10).TextColor(t.Muted)
				})

				// The scroll view fills a box of a fixed height: a modal
				// stretches its children, so the box, not the view itself,
				// is what bounds the form. A short form stays short and a
				// long one (HTTPS with every field) scrolls.
				ui.Box(c).Height(formHeight(f, windowHeight(c))).Clip().Children(func() {
					ui.Scroll(c).Fill().Children(func() {
						ui.Column(c).Gap(16).Children(func() {

							fld("名称", "Name", func() {
								ui.TextInput(c, &f.Name).AutoFocus().Placeholder("例如：内网 MySQL")
							})
							ui.Field(c, "协议", func() {
								protoIdx := appt.ProtocolIndex(f.Protocol)
								ui.Segmented(c, &protoIdx, appt.ProtocolChoices...).OnChange(func() {
									f.Protocol = appt.ProtocolChoices[protoIdx]
									a.SetForm(f)
								})
							})

							groupHead("转发")
							ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
								fld("监听端口", "ListenPort", func() {
									if isWeb(f.Protocol) {
										ui.TextInput(c, &f.ListenPort).Placeholder("留空用 80 / 443").Width(180)
									} else {
										ui.TextInput(c, &f.ListenPort).Placeholder("9000").Width(180)
									}
								})
								if isWeb(f.Protocol) {
									fld("server_name", "ServerNames", func() {
										ui.TextInput(c, &f.ServerNames).Placeholder("git.example.com").Grow(1)
									})
								}
							})
							ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
								if isWeb(f.Protocol) {
									fld("location 路径", "", func() {
										ui.TextInput(c, &f.ListenPath).Placeholder("/").Width(140)
									})
								}
								// Host and port on one line, the way an address
								// is written. Each keeps its own label: two
								// controls in one field have to say which is
								// which, to a screen reader and to the tests.
								ui.Field(c, "上游地址", func() {
									ui.Row(c).Gap(8).Children(func() {
										// The host is the form's most repetitive
										// typing: an internal network has a handful
										// of servers, so the ones already in use
										// suggest themselves here.
										e := ui.Autocomplete(c, &f.UpstreamHost, v.App.UpstreamHostSuggestions()).
											Label("上游主机").Placeholder("192.168.1.10")
										if msg := errs["UpstreamHost"]; msg != "" {
											e.Error(msg)
										}
										e.Grow(1)
										e2 := ui.TextInput(c, &f.UpstreamPort).Label("上游端口").Placeholder("3306").Width(90)
										if msg := errs["UpstreamPort"]; msg != "" {
											e2.Error(msg)
										}
									})
								}).Grow(1)
							})

							if isWeb(f.Protocol) {
								groupHead("反向代理")
								modeIdx := appt.ProxyModeIndex(f.ProxyMode)
								ui.Field(c, "代理模式", func() {
									ui.Segmented(c, &modeIdx, appt.ProxyModes...).OnChange(func() {
										f.ProxyMode = appt.ProxyModes[modeIdx]
										a.SetForm(f)
									})
								})
								if f.ProxyMode == appt.ProxyModes[2] {
									fld("自定义代理头", "", func() {
										ui.TextArea(c, &f.ProxyHeaders).Lines(3, 8).
											Placeholder("proxy_set_header Host $host;")
									})
								}
								fld("自定义 proxy_pass", "ProxyPassURL", func() {
									ui.TextInput(c, &f.ProxyPassURL).Placeholder("留空则用上面的上游地址")
								})
							}

							if f.Protocol == appt.ProtocolChoices[3] {
								groupHead("TLS 证书")
								ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
									fld("证书文件", "TLSCertPath", func() {
										ui.TextInput(c, &f.TLSCertPath).Placeholder("/etc/ssl/cert.pem").Grow(1)
									})
									fld("私钥文件", "", func() {
										ui.TextInput(c, &f.TLSKeyPath).Placeholder("/etc/ssl/key.pem").Grow(1)
									})
								})
								ui.Field(c, "HTTP 跳转 HTTPS", func() {
									ui.Switch(c, &f.RedirectHTTP).OnChange(func() { a.SetForm(f) })
								})
							}

							groupHead("健康检查")
							ui.Field(c, "启用检查", func() {
								ui.Switch(c, &f.HealthCheckEnabled).OnChange(func() { a.SetForm(f) })
							})
							if f.HealthCheckEnabled {
								ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
									fld("间隔（秒）", "HealthCheckInterval", func() {
										ui.TextInput(c, &f.HealthCheckInterval).Placeholder("30").Width(140)
									})
									if isWeb(f.Protocol) {
										fld("检查路径", "", func() {
											ui.TextInput(c, &f.HealthCheckPath).Placeholder("/health").Width(180)
										})
										fld("期望状态码", "HealthCheckCode", func() {
											ui.TextInput(c, &f.HealthCheckCode).Placeholder("200").Width(140)
										})
									}
								})
							}

							groupHead("额外指令")
							fld("", "", func() {
								ui.TextArea(c, &f.ExtraDirectives).Lines(3, 8).
									Placeholder("proxy_timeout 1h; —— 引擎能识别的指令会生效，其余会在应用时提示")
							})
							// Quick inserts for the directives the engine
							// honors: a click appends one to the box, so the
							// common cases need no typing.
							ui.Row(c).Gap(8).Children(func() {
								for _, chip := range directiveChips {
									if ui.Button(c, chip.label).FontSize(11).
										Tooltip("插入 " + chip.directive).Clicked() {
										f.ExtraDirectives = appendDirective(f.ExtraDirectives, chip.directive)
										a.SetForm(f)
									}
								}
							})
						})
					})
				})

				if n := len(errs); n > 0 {
					ui.Textf(c, "还有 %d 处需要修正", n).FontSize(11).TextColor(t.Danger)
				}

				ui.Divider(c)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "「保存」只入库；「立即生效」保存并应用配置").FontSize(11).TextColor(t.Muted)
					ui.Spacer(c)
					if ui.Button(c, "取消").Clicked() {
						a.CloseForm()
					}
					if ui.Button(c, "保存").Clicked() || c.Shortcut(ui.Cmd, ui.KeyS) {
						f := f
						a.SetForm(f)
						a.RunBackground(a.SaveForm)
					}
					if ui.PrimaryButton(c, "立即生效").
						Tooltip("保存规则并立即应用配置").Label("立即生效").Clicked() {
						f := f
						a.SetForm(f)
						a.RunBackground(a.SaveFormAndApply)
					}
				})
			})
	})
	a.SetForm(f)

	// Escape or a click outside sets open to false: close the dialog.
	if !open {
		a.CloseForm()
	}
}

// directiveChip is a quick-insert for the extra-directives box.
type directiveChip struct {
	label     string
	directive string
}

// directiveChips are the directives the engine honors, which is also what
// makes them worth a button.
var directiveChips = []directiveChip{
	{"超时 1 小时", "proxy_timeout 1h;"},
	{"WebSocket", "proxy_http_version 1.1;"},
	{"不缓冲", "proxy_buffering off;"},
}

// appendDirective adds a directive to the box, on its own line, skipping
// one that is already there.
func appendDirective(existing, directive string) string {
	trimmed := strings.TrimSpace(existing)
	if trimmed != "" && strings.Contains(trimmed, strings.Split(directive, " ")[0]) {
		return existing
	}
	if trimmed == "" {
		return directive
	}
	return trimmed + "\n" + directive
}

// windowHeight is the room a dialog has: the window's height, or a
// generous default when the frame is measured before layout.
func windowHeight(c *ui.Context) float32 {
	_, h := c.Size()
	if h <= 0 {
		return 760
	}
	return h
}

// isWeb reports whether a protocol label is HTTP or HTTPS, the two that
// reverse-proxy and therefore take the web fields.
func isWeb(protocol string) bool {
	return protocol == appt.ProtocolChoices[2] || protocol == appt.ProtocolChoices[3]
}

// formHeight is how tall the scroll area of the form should be.
//
// The toolkit has no "at most the content, at most this" size, so the
// height is estimated from the fields the form shows. The grid packs two
// fields to a row, so a row costs one field's height (about 57 DIPs: the
// label, its gap, and the control) and a group heading about 25.
//
// maxHeight is the room the window has for the whole dialog: a modal
// taller than that pushes its own buttons off screen, where nothing can
// reach them — the smallest window the app allows must still be able to
// save.
func formHeight(f appt.FormValues, maxHeight float32) float32 {
	const row, head = float32(57), float32(25)
	h := 2*row + head // name, protocol, the 转发 heading
	h += row          // listen port, sharing a row with server_name
	h += row          // upstream, sharing a row with the location path
	if f.Protocol == appt.ProtocolChoices[3] {
		h += head + row + row // TLS heading, cert+key row, redirect
	}
	if isWeb(f.Protocol) {
		h += head + row + row // 反向代理 heading, mode, proxy_pass
		if f.ProxyMode == appt.ProxyModes[2] {
			h += 3 * row // custom headers, as a text area
		}
	}
	h += head + row // 健康检查 heading and its switch
	if f.HealthCheckEnabled {
		h += row // the interval, path, code share a row
	}
	h += head + 3*row + row // 额外指令 heading, the box, the chips
	// Everything but the scroll area: the title row, the live-preview band,
	// the error line, the divider, the footer, and the dialog's padding.
	const chrome = 268
	ceiling := maxHeight - chrome
	if ceiling < 160 {
		// A window too short for any form: keep the area usable and let
		// the dialog overflow rather than vanish.
		ceiling = 160
	}
	if h > ceiling {
		h = ceiling
	}
	return h
}

// templateDialog lists the templates a new mapping can start from.
func (v *View) templateDialog(c *ui.Context, t Tokens) {
	a := v.App
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(680).Gap(12).Padding(20).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Text(c, "从模板新建").FontSize(16).Bold().TextColor(t.FG)
			ui.Text(c, "选一个模板，表单会预先填好常用端口和指令。").FontSize(12).TextColor(t.Muted)
			ui.Scroll(c).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					for _, tpl := range a.Tpl.List() {
						tpl := tpl
						ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(RadiusChip).
							Background(t.Surface2).Border(1, t.Border).Children(func() {
							ui.Text(c, tpl.Name).FontSize(13).Bold().TextColor(t.FG).Width(160)
							ui.Text(c, tpl.Category).FontSize(11).TextColor(t.Muted).Width(70)
							ui.Text(c, tpl.Description).FontSize(11).TextColor(t.FG2).Grow(1).SingleLine()
							// Each row's buttons name their template: a list of
							// identical "使用" buttons is ambiguous to a
							// screen reader, and to the tests.
							if ui.Button(c, "使用 "+tpl.Name).FontSize(11).Label("使用 " + tpl.Name).Clicked() {
								a.SetTemplatePicker(false)
								a.OpenFormFromTemplate(tpl)
							}
							if !tpl.Builtin {
								if ui.Button(c, "删除 "+tpl.Name).FontSize(11).Label("删除模板 " + tpl.Name).Clicked() {
									if err := a.Tpl.Delete(tpl.ID); err != nil {
										a.Fail("%v", err)
									} else {
										a.Info("已删除模板「%s」", tpl.Name)
									}
								}
							}
						})
					}
				})
			}).Height(380)
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					a.SetTemplatePicker(false)
				}
			})
		})
	})
	if !open && a.TemplatePickerOpen() {
		a.SetTemplatePicker(false)
	}
}

// confirmDialog asks a question with an OK and a cancel.
func (v *View) confirmDialog(c *ui.Context, q *appt.Confirm, t Tokens) {
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(420).Gap(12).Padding(20).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Text(c, q.Title).FontSize(15).Bold().TextColor(t.FG)
			ui.Text(c, q.Body).FontSize(12).TextColor(t.FG2)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					v.App.ResolveConfirm(false)
				}
				// The label names the confirmation apart from the button
				// that opened it: both say 清空 or 回滚, and a test
				// clicking the first match would hit the opener.
				btn := ui.PrimaryButton(c, q.Label).Label("确认" + q.Label)
				if q.Destructive {
					btn.Background(t.Danger)
				}
				if btn.Clicked() {
					v.App.ResolveConfirm(true)
				}
			})
		})
	})
	if !open && v.App.ConfirmState() != nil {
		v.App.ResolveConfirm(false)
	}
}

// snapshotDialog shows what rolling back to a saved configuration would
// change: the rules as they stand against the snapshot, as a diff. The
// generated header carries the time it was written, which would show as a
// change on every snapshot, so that one line is blanked on both sides
// before they are compared.
func (v *View) snapshotDialog(c *ui.Context, t Tokens, sn *model.Snapshot) {
	a := v.App
	current, _, err := a.NginxExport()
	if err != nil {
		current = ""
	}
	diff := &editor.DiffState{
		Old:          editor.NewBuffer(stripConfTimestamp(current)),
		New:          editor.NewBuffer(stripConfTimestamp(sn.Content)),
		Layout:       editor.DiffUnified,
		Inline:       editor.DiffInlineWordAlt,
		ContextLines: 3,
	}
	added, deleted := diff.Stats()

	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(760).Gap(12).Padding(20).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "配置快照").FontSize(15).Bold().TextColor(t.FG)
				ui.Text(c, fmt.Sprintf("差异 +%d −%d", added, deleted)).FontSize(11).TextColor(t.Meta)
				ui.Spacer(c)
				ui.Text(c, formatTime(sn.CreatedAt)).FontSize(11).TextColor(t.Muted)
			})
			ui.Scroll(c).Height(460).Children(func() {
				editor.DiffView(c, diff)
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					a.SetSnapshotView(nil)
				}
				// The button used to roll back the moment it was clicked, with
				// nothing to confirm against. The diff above is what the
				// question refers to.
				if ui.PrimaryButton(c, "回滚到此配置").Clicked() {
					snap := *sn
					a.SetSnapshotView(nil)
					a.Ask(&appt.Confirm{
						Title: "回滚配置",
						Body: fmt.Sprintf("回滚到 %s 的配置，相对当前 %+d −%d 行。当前配置会先存为一条快照。",
							formatTime(snap.CreatedAt), added, deleted),
						Label: "回滚",
						Run:   func() error { return a.RollbackSnapshot(snap) },
					})
				}
			})
		})
	})
	if !open && a.SnapshotView() != nil {
		a.SetSnapshotView(nil)
	}
}

// stripConfTimestamp blanks the generated header's timestamp, which changes
// on every write and would otherwise be the only difference between two
// identical rule sets. The line is kept, so the rest of the file stays on
// the same line numbers.

// snapshotDiffStats is the added and deleted line count the dialog shows,
// computed the same way, so a test can check the numbers without a window.
func snapshotDiffStats(current, snapshot string) (added, deleted int) {
	d := &editor.DiffState{
		Old: editor.NewBuffer(stripConfTimestamp(current)),
		New: editor.NewBuffer(stripConfTimestamp(snapshot)),
	}
	return d.Stats()
}
func stripConfTimestamp(conf string) string {
	const prefix = "# 生成时间: "
	lines := strings.Split(conf, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			lines[i] = prefix
		}
	}
	return strings.Join(lines, "\n")
}

// toast draws the last action's outcome.
//
// It is shown once per status: the toolkit re-shows a toast added with an
// existing ID, so calling it every frame would restart the countdown and
// the message would never leave.
func (v *View) toast(c *ui.Context, t Tokens) {
	st, ok := v.App.Status()
	if !ok || st.Text == "" {
		return
	}
	if st.At.Equal(v.shownStatusAt) && st.Text == v.shownStatusText {
		return
	}
	v.shownStatusAt, v.shownStatusText = st.At, st.Text
	c.Toast(st.Text)
	// The status is dropped after its time, so that the next frame does
	// not add it again.
	c.After(5 * time.Second)
	v.App.ClearStatusSoon(st.At)
}

// formPreviewLocal is the left side of the form's live forward path.
func formPreviewLocal(f appt.FormValues) string {
	port := strings.TrimSpace(f.ListenPort)
	if port == "" {
		if f.Protocol == appt.ProtocolChoices[3] {
			port = "443"
		} else if isWeb(f.Protocol) {
			port = "80"
		} else {
			port = "?"
		}
	}
	if name := strings.TrimSpace(f.ServerNames); name != "" {
		return name + ":" + port
	}
	return "本机:" + port
}

// formPreviewUpstream is the right side of the form's live forward path.
func formPreviewUpstream(f appt.FormValues) string {
	host := strings.TrimSpace(f.UpstreamHost)
	port := strings.TrimSpace(f.UpstreamPort)
	if host == "" {
		host = "?"
	}
	if port == "" {
		port = "?"
	}
	return host + ":" + port
}

// formatTime renders a millisecond timestamp as a short local time.
func formatTime(ms int64) string {
	if ms == 0 {
		return "—"
	}
	return timeAt(ms).Format("2006-01-02 15:04")
}
