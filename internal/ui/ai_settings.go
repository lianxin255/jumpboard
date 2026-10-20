package ui

import (
	"github.com/egoist/mygo/ui"
)

// aiCard draws the AI settings: where the model lives, what it is called,
// and the key, which goes to the system credential store.
func (v *View) aiCard(c *ui.Context, t Tokens) {
	a := v.App
	if !v.aiSettingsLoaded {
		v.aiBaseURL = a.AIBaseURL()
		v.aiModel = a.AIModel()
		v.aiKeySet = a.HasAIKey()
		v.aiSettingsLoaded = true
	}

	v.card(c, t, "AI 配置", func() {
		ui.Field(c, "接口地址", func() {
			ui.TextInput(c, &v.aiBaseURL).Placeholder("https://api.openai.com 或本地网关")
		})
		ui.Field(c, "模型", func() {
			ui.TextInput(c, &v.aiModel).Placeholder("gpt-4o-mini / deepseek-chat / 本地模型名")
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
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "保存").Clicked() {
				a.SetAI(v.aiBaseURL, v.aiModel)
				v.saveAIKey()
				a.Info("AI 配置已保存")
			}
			if ui.Button(c, "测试连接").Clicked() {
				a.RunBackground(a.ProbeAI)
			}
		})
		ui.Text(c, "Key 保存在系统凭据库（macOS 钥匙串 / Windows 凭据管理器），不会写进数据库或导出文件。").
			FontSize(11).TextColor(t.Muted)
		ui.Text(c, "使用 AI 时，你输入的那句话会发送到上面的地址；当前映射列表不会被发送。").
			FontSize(11).TextColor(t.Muted)
		if !a.AIReady() {
			ui.Text(c, "填好地址和模型后，映射页才会出现 AI 入口。").
				FontSize(11).TextColor(t.Warning)
		}
	})
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
