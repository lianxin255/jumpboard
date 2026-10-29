package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

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
	fill, line := statusTint(t.Success)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(RadiusChip).
		Background(fill).Border(1, line).Children(func() {
		ui.Textf(c, "已导入 %s（合并 %d 条映射）", imp.Name, imp.Count).
			FontSize(CaptionSize).TextColor(t.FG)
		ui.Spacer(c)
		if a.AIReady() {
			if ui.Button(c, "AI 解读").Disabled(busy).
				Tooltip("把这份配置交给模型解释，只读，不写回").Clicked() {
				a.RunBackground(a.InterpretDroppedConf)
			}
		}
		if ui.Button(c, "×").FontSize(CaptionSize).Label("关闭导入提示").
			Tooltip("关掉这条导入提示，已导入的映射不受影响").Clicked() {
			a.DismissConfImport()
		}
	})
}

// interpretSection draws the interpretation card for one source — the
// import bar or the logs page — in the diagnosis card's shape: read-only
// text, a fold button, and nothing that executes. It draws only for the
// source that asked, so the answer lands where the question was pressed.
func (v *View) interpretSection(c *ui.Context, t Tokens, source string) {
	a := v.App
	busy, errText, interp, target, src := a.Interpretation()
	if src != source || (!busy && errText == "" && interp == nil) {
		return
	}
	resultCard(c, t).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "AI 解读").FontSize(NoteSize).Bold().TextColor(t.FG)
				if target != "" {
					ui.Text(c, target).FontSize(NoteSize).TextColor(t.Muted)
				}
				ui.Spacer(c)
				ui.Text(c, "仅供参考，只读展示").FontSize(CaptionSize).TextColor(t.Muted)
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
							ui.Text(c, "正在分析文件…").FontSize(NoteSize).TextColor(t.Muted)
						})
					}
					if errText != "" {
						ui.Text(c, errText).FontSize(CaptionSize).TextColor(t.Danger).Selectable()
					}
					if interp != nil {
						v.interpretBody(c, t, *interp)
					}
				})
			})
		})
	})
}

// interpretFilePicker asks for one configuration or log — an external
// nginx's error.log, a hand-written conf — and hands it to the
// interpretation on the background goroutine: reading the file is disk
// work no frame should wait on. The answer lands on the logs page's card,
// where the 解读文件… button that opened the picker lives.
func (v *View) interpretFilePicker() {
	a := v.App
	// A nil hook reads as a canceled dialog, the way importBackup reads it.
	if a.OpenDialog == nil {
		return
	}
	paths, err := a.OpenDialog(mygo.OpenDialogOptions{
		Title:   "解读文件",
		Filters: []mygo.FileFilter{{Name: "配置与日志", Extensions: []string{"conf", "log", "txt"}}},
	})
	if err != nil {
		a.Fail("打开文件选择器失败：%v", err)
		return
	}
	if len(paths) == 0 {
		return
	}
	path := paths[0]
	a.RunBackground(func() error { return a.InterpretFile("", path) })
}

// interpretBody draws the interpretation itself: the markdown document
// the model wrote (or the JSON fallback rendered into one), shown as it
// streams and kept when it is finished. A streaming card mid-sentence and
// a finished document are different renders — Stream paints progressively
// for the arriving text, View lays the finished document out.
func (v *View) interpretBody(c *ui.Context, t Tokens, ip ai.Interpretation) {
	if ip.Text == "" {
		return
	}
	if v.App.InterpretStreaming() {
		markdown.Stream(c, ip.Text, &markdown.Options{})
		return
	}
	markdown.View(c, ip.Text, &markdown.Options{})
}
