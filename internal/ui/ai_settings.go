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
		v.aiKeySet = a.HasAIKey()
		v.aiSettingsLoaded = true
	}
	// One snapshot of what the fetch goroutine may have changed, so the
	// frame never reads a field it is writing.
	v.aiMu.Lock()
	models, fetching, listErr := v.aiModels, v.aiFetching, v.aiListErr
	v.aiMu.Unlock()

	// A fetch that came back with a list, over an empty model field,
	// suggests the first one: the point of 拉取模型 is to save typing, and
	// leaving the field blank would make the user open the list anyway.
	// It is only a suggestion — the field stays editable and 保存 is still
	// theirs to press.
	if len(models) > 0 && v.aiModel == "" {
		v.aiModel = models[0]
	}

	v.card(c, t, "AI 配置", func() {
		ui.Field(c, "接口地址", func() {
			ui.TextInput(c, &v.aiBaseURL).Placeholder("https://api.openai.com 或本地网关")
		})
		ui.Field(c, "模型", func() {
			if len(models) > 0 {
				if ui.Combobox(c, &v.aiModel, models).
					Placeholder("从列表选择，或直接输入").Submitted() {
					a.SetAI(v.aiBaseURL, v.aiModel)
				}
			} else {
				ui.TextInput(c, &v.aiModel).Placeholder("gpt-4o-mini / deepseek-chat / 本地模型名")
			}
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
				a.SetAI(v.aiBaseURL, v.aiModel)
				v.saveAIKey()
				a.Info("AI 配置已保存")
			}
			if ui.Button(c, "测试连接").Clicked() {
				a.RunBackground(a.ProbeAI)
			}
		})
		if listErr != "" {
			ui.Text(c, listErr).FontSize(11).TextColor(t.Danger).Wrap()
		} else if len(models) > 0 {
			ui.Textf(c, "已拉取 %d 个模型，可从「模型」下拉里选。", len(models)).
				FontSize(11).TextColor(t.Muted).Wrap()
		}
		ui.Text(c, "Key 存在系统凭据库，不写进数据库或导出文件。").
			FontSize(11).TextColor(t.Muted).Wrap()
		ui.Text(c, "使用 AI 时只发送你输入的那句话，不上传映射列表。").
			FontSize(11).TextColor(t.Muted).Wrap()
		if !a.AIReady() {
			ui.Text(c, "填好地址和模型后，映射页才会出现 AI 入口。").
				FontSize(11).TextColor(t.Warning)
		}
	})
}

// fetchModels asks the endpoint what it offers, off the main thread.
func (v *View) fetchModels() {
	a := v.App
	base, key := v.aiBaseURL, v.aiKey
	v.aiFetching, v.aiListErr = true, ""
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
