package ui

import (
	"github.com/egoist/mygo/ui"
)

// aiCard draws the AI settings: where the model lives, what it is called,
// and the key, which goes to the system credential store.
//
// The model field is a combo box once the endpoint's list has been
// fetched: a gateway can offer dozens of names, and typing one from memory
// is how you end up with "gpt-4o" where the gateway spells it
// "gpt-4o-2024-08-06". It stays free text, since the list may omit a model
// the gateway still accepts.
func (v *View) aiCard(c *ui.Context, t Tokens) {
	a := v.App
	if !v.aiSettingsLoaded {
		v.aiBaseURL = a.AIBaseURL()
		v.aiModel = a.AIModel()
		v.aiDiagModel = a.AIDiagModel()
		v.aiKeySet = a.HasAIKey()
		v.aiSettingsLoaded = true
	}
	// One snapshot of what the fetch goroutine may have changed, so the
	// frame never reads a field it is writing.
	v.aiMu.Lock()
	models, fetching, listErr, suggest := v.aiModels, v.aiFetching, v.aiListErr, v.aiSuggest
	v.aiMu.Unlock()
	// The fetch's suggestion is applied here, on the frame's own thread:
	// the goroutine never touches aiModel, the field the TextInput
	// buffers. It fills the field once, only while it is still empty, so
	// a user who typed meanwhile keeps what they typed.
	if suggest != "" {
		v.aiMu.Lock()
		v.aiSuggest = ""
		v.aiMu.Unlock()
		if v.aiModel == "" {
			v.aiModel = suggest
		}
	}

	wideCard(c, t, "AI 配置", func() {
		// The form the settings live in and the reading that explains
		// them sit side by side while the card has room for both, and
		// stack — the notes below the form — once it does not. Nothing is
		// folded away for either shape: every line stays whole.
		ui.Row(c).Gap(24).AlignItems(ui.Start).Wrap().Children(func() {
			ui.Column(c).Grow(1).MinWidth(settingsCardMin).Gap(10).Children(func() {
				ui.Field(c, "接口地址", func() {
					ui.TextInput(c, &v.aiBaseURL).Placeholder("https://api.openai.com 或本地网关")
				})
				ui.Field(c, "模型", func() {
					// A plain text field with a list beside it, not a combo box:
					// a combo restores the field to the last chosen option when it
					// loses focus, which silently throws away a name typed for a
					// gateway whose list does not mention it. The field is the
					// truth; the menu only fills it in.
					ui.Row(c).Gap(8).Children(func() {
						e := ui.TextInput(c, &v.aiModel).Placeholder("gpt-4o-mini / deepseek-chat / 本地模型名")
						if ui.Button(c, "从列表选").Disabled(len(models) == 0).
							Tooltip("先点「拉取模型」").Clicked() {
							v.modelPicker = !v.modelPicker
						}
						if len(models) > 0 && v.modelPicker {
							if ui.Button(c, "收起").Clicked() {
								v.modelPicker = false
							}
						}
						_ = e
					})
					if len(models) > 0 && v.modelPicker {
						ui.Scroll(c).MaxHeight(150).Children(func() {
							ui.Column(c).Gap(2).Children(func() {
								for _, name := range models {
									name := name
									on := name == v.aiModel
									row := ui.Text(c, name).FontSize(NoteSize).Label("选 "+name).Padding(3, 6)
									if on {
										row.Bold().TextColor(t.Accent)
									} else {
										row.TextColor(t.FG2)
									}
									if row.Clicked() {
										v.aiModel = name
										v.modelPicker = false
									}
								}
							})
						})
					}
				})
				ui.Field(c, "诊断/解读模型", func() {
					ui.TextInput(c, &v.aiDiagModel).Placeholder("留空 = 与主模型相同")
				})
				ui.Field(c, "API Key", func() {
					placeholder := "sk-…"
					if v.aiKeySet {
						placeholder = "已保存（留空则不修改）"
					}
					if ui.TextInput(c, &v.aiKey).Password().Placeholder(placeholder).Submitted() {
						v.saveAIKey()
					}
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					if ui.Button(c, "拉取模型").Disabled(fetching).
						Tooltip("从接口地址读取可用模型列表（需要网关支持 /v1/models）").Clicked() {
						v.fetchModels()
					}
					if fetching {
						ui.Spinner(c).Label("拉取中")
					}
					if ui.Button(c, "保存").Clicked() {
						if err := a.SetAI(v.aiBaseURL, v.aiModel); err != nil {
							a.Fail("AI 配置未保存：%v", err)
						} else if err := a.SetAIDiagModel(v.aiDiagModel); err != nil {
							a.Fail("诊断模型未保存：%v", err)
						} else {
							v.saveAIKey()
							a.Info("AI 配置已保存")
						}
					}
					if ui.Button(c, "测试连接").Clicked() {
						a.RunBackground(a.ProbeAI)
					}
				})
				if listErr != "" {
					ui.Text(c, listErr).FontSize(CaptionSize).TextColor(t.Danger).Wrap()
				} else if len(models) > 0 {
					ui.Textf(c, "已拉取 %d 个模型，可从「模型」下拉里选。", len(models)).
						FontSize(CaptionSize).TextColor(t.Muted).Wrap()
				}
			})

			ui.Column(c).Grow(1).MinWidth(settingsCardMin).Gap(10).Children(func() {
				ui.Text(c, "模型建议：映射草稿、失败诊断与文件解读用稳定的强模型（如 group/sota）；fast 类模型只建议用于拉取模型列表和测试连接。").
					FontSize(CaptionSize).TextColor(t.Muted).Wrap()
				// What the AI features send is required reading, so none of it is
				// hidden behind a disclosure: the lines group under one quiet
				// heading with tighter spacing instead of four same-weight
				// captions in a row. (A collapsible would read cleaner but keeps
				// an open/closed bool alive across frames — state this page does
				// not hold.)
				ui.Text(c, "数据与隐私").FontSize(CaptionSize).Bold().TextColor(t.FG2)
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "Key 存在系统凭据库，不写进数据库或导出文件。").
						FontSize(CaptionSize).TextColor(t.Muted).Wrap()
					ui.Text(c, "生成草稿（一次可生成多条，最多 5 条）只发送你输入的那句话；失败诊断会发送错误信息、涉及映射的摘要（至多两条）和两份日志的尾部片段；解读会发送所选配置或日志的文件内容，超过 256 KB 的文件会被拒绝；自然语言筛选会把全部映射的摘要（名称/协议/端口/上游/启用状态，不含密钥）发送到所配置的网关。").
						FontSize(CaptionSize).TextColor(t.Muted).Wrap()
					ui.Text(c, "除自然语言筛选发送全部映射的摘要外，其余功能均不上传完整映射列表；映射摘要里永远不含密钥。").
						FontSize(CaptionSize).TextColor(t.Muted).Wrap()
				})
				if !a.AIReady() {
					ui.Text(c, "填好地址和模型后，映射页才会出现 AI 入口。").
						FontSize(CaptionSize).TextColor(t.Warning)
				}
			})
		})
	})
}

// fetchModels asks the endpoint what it offers, off the main thread.
//
// Every field the goroutine also touches goes through aiMu, including the
// ones set before it starts: the button's Disabled gate stops a second
// click today, but the lock is what makes that true rather than a hope.
func (v *View) fetchModels() {
	a := v.App
	base, key := v.aiBaseURL, v.aiKey
	v.aiMu.Lock()
	v.aiFetching, v.aiListErr = true, ""
	v.aiMu.Unlock()
	go func() {
		models, err := a.FetchModels(base, key)
		v.aiMu.Lock()
		v.aiFetching = false
		if err != nil {
			v.aiListErr = "拉取失败：" + err.Error()
			v.aiModels = nil
		} else if len(models) == 0 {
			v.aiListErr = "接口没有返回任何模型"
			v.aiModels = nil
		} else {
			v.aiModels = models
			// Offer the first model once, as the fetch lands, so that a
			// user who only wanted a working default has one step left:
			// 保存. The offer rides aiSuggest and the frame fills the
			// field itself; writing aiModel here would race the
			// TextInput's buffer.
			v.aiSuggest = models[0]
		}
		v.aiMu.Unlock()
		a.Change()
	}()
}

// saveAIKey stores what the field holds, if anything was typed.
func (v *View) saveAIKey() {
	if v.aiKey == "" {
		return
	}
	a := v.App
	key := v.aiKey
	v.aiKey = ""
	v.aiKeySet = true
	a.RunBackground(func() error { return a.SetAIKey(key) })
}
