package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// aiEntry draws the mappings page's door to the AI chat page, when AI is
// configured: the conversation is the one place a mapping draft is
// proposed now. A sentence generates rule cards there, the turns stay
// for follow-up questions, and 填入表单 and 导入到当前场景 both work where
// the reply landed. Without an endpoint and a model the button hides —
// a door that always fails is worse than no door.
func (v *View) aiEntry(c *ui.Context, t Tokens) {
	if !v.App.AIReady() {
		return
	}
	if ui.Button(c, "✨ AI 建映射").Tooltip("去 AI 对话页：一句话生成映射草稿，可连续追问、聊配置").Clicked() {
		v.App.SetPage(appt.PageAIChat)
	}
}
