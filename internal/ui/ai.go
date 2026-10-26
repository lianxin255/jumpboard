package ui

import (
	"strconv"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// aiState is the AI panel's own state.
//
// open and prompt are the main thread's: only a frame or an action a frame
// ran touches them. busy, err, note and drafts are written by the request
// goroutine and read by every frame, so they live behind mu, and the frame
// takes one snapshot instead of holding the lock while it draws.
type aiState struct {
	open   bool
	prompt string

	mu   sync.Mutex
	busy bool
	err  string
	// note says what was dropped or capped, in plain text.
	note string
	// drafts are the mappings the model proposed, waiting for the user.
	drafts []appt.AIDraft
}

// snapshot reads what the goroutine may have changed.
func (s *aiState) snapshot() (busy bool, errMsg, note string, drafts []appt.AIDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy, s.err, s.note, s.drafts
}

// aiDialog draws the AI panel when it is open.
func (v *View) aiDialog(c *ui.Context, t Tokens) {
	if !v.ai.open {
		return
	}
	busy, errMsg, note, drafts := v.ai.snapshot()
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
					Placeholder("把内网 MySQL 和 Redis 分别暴露到 9000/9001").
					Grow(1).Label("描述").Submitted() {
					v.runAI()
				}
				if ui.PrimaryButton(c, "生成").Disabled(busy).Clicked() {
					v.runAI()
				}
			})

			// The panel also reads a file the user picks — an external
			// nginx's error.log, a hand-written conf — and explains it
			// read-only, on the card below the picker's row.
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "解读文件…").
					Tooltip("选一份 .conf / .log / .txt 交给模型解读，比如外部 nginx 的 error.log").Clicked() {
					v.interpretFilePicker()
				}
			})
			v.interpretSection(c, t, "panel")

			if busy {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c).Label("生成中")
					ui.Text(c, "正在生成草稿…").FontSize(12).TextColor(t.Muted)
				})
			}
			if errMsg != "" {
				ui.Text(c, errMsg).FontSize(12).TextColor(t.Danger)
			}
			if note != "" {
				ui.Text(c, note).FontSize(11).TextColor(t.Muted).Wrap()
			}
			if len(drafts) > 0 {
				ui.Textf(c, "草稿（%d 条）", len(drafts)).FontSize(11).Bold().TextColor(t.Muted)
				for i, d := range drafts {
					v.draftCard(c, t, i, d)
				}
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					v.ai.open = false
				}
			})
		})
	})
	if !open {
		v.ai.open = false
	}
}

// draftCard draws one proposed mapping with its two actions. 采用 takes
// the card out of the list and opens the ordinary form prefilled — saving
// still goes through the validation a hand-typed mapping takes — and
// 丢弃 removes just this card, leaving the others for their own decision.
func (v *View) draftCard(c *ui.Context, t Tokens, i int, d appt.AIDraft) {
	a := v.App
	m := d.Mapping
	listen := "待定"
	if m.ListenPort != 0 {
		listen = strconv.Itoa(m.ListenPort)
	}
	ui.Column(c).Gap(6).Padding(10).Radius(RadiusChip).
		Background(t.Surface3).Border(1, t.Border).Children(func() {
		ui.Textf(c, "%s · %s · 监听 %s → %s:%d",
			m.Name, m.Protocol.Label(), listen, m.UpstreamHost, m.UpstreamPort).
			FontSize(12).TextColor(t.FG)
		if d.Explanation != "" {
			ui.Text(c, d.Explanation).FontSize(11).TextColor(t.Muted).Wrap()
		}
		if m.ExtraDirectives != "" {
			ui.Text(c, m.ExtraDirectives).Font("monospace").FontSize(11).TextColor(t.Muted)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "丢弃").Clicked() {
				v.dropDraft(i)
			}
			if ui.PrimaryButton(c, "采用").Clicked() {
				v.dropDraft(i)
				v.ai.open = false
				a.OpenNewFormFrom(m)
				a.Info("草稿已填入表单，确认后保存")
			}
		})
	})
}

// dropDraft removes one card's draft; the rest stay until the user
// decides about them too. The slice is rebuilt rather than shifted in
// place — the frame holds a snapshot that shares the backing array, and a
// shift would redraw its tail as duplicates for one frame. An index the
// list has outgrown is a no-op: another click may have already removed it.
func (v *View) dropDraft(i int) {
	v.ai.mu.Lock()
	if i >= 0 && i < len(v.ai.drafts) {
		next := make([]appt.AIDraft, 0, len(v.ai.drafts)-1)
		next = append(next, v.ai.drafts[:i]...)
		next = append(next, v.ai.drafts[i+1:]...)
		v.ai.drafts = next
	}
	v.ai.mu.Unlock()
	v.App.Change()
}

// runAI asks the model for drafts from the user's sentence.
func (v *View) runAI() {
	busy, _, _, _ := v.ai.snapshot()
	if v.ai.prompt == "" || busy {
		return
	}
	a := v.App
	v.ai.mu.Lock()
	v.ai.busy, v.ai.err, v.ai.note, v.ai.drafts = true, "", "", nil
	v.ai.mu.Unlock()
	prompt := v.ai.prompt

	go func() {
		note, drafts, err := a.AskAIMulti(prompt)
		v.ai.mu.Lock()
		v.ai.busy = false
		v.ai.note = note
		if err != nil {
			v.ai.err = err.Error()
		} else {
			v.ai.drafts = drafts
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
	if ui.Button(c, "✨ AI 建映射").Tooltip("用一句话描述，可一次生成多条草稿").Clicked() {
		v.ai.open = true
	}
}

// interpretFilePicker asks for one configuration or log and hands it to
// the interpretation on the background goroutine: reading the file is
// disk work no frame should wait on.
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
	if err != nil || len(paths) == 0 {
		return
	}
	path := paths[0]
	a.RunBackground(func() error { return a.InterpretFile("", path) })
}
