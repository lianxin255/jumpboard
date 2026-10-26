package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/ai"
)

// confImportBar is the strip under the mappings page's actions: it says a
// dropped .conf merged in, and offers the one read-only follow-up — let
// the model explain what landed. The file may already be gone by then;
// the interpretation reports that as an error rather than guessing.
func (v *View) confImportBar(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	imp, ok := a.LastConfImport()
	if !ok {
		return
	}
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(RadiusChip).
		Background(t.Success.Alpha(0.10)).Border(1, t.Success.Alpha(0.30)).Children(func() {
		ui.Textf(c, "已导入 %s（合并 %d 条映射）", imp.Name, imp.Count).
			FontSize(11).TextColor(t.FG)
		ui.Spacer(c)
		if a.AIReady() {
			if ui.Button(c, "AI 解读").Disabled(busy).
				Tooltip("把这份配置交给模型解释，只读，不写回").Clicked() {
				a.RunBackground(a.InterpretDroppedConf)
			}
		}
		if ui.Button(c, "×").FontSize(11).Label("关闭导入提示").
			Tooltip("关掉这条导入提示，已导入的映射不受影响").Clicked() {
			a.DismissConfImport()
		}
	})
}

// interpretSection draws the interpretation card for one source — the
// import bar, the logs page or the AI panel — in the diagnosis card's
// shape: read-only text, a fold button, and nothing that executes. It
// draws only for the source that asked, so the answer lands where the
// question was pressed.
func (v *View) interpretSection(c *ui.Context, t Tokens, source string) {
	a := v.App
	busy, errText, interp, target, src := a.Interpretation()
	if src != source || (!busy && errText == "" && interp == nil) {
		return
	}
	ui.Box(c).FillWidth().Padding(10, 12).Radius(RadiusChip).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "AI 解读").FontSize(12).Bold().TextColor(t.FG)
				if target != "" {
					ui.Text(c, target).FontSize(12).TextColor(t.Muted)
				}
				ui.Spacer(c)
				ui.Text(c, "仅供参考，只读展示").FontSize(10).TextColor(t.Muted)
				if ui.Button(c, "收起").Clicked() {
					a.DismissInterpretation()
				}
			})
			// A long explanation must not push the page's rows away; the
			// header stays outside so 收起 is always in reach.
			ui.Scroll(c).MaxHeight(260).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					if busy {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Spinner(c).Label("解读中")
							ui.Text(c, "正在分析文件…").FontSize(12).TextColor(t.Muted)
						})
					}
					if errText != "" {
						ui.Text(c, errText).FontSize(11).TextColor(t.Danger).Selectable()
					}
					if interp != nil {
						v.interpretBody(c, t, *interp)
					}
				})
			})
		})
	})
}

// interpretBody draws the interpretation itself: the summary, then the
// four sections, each only when the model filled it. The risks are the
// one section painted in warning — it is the part meant to stop a
// copy-paste.
func (v *View) interpretBody(c *ui.Context, t Tokens, ip ai.Interpretation) {
	if ip.Summary != "" {
		ui.Text(c, ip.Summary).FontSize(12).Bold().TextColor(t.FG).Wrap()
	}
	sections := []struct {
		title string
		lines []string
		warn  bool
	}{
		{"谁在听", ip.Listeners, false},
		{"转发关系", ip.Forwards, false},
		{"冲突与风险", ip.Risks, true},
		{"注意事项", ip.Notes, false},
	}
	for _, s := range sections {
		if len(s.lines) == 0 {
			continue
		}
		ui.Text(c, s.title).FontSize(11).Bold().TextColor(t.Muted)
		for _, line := range s.lines {
			color := t.FG2
			if s.warn {
				color = t.Warning
			}
			ui.Text(c, "• "+line).FontSize(11).TextColor(color).Wrap()
		}
	}
}
