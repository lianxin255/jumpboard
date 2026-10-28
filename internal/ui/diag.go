package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/ai"
)

// diagSection draws the AI diagnose button and, once advice arrived, the
// advice card under the apply-failure bar.
//
// The entry only exists when a failure is showing and AI is configured:
// a button that cannot run is worse than none. The advice is for a human
// to read — a command can be copied, never executed, and nothing about
// it changes the configuration.
func (v *View) diagSection(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	if !a.AIReady() {
		return
	}
	diagBusy, diagErr, advice := a.Diagnosis()
	if advice == nil && !diagBusy && diagErr == "" {
		if ui.Button(c, "AI 诊断").Disabled(busy).
			Tooltip("把错误与日志片段交给模型分析，仅供参考").Clicked() {
			a.RunBackground(func() error { return a.DiagnoseFailure(a.FailKind()) })
		}
		return
	}

	// The card stays until it is dismissed, unlike the toast: an advice
	// is read more than once while fixing the failure it is about.
	ui.Box(c).FillWidth().Padding(10, 12).Radius(RadiusChip).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "AI 诊断").FontSize(NoteSize).Bold().TextColor(t.FG)
				ui.Spacer(c)
				ui.Text(c, "仅供参考，不会自动修改配置").FontSize(CaptionSize).TextColor(t.Muted)
				if ui.Button(c, "收起").Clicked() {
					a.DismissDiagnosis()
				}
			})
			// A long diagnosis must not push the rows off the page; the
			// header stays outside so 收起 is always in reach.
			ui.Scroll(c).MaxHeight(260).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					if diagBusy {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Spinner(c).Label("诊断中")
							ui.Text(c, "正在分析失败原因…").FontSize(NoteSize).TextColor(t.Muted)
						})
					}
					if diagErr != "" {
						ui.Text(c, diagErr).FontSize(CaptionSize).TextColor(t.Danger).Selectable()
					}
					if advice != nil {
						v.adviceBody(c, t, *advice)
					}
				})
			})
		})
	})
}

// adviceBody draws the diagnosis itself: the summary, the likely causes
// and the steps, each command copyable and each risk said out loud.
func (v *View) adviceBody(c *ui.Context, t Tokens, adv ai.Advice) {
	a := v.App
	if adv.Summary != "" {
		ui.Text(c, adv.Summary).FontSize(NoteSize).Bold().TextColor(t.FG).Wrap()
	}
	if len(adv.Causes) > 0 {
		ui.Text(c, "可能原因").FontSize(CaptionSize).Bold().TextColor(t.Muted)
		for _, cause := range adv.Causes {
			ui.Text(c, "• "+cause).FontSize(CaptionSize).TextColor(t.FG2).Wrap()
		}
	}
	if len(adv.Steps) > 0 {
		ui.Text(c, "建议步骤").FontSize(CaptionSize).Bold().TextColor(t.Muted)
		for _, step := range adv.Steps {
			ui.Column(c).Gap(3).Children(func() {
				if step.Title != "" {
					ui.Text(c, step.Title).FontSize(CaptionSize).Bold().TextColor(t.FG).Wrap()
				}
				if step.Detail != "" {
					ui.Text(c, step.Detail).FontSize(CaptionSize).TextColor(t.FG2).Wrap()
				}
				if step.Command != "" {
					cmd := step.Command
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, cmd).Font("monospace").FontSize(CaptionSize).
							TextColor(t.FG2).Selectable()
						// The label carries the step's title: several steps
						// can offer a copy, and a control has to say which
						// one it acts on.
						if ui.Button(c, "复制命令").FontSize(CaptionSize).
							Label("复制命令-" + step.Title).
							Tooltip("复制这条命令，自己决定要不要运行").Clicked() {
							a.RunBackground(func() error { return a.CopyText(cmd) })
						}
					})
				}
				if step.Risk != "" {
					ui.Text(c, "风险："+step.Risk).FontSize(CaptionSize).TextColor(t.Warning).Wrap()
				}
			})
		}
	}
}
