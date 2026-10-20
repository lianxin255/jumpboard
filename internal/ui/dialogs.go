package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
	"github.com/lianxin255/nginx-jump-gui/internal/model"
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
func (v *View) formDialog(c *ui.Context, t Tokens, editing *model.Mapping, f appt.FormValues, errs map[string]string) {
	a := v.App
	open := true
	title := "新建映射"
	if editing != nil {
		title = "编辑映射"
	}

	ui.Modal(c, &open, func() {
		ui.Column(c).Width(600).Gap(14).Padding(20).Radius(6).
			Background(t.Surface).Border(1, t.Border).Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.5)).
			Children(func() {

				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, title).FontSize(16).Bold().TextColor(t.FG)
					ui.Spacer(c)
					if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
						ui.Text(c, "反代").FontSize(10).TextColor(t.HTTP).
							Padding(2, 6).Radius(2).Border(1, t.HTTP)
					}
				})

				// The scroll view fills a box of a fixed height: a modal
				// stretches its children, so the box, not the view itself,
				// is what bounds the form. A short form stays short and a
				// long one (HTTPS with every field) scrolls.
				ui.Box(c).Height(formHeight(f)).Clip().Children(func() {
					ui.Scroll(c).Fill().Children(func() {
						ui.Column(c).Gap(12).Children(func() {
							ui.Form(c, func() {
								fld := func(label, errKey string, fn func()) {
									e := ui.Field(c, label, fn)
									if msg := errs[errKey]; msg != "" {
										e.Error(msg)
									}
								}
								fld("名称", "Name", func() {
									ui.TextInput(c, &f.Name).AutoFocus().Placeholder("例如：内网 MySQL")
								})
								fld("协议", "", func() {
									ui.Select(c, &f.Protocol, []string{"TCP", "UDP", "HTTP", "HTTPS"}).OnChange(func() {
										a.SetForm(f)
									})
								})
								if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
									fld("server_name", "ServerNames", func() {
										ui.TextInput(c, &f.ServerNames).Placeholder("git.example.com")
									})
									fld("location 路径", "", func() {
										ui.TextInput(c, &f.ListenPath).Placeholder("/")
									})
								}
								fld("监听端口", "ListenPort", func() {
									if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
										ui.TextInput(c, &f.ListenPort).Placeholder("留空用 80 / 443")
									} else {
										ui.TextInput(c, &f.ListenPort).Placeholder("9000")
									}
								})
								fld("上游主机", "UpstreamHost", func() {
									ui.TextInput(c, &f.UpstreamHost).Placeholder("192.168.1.10")
								})
								fld("上游端口", "UpstreamPort", func() {
									ui.TextInput(c, &f.UpstreamPort).Placeholder("3306")
								})

								if f.Protocol == "HTTPS" {
									fld("TLS 证书", "TLSCertPath", func() {
										ui.TextInput(c, &f.TLSCertPath).Placeholder("/etc/ssl/cert.pem")
									})
									fld("TLS 私钥", "", func() {
										ui.TextInput(c, &f.TLSKeyPath).Placeholder("/etc/ssl/key.pem")
									})
									fld("HTTP 跳转 HTTPS", "", func() {
										ui.Switch(c, &f.RedirectHTTP).OnChange(func() { a.SetForm(f) })
									})
								}

								if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
									fld("代理模式", "", func() {
										ui.Select(c, &f.ProxyMode, []string{"标准反代", "精简代理", "完全自定义"}).OnChange(func() {
											a.SetForm(f)
										})
									})
									if f.ProxyMode == "完全自定义" {
										fld("自定义代理头", "", func() {
											ui.TextArea(c, &f.ProxyHeaders).Lines(4, 8).
												Placeholder("proxy_set_header Host $host;")
										})
									}
									fld("自定义 proxy_pass", "ProxyPassURL", func() {
										ui.TextInput(c, &f.ProxyPassURL).Placeholder("留空则用 http://上游主机:端口")
									})
								}

								fld("健康检查", "", func() {
									ui.Switch(c, &f.HealthCheckEnabled).OnChange(func() { a.SetForm(f) })
								})
								if f.HealthCheckEnabled {
									fld("检查间隔（秒）", "HealthCheckInterval", func() {
										ui.TextInput(c, &f.HealthCheckInterval).Placeholder("30")
									})
									if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
										fld("检查路径", "", func() {
											ui.TextInput(c, &f.HealthCheckPath).Placeholder("/health")
										})
										fld("期望状态码", "HealthCheckCode", func() {
											ui.TextInput(c, &f.HealthCheckCode).Placeholder("200")
										})
									}
								}

								fld("额外指令", "", func() {
									ui.TextArea(c, &f.ExtraDirectives).Lines(3, 8).
										Placeholder("proxy_timeout 1h; —— 引擎能识别的指令会生效，其余会在应用时提示")
								})
							})
						})
					})
				})

				if n := len(errs); n > 0 {
					ui.Textf(c, "还有 %d 处需要修正", n).FontSize(11).TextColor(t.Danger)
				}

				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if ui.Button(c, "取消").Clicked() {
						a.CloseForm()
					}
					if ui.PrimaryButton(c, "保存").Clicked() || c.Shortcut(ui.Cmd, ui.KeyS) {
						f := f
						a.SetForm(f)
						a.RunBackground(a.SaveForm)
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

// formHeight is how tall the scroll area of the form should be.
//
// The toolkit has no "at most the content, at most this" size, so the
// height is estimated from the fields the form shows: about 48 DIPs each,
// with the text areas counted higher.
func formHeight(f appt.FormValues) float32 {
	rows := 6 // name, protocol, listen, host, port, health
	if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
		rows += 2 // server_name, listen path
	}
	if f.Protocol == "HTTPS" {
		rows += 3 // certificate, key, redirect
	}
	if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
		rows++ // proxy mode
		if f.ProxyMode == "完全自定义" {
			rows += 2 // custom headers, as a text area
		}
		rows++ // custom proxy_pass
	}
	if f.HealthCheckEnabled {
		rows += 2
		if f.Protocol == "HTTP" || f.Protocol == "HTTPS" {
			rows += 2
		}
	}
	rows += 2 // extra directives, as a text area
	// A field with its label is about 44 DIPs; the ceiling is what the
	// window can spare once the title, the buttons and the padding are
	// there, so a long form scrolls rather than growing past the screen.
	h := float32(rows) * 44
	if h > 600 {
		h = 600
	}
	return h
}

// templateDialog lists the templates a new mapping can start from.
func (v *View) templateDialog(c *ui.Context, t Tokens) {
	a := v.App
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(680).Gap(12).Padding(20).Radius(6).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Text(c, "从模板新建").FontSize(16).Bold().TextColor(t.FG)
			ui.Text(c, "选一个模板，表单会预先填好常用端口和指令。").FontSize(12).TextColor(t.Muted)
			ui.Scroll(c).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					for _, tpl := range a.Tpl.List() {
						tpl := tpl
						ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(4).
							Background(t.Surface2).Children(func() {
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
		ui.Column(c).Width(420).Gap(12).Padding(20).Radius(6).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Text(c, q.Title).FontSize(15).Bold().TextColor(t.FG)
			ui.Text(c, q.Body).FontSize(12).TextColor(t.FG2)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					v.App.ResolveConfirm(false)
				}
				btn := ui.PrimaryButton(c, q.Label)
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

// snapshotDialog shows a saved configuration, read only.
func (v *View) snapshotDialog(c *ui.Context, t Tokens, sn *model.Snapshot) {
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(760).Gap(12).Padding(20).Radius(6).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "配置快照").FontSize(15).Bold().TextColor(t.FG)
				ui.Spacer(c)
				ui.Text(c, formatTime(sn.CreatedAt)).FontSize(11).TextColor(t.Muted)
			})
			ui.Scroll(c).Height(460).Children(func() {
				ui.Text(c, sn.Content).Font("monospace").FontSize(11).
					TextColor(t.FG2).Selectable()
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					v.App.SetSnapshotView(nil)
				}
				if ui.PrimaryButton(c, "回滚到此配置").Clicked() {
					snap := *sn
					v.App.SetSnapshotView(nil)
					v.App.RunBackground(func() error { return v.App.RollbackSnapshot(snap) })
				}
			})
		})
	})
	if !open && v.App.SnapshotView() != nil {
		v.App.SetSnapshotView(nil)
	}
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

// formatTime renders a millisecond timestamp as a short local time.
func formatTime(ms int64) string {
	if ms == 0 {
		return "—"
	}
	return timeAt(ms).Format("2006-01-02 15:04")
}

// settingString reads a setting for the frame.
func (v *View) settingString(key, def string) string { return v.App.Store.Setting(key, def) }
