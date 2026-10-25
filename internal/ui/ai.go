package ui

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lianxin255/jumpboard/internal/model"
)

// markdownState is the renderer's cache, kept across frames.
type markdownState = markdown.Options

// aiState is the AI panel's own state.
//
// open and prompt are the main thread's: only a frame or an action a frame
// ran touches them. busy, err, reply and draft are written by the request
// goroutine and read by every frame, so they live behind mu, and the frame
// takes one snapshot instead of holding the lock while it draws.
type aiState struct {
	open   bool
	prompt string

	mu    sync.Mutex
	busy  bool
	err   string
	reply string
	// draft is the mapping the model proposed, waiting for the user.
	draft *model.Mapping
}

// snapshot reads what the goroutine may have changed.
func (s *aiState) snapshot() (busy bool, errMsg, reply string, draft *model.Mapping) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy, s.err, s.reply, s.draft
}

// aiDialog draws the AI panel when it is open.
func (v *View) aiDialog(c *ui.Context, t Tokens) {
	if !v.ai.open {
		return
	}
	a := v.App
	busy, errMsg, reply, draft := v.ai.snapshot()
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(620).Gap(12).Padding(20).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "用一句话新建映射").FontSize(16).Bold().TextColor(t.FG)
				ui.Spacer(c)
				ui.Text(c, "AI 只生成草稿，保存前你可以逐项修改").FontSize(11).TextColor(t.Muted)
			})

			ui.Row(c).Gap(8).Children(func() {
				if ui.TextInput(c, &v.ai.prompt).
					Placeholder("把内网 MySQL 192.168.1.10:3306 暴露到 9000").
					Grow(1).Label("描述").Submitted() {
					v.runAI()
				}
				if ui.PrimaryButton(c, "生成").Disabled(busy).Clicked() {
					v.runAI()
				}
			})

			if busy {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c).Label("生成中")
					ui.Text(c, "正在生成草稿…").FontSize(12).TextColor(t.Muted)
				})
			}
			if errMsg != "" {
				ui.Text(c, errMsg).FontSize(12).TextColor(t.Danger)
			}
			if reply != "" {
				ui.Scroll(c).Children(func() {
					markdown.View(c, reply, &v.md)
				}).MaxHeight(200)
			}
			if d := draft; d != nil {
				ui.Column(c).Gap(6).Padding(10).Radius(RadiusChip).
					Background(t.Surface3).Border(1, t.Border).Children(func() {
					ui.Text(c, "草稿").FontSize(11).Bold().TextColor(t.Muted)
					ui.Textf(c, "%s · %s · 监听 %d → %s:%d",
						d.Name, d.Protocol.Label(), d.ListenPort, d.UpstreamHost, d.UpstreamPort).
						FontSize(12).TextColor(t.FG)
					if d.ExtraDirectives != "" {
						ui.Text(c, d.ExtraDirectives).Font("monospace").FontSize(11).TextColor(t.Muted)
					}
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					v.ai.open = false
				}
				if draft != nil {
					if ui.Button(c, "在表单里改").Clicked() || ui.PrimaryButton(c, "采用草稿").Clicked() {
						m := *draft
						v.ai.open = false
						a.OpenNewFormFrom(m)
						a.Info("草稿已填入表单，确认后保存")
					}
				}
			})
		})
	})
	if !open {
		v.ai.open = false
	}
}

// runAI asks the model for a draft from the user's sentence.
func (v *View) runAI() {
	busy, _, _, _ := v.ai.snapshot()
	if v.ai.prompt == "" || busy {
		return
	}
	a := v.App
	v.ai.mu.Lock()
	v.ai.busy, v.ai.err, v.ai.reply, v.ai.draft = true, "", "", nil
	v.ai.mu.Unlock()
	prompt := v.ai.prompt

	go func() {
		reply, draft, err := a.AskAI(prompt)
		v.ai.mu.Lock()
		v.ai.busy = false
		v.ai.reply = reply
		if err != nil {
			v.ai.err = err.Error()
		} else {
			v.ai.draft = draft
		}
		v.ai.mu.Unlock()
		a.Change()
	}()
}

// aiEntry draws the button that opens the AI panel, when AI is configured.
func (v *View) aiEntry(c *ui.Context, t Tokens) {
	if !v.App.AIReady() {
		return
	}
	if ui.Button(c, "✨ AI 建映射").Tooltip("用一句话描述，由模型生成草稿").Clicked() {
		v.ai.open = true
	}
}
