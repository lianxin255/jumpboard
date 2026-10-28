package ui

import (
	"fmt"
	"strconv"
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
		v.formDialog(c, t, editing, f, errs, a.FormPortFix())
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
	if a.ConfViewOpen() {
		v.confViewDialog(c, t)
	}
	if pv := a.PastePreview(); pv != nil {
		v.pasteDialog(c, t, pv)
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
func (v *View) formDialog(c *ui.Context, t Tokens, editing *model.Mapping, f appt.FormValues, errs map[string]string, portFix int) {
	a := v.App
	open := true
	title := "新建映射"
	if editing != nil {
		title = "编辑映射"
	}

	// fld is one labelled field: the label above the control, the error
	// under it. Outside a ui.Form, Field draws exactly that. The element
	// comes back so a field can share a row and still take its share of it.
	fld := func(label, errKey string, fn func()) ui.Element {
		e := ui.Field(c, label, fn)
		if msg := errs[errKey]; msg != "" {
			e.Error(msg)
		}
		return e
	}
	// groupHead is the small heading above a set of fields.
	groupHead := func(label string) {
		ui.Text(c, label).FontSize(NoteSize).Bold().TextColor(t.FG)
	}

	ui.Modal(c, &open, func() {
		ui.Column(c).Width(640).Gap(16).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {

				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, title).FontSize(HeadSize).Bold().TextColor(t.FG)
					if isWeb(f.Protocol) {
						ui.Text(c, "反代").FontSize(CaptionSize).Bold().TextColor(t.HTTP).
							Padding(2, 8).Radius(RadiusBadge).Background(t.HTTP.Alpha(0.14))
					}
					ui.Spacer(c)
					if ui.Text(c, "×").FontSize(HeadSize).TextColor(t.Muted).Label("关闭").Clicked() {
						a.CloseForm()
					}
				})
				// Live forward preview so the rule reads as an address path while editing.
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(RadiusChip).
					Background(t.Surface3).Border(1, t.Border).Children(func() {
					ui.Text(c, "转发").FontSize(CaptionSize).Bold().TextColor(t.Muted)
					ui.Text(c, formPreviewLocal(f)).FontSize(NoteSize).Bold().TextColor(t.Accent)
					ui.Text(c, "→").FontSize(NoteSize).TextColor(t.Muted)
					ui.Text(c, formPreviewUpstream(f)).FontSize(NoteSize).TextColor(t.FG2)
					ui.Spacer(c)
					ui.Text(c, f.Protocol).FontSize(CaptionSize).TextColor(t.Muted)
				})

				// The scroll view fills a box of a fixed height: a modal
				// stretches its children, so the box, not the view itself,
				// is what bounds the form. A short form stays short and a
				// long one (HTTPS with every field) scrolls.
				ui.Box(c).Height(formHeight(f, windowHeight(c))).Clip().Children(func() {
					ui.Scroll(c).Fill().Children(func() {
						ui.Column(c).Gap(16).Children(func() {

							// The name and its group share a row: the group is
							// the name's qualifier, a label beside a label. The
							// groups already in use suggest themselves, the way
							// the upstream hosts do.
							ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
								fld("名称", "Name", func() {
									ui.TextInput(c, &f.Name).AutoFocus().Placeholder("例如：内网 MySQL").Grow(1)
								}).Grow(1)
								fld("分组", "", func() {
									ui.Autocomplete(c, &f.Group, v.App.GroupSuggestions()).
										Placeholder("可选，如 数据库").Width(220)
								})
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
							// A save that found the port taken comes back with the
							// nearest free port: one button fills the field, so
							// "who holds the port" leads to "move to this one"
							// without retyping or investigating further.
							if portFix != 0 {
								port := strconv.Itoa(portFix)
								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									ui.Textf(c, "或改用 %d：", portFix).FontSize(CaptionSize).TextColor(t.Muted)
									if ui.Button(c, "改用 "+port).FontSize(CaptionSize).Clicked() {
										// f is this frame's copy of the form;
										// the app holds its own. Writing only
										// the app's would not do: the input
										// built this frame against f writes f
										// back over the app's at the frame's
										// end, and the editor, mid-input, can
										// still prefer its own text. Both
										// sides get the port: the app's, and
										// the copy the next frame builds from.
										f.ListenPort = port
										a.UseFormPortFix()
										a.SetForm(f)
									}
									if ui.Button(c, "忽略").FontSize(CaptionSize).Clicked() {
										a.ClearFormPortFix()
									}
								})
							}
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
									if ui.Button(c, chip.label).FontSize(CaptionSize).
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
					ui.Textf(c, "还有 %d 处需要修正", n).FontSize(CaptionSize).TextColor(t.Danger)
				}

				ui.Divider(c)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "「保存」只入库；「应用配置」保存并生效").FontSize(CaptionSize).TextColor(t.Muted)
					ui.Spacer(c)
					if ui.Button(c, "取消").Clicked() {
						a.CloseForm()
					}
					if ui.Button(c, "保存").Clicked() || c.Shortcut(ui.Cmd, ui.KeyS) {
						f := f
						a.SetForm(f)
						a.RunBackground(a.SaveForm)
					}
					if ui.PrimaryButton(c, "应用配置").
						Tooltip("保存规则并应用配置").Clicked() {
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
		ui.Column(c).Width(680).Gap(12).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Text(c, "从模板新建").FontSize(HeadSize).Bold().TextColor(t.FG)
			ui.Text(c, "选一个模板，表单会预先填好常用端口和指令。").FontSize(NoteSize).TextColor(t.Muted)
			ui.Scroll(c).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					for _, tpl := range a.Tpl.List() {
						tpl := tpl
						ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(RadiusChip).
							Background(t.Surface2).Border(1, t.Border).Children(func() {
							ui.Text(c, tpl.Name).FontSize(BodySize).Bold().TextColor(t.FG).Width(160)
							ui.Text(c, tpl.Category).FontSize(CaptionSize).TextColor(t.Muted).Width(70)
							ui.Text(c, tpl.Description).FontSize(CaptionSize).TextColor(t.FG2).Grow(1).SingleLine()
							// Each row's buttons name their template: a list of
							// identical "使用" buttons is ambiguous to a
							// screen reader, and to the tests.
							if ui.Button(c, "使用 "+tpl.Name).FontSize(CaptionSize).Label("使用 " + tpl.Name).Clicked() {
								a.SetTemplatePicker(false)
								a.OpenFormFromTemplate(tpl)
							}
							if !tpl.Builtin {
								if DangerQuietButton(c, t, "删除 "+tpl.Name).FontSize(CaptionSize).Label("删除模板 " + tpl.Name).Clicked() {
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
		ui.Column(c).Width(420).Gap(12).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Text(c, q.Title).FontSize(HeadSize).Bold().TextColor(t.FG)
			ui.Text(c, q.Body).FontSize(NoteSize).TextColor(t.FG2)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					v.App.ResolveConfirm(false)
				}
				// The label names the confirmation apart from the button
				// that opened it: both say 清空 or 回滚, and a test
				// clicking the first match would hit the opener.
				// Destructive confirms answer in the danger color, the
				// same step every destructive page button takes — and only
				// the one button is built, so the click finds it.
				var btn ui.Element
				if q.Destructive {
					btn = DangerButton(c, t, q.Label).Label("确认" + q.Label)
				} else {
					btn = ui.PrimaryButton(c, q.Label).Label("确认" + q.Label)
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
		ui.Column(c).Width(760).Gap(12).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "配置快照").FontSize(HeadSize).Bold().TextColor(t.FG)
				ui.Text(c, fmt.Sprintf("差异 +%d −%d", added, deleted)).FontSize(CaptionSize).TextColor(t.Meta)
				ui.Spacer(c)
				ui.Text(c, formatTime(sn.CreatedAt)).FontSize(CaptionSize).TextColor(t.Muted)
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
				// question refers to. Rolling back replaces the running
				// configuration: a destructive main action, so the danger
				// face; the confirm it opens still asks before it acts.
				if DangerButton(c, t, "回滚到此配置").Clicked() {
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

// confViewDialog shows the rules as they stand, as the configuration the
// engine runs, in a read-only editor. It is the "what is my nginx.conf
// right now" answer on the settings page: the text is regenerated each
// frame from the mappings, so nothing is cached and a change made while
// the viewer is open shows on the next frame. The buffer is the view's
// own, so the caret and scroll survive the refresh; the language is
// LanguageNamed("shell") — .conf is the syntax this file really is, not
// the TOML that LanguageFor aliases it to.
func (v *View) confViewDialog(c *ui.Context, t Tokens) {
	a := v.App
	if v.confEditor == nil {
		v.confEditor = &editor.State{
			Buffer:   editor.NewBuffer(""),
			ReadOnly: true,
			Label:    "当前 nginx 配置",
		}
		v.confEditor.Buffer.SetLanguage(editor.LanguageNamed("shell"))
	}
	conf, n, err := a.NginxExport()
	if err != nil {
		conf = "# " + err.Error() + "\n"
	} else if conf == "" {
		conf = "# 还没有映射，应用一次配置后这里才有内容。\n"
	}
	if v.confEditor.Buffer.String() != conf {
		buf := editor.NewBuffer(conf)
		buf.SetLanguage(editor.LanguageNamed("shell"))
		v.confEditor.Buffer = buf
	}

	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(860).Gap(12).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "当前配置").FontSize(HeadSize).Bold().TextColor(t.FG)
				ui.Textf(c, "%d 条规则", n).FontSize(CaptionSize).TextColor(t.Meta)
				ui.Spacer(c)
				ui.Text(c, "只读").FontSize(CaptionSize).TextColor(t.Muted)
				if ui.Button(c, "复制全文").FontSize(CaptionSize).
					Tooltip("把这份配置复制到剪贴板").Clicked() {
					a.RunBackground(func() error { return a.CopyText(conf) })
				}
			})
			ui.Box(c).Height(520).Clip().Children(func() {
				editor.View(c, v.confEditor).Grow(1).FillWidth()
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					a.SetConfViewOpen(false)
				}
			})
		})
	})
	if !open {
		a.SetConfViewOpen(false)
	}
}

// pasteDialog shows what a 粘贴导入 would do before it does it: the kind
// the clipboard text turned out to be, how many mappings it touches, and
// for a backup, the plain warning that everything now in the app is
// replaced. Nothing is imported until 导入 runs, so a copy of the wrong
// thing costs nothing.
func (v *View) pasteDialog(c *ui.Context, t Tokens, pv *appt.PastePreview) {
	a := v.App
	open := true
	title := "粘贴导入"
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(520).Gap(12).Padding(24).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Text(c, title).FontSize(HeadSize).Bold().TextColor(t.FG)
			kind := "nginx 配置"
			if pv.Replace {
				kind = "JumpBoard 备份"
			}
			ui.Text(c, "剪贴板内容是 "+kind).FontSize(NoteSize).TextColor(t.FG2)
			ui.Box(c).FillWidth().Padding(10, 12).Radius(RadiusChip).
				Background(t.Surface2).Border(1, t.Border).Children(func() {
				ui.Text(c, pv.Detail).FontSize(NoteSize).TextColor(t.FG).Wrap()
			})
			if pv.Replace {
				ui.Text(c, "现有映射会被备份中的替换，不可撤销（备份前可先导出当前配置）。").
					FontSize(CaptionSize).TextColor(t.Warning).Wrap()
			} else {
				ui.Text(c, "与现有规则合并，只新增，不改动已有的映射。").
					FontSize(CaptionSize).TextColor(t.Muted).Wrap()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.CancelPasteImport()
				}
				// Replacing everything is the destructive path, so its
				// button answers in the danger color; either way one
				// button is built, so the click finds it.
				var btn ui.Element
				if pv.Replace {
					btn = DangerButton(c, t, "导入").Label("导入粘贴内容")
				} else {
					btn = ui.PrimaryButton(c, "导入").Label("导入粘贴内容")
				}
				if btn.Clicked() {
					a.RunBackground(a.ConfirmPasteImport)
				}
			})
		})
	})
	if !open {
		a.CancelPasteImport()
	}
}
