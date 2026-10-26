package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// aiFilterBar draws the natural language filter in the mappings page's
// toolbar row: one input, one button, and a spinner while the ask is
// out. The filter only narrows what the table shows — nothing it does
// saves, applies or writes anything — and the one thing it sends is a
// summary of the table, which the input's tooltip says out loud.
func (v *View) aiFilterBar(c *ui.Context, t Tokens) {
	a := v.App
	if !a.AIReady() {
		return
	}
	busy, _, _, _, _ := a.AIFilterState()
	if ui.TextInput(c, &v.aiFilterPrompt).
		Placeholder("用一句话筛选，如：8080 且未启用").
		Width(230).
		Label("AI 筛选").
		Tooltip("把全部映射的摘要（名称、协议、端口、上游、启用状态，不含密钥）发送到所配置的 AI 网关，只筛选显示，不会修改配置").
		Submitted() {
		v.runAIFilter()
	}
	if ui.PrimaryButton(c, "开始筛选").
		Disabled(busy || strings.TrimSpace(v.aiFilterPrompt) == "").
		Tooltip("按这句话筛选表格，只影响显示").Clicked() {
		v.runAIFilter()
	}
	if busy {
		ui.Spinner(c).Label("筛选中")
		ui.Text(c, "筛选中…").FontSize(11).TextColor(t.Muted)
	}
}

// aiFilterResult draws the slim line an ask leaves under the toolbar:
// the error in red when it failed, and — once it landed — the query, the
// model's one-line rationale, how many rows survived and the button that
// puts the table back. When nothing matched, the empty-state card below
// already carries the clear button, so this line does not add a second
// one; its rationale is the sentence that card's hint cannot know.
func (v *View) aiFilterResult(c *ui.Context, t Tokens) {
	a := v.App
	busy, errText, query, rationale, ids := a.AIFilterState()
	if busy || (errText == "" && query == "") {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if errText != "" {
			ui.Text(c, "AI 筛选失败："+errText).FontSize(11).TextColor(t.Danger)
			return
		}
		line := "AI 筛选「" + query + "」"
		if rationale != "" {
			line += "：" + rationale
		}
		ui.Text(c, line).FontSize(11).TextColor(t.Muted)
		ui.Textf(c, "（%d 条）", len(ids)).FontSize(11).TextColor(t.Muted)
		if len(ids) > 0 {
			if ui.Button(c, "清除筛选").FontSize(11).
				Tooltip("取消这句话的筛选，回到全部映射").Clicked() {
				a.ClearAIFilter()
			}
		}
	})
}

// runAIFilter hands the typed sentence to the model on the background
// goroutine: the request blocks for seconds, which no frame may wait
// out. The button is disabled while an ask is out, and AskAIFilter
// refuses a second one itself — the check here keeps a stale click from
// even reaching it.
func (v *View) runAIFilter() {
	a := v.App
	prompt := strings.TrimSpace(v.aiFilterPrompt)
	busy, _, _, _, _ := a.AIFilterState()
	if prompt == "" || busy {
		return
	}
	a.RunBackground(func() error { return a.AskAIFilter(prompt) })
}
