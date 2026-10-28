// The AI chat page: the conversation as bubbles over a list that follows
// the newest message, the drafts a reply proposed as preview cards under
// it, and the import that turns them into mappings in the current scene.
// The one-sentence panel (aiDialog) stays for what it is good at; this
// page is for the conversation that arrives at a configuration.
package ui

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/appt"
)

// chatItem is one row of the page's list: a turn, the busy line, the
// error, or one piece of the preview section. The list is walked by
// index over a slice built per frame, the way a list of one kind would
// be; the kinds only differ in how they draw.
type chatItem struct {
	kind  int // 0 turn, 1 busy, 2 error, 3 preview head, 4 draft card, 5 import bar
	turn  ai.ChatTurn
	draft appt.AIDraft
	index int // the draft's place in the waiting list
}

// aiChatPage draws the conversation page.
func (v *View) aiChatPage(c *ui.Context, t Tokens) {
	a := v.App
	turns, partial, streaming, busy, errText, note, drafts := a.AIChatState()

	ui.Column(c).Fill().Padding(PagePadY, PagePadX).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, "AI 对话").FontSize(TitleSize).Bold().TextColor(t.FG)
				ui.Text(c, "聊端口映射，聊出来的配置确认后导入当前场景").FontSize(SubtitleSz).TextColor(t.Muted)
			})
			ui.Spacer(c)
			if len(turns) > 0 {
				if ui.Button(c, "清空对话").Disabled(busy).
					Tooltip("清掉这段对话和等待导入的草稿；已导入的映射不受影响").Clicked() {
					a.ClearChat()
				}
			}
		})

		// Without an endpoint and a model the page is a door that leads
		// to the settings, not a chat that would fail on every send.
		if !a.AIReady() {
			v.chatNotReady(c, t)
			return
		}

		v.chatList(c, t, turns, partial, streaming, busy, errText, note, drafts)
		v.chatInputRow(c, t, busy)
	})
}

// chatNotReady is the page a user without AI configured lands on: what
// is missing, and the one button that goes where it is fixed.
func (v *View) chatNotReady(c *ui.Context, t Tokens) {
	ui.Column(c).Fill().Center().Gap(14).Children(func() {
		ui.Box(c).Width(460).Padding(32, 28).Radius(RadiusModal).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Column(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(36, 36).Radius(10).Background(t.Accent.Alpha(0.14)).
					Border(1, t.Accent.Alpha(0.28)).Children(func() {
					ui.Column(c).Fill().Center().Children(func() {
						ui.Text(c, "✨").FontSize(16).TextColor(t.Accent)
					})
				})
				ui.Text(c, "还没有配置 AI 服务").FontSize(18).Bold().TextColor(t.FG)
				ui.Text(c, "填好接口地址和模型后，这里可以和模型对话生成映射配置。").
					FontSize(13).TextColor(t.Muted)
				if ui.PrimaryButton(c, "去设置").Clicked() {
					v.App.SetPage(appt.PageSettings)
				}
			})
		})
	})
}

// chatList draws the conversation. The list follows its end while the
// newest message arrives and lets go when the user scrolls up, the
// toolkit's own chat shape; the busy line, an error, and the preview
// section ride as items so everything the conversation produced stays
// in one scroll.
func (v *View) chatList(c *ui.Context, t Tokens, turns []ai.ChatTurn, partial string,
	streaming, busy bool, errText, note string, drafts []appt.AIDraft) {
	items := make([]chatItem, 0, len(turns)+len(drafts)+3)
	for _, turn := range turns {
		items = append(items, chatItem{kind: 0, turn: turn})
	}
	if busy && partial == "" {
		items = append(items, chatItem{kind: 1})
	}
	if partial != "" {
		items = append(items, chatItem{kind: 0, turn: ai.ChatTurn{Role: ai.RoleAssistant, Content: partial}})
	}
	if errText != "" {
		items = append(items, chatItem{kind: 2})
	}
	if len(drafts) > 0 {
		items = append(items, chatItem{kind: 3})
		for i, d := range drafts {
			items = append(items, chatItem{kind: 4, draft: d, index: i})
		}
		items = append(items, chatItem{kind: 5})
	}
	_ = streaming

	v.chatListState.Key = func(i int) any {
		switch items[i].kind {
		case 0:
			// Turns only append, and the partial reply grows in place of
			// the busy line, so index- keyed rows keep their state.
			return strconv.Itoa(i)
		case 3:
			return "preview-head"
		case 4:
			return "draft-" + strconv.Itoa(items[i].index)
		case 5:
			return "import-bar"
		default:
			return "chat-" + strconv.Itoa(items[i].kind)
		}
	}
	v.chatListState.FollowEnd = true
	v.chatListState.FollowAnimation = &ui.Smooth

	ui.Column(c).Fill().Radius(RadiusPanel).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
		ui.List(c, &v.chatListState, len(items), func(i int) {
			v.chatRow(c, t, items[i], errText, note, drafts, busy)
		}).Grow(1).Padding(14, 16).Gap(12).Justify(ui.End).Children(func() {
			if len(items) == 0 {
				ui.Text(c, "说说你想转发的服务，比如：把内网 MySQL 和 Redis 分别暴露到 9000/9001").
					FontSize(12).TextColor(t.Muted).Margin(ui.Auto)
			}
		})
	})
}

// chatRow draws one item of the conversation's list.
func (v *View) chatRow(c *ui.Context, t Tokens, it chatItem, errText, note string, drafts []appt.AIDraft, busy bool) {
	switch it.kind {
	case 0:
		if it.turn.Role == ai.RoleUser {
			ui.Row(c).Children(func() {
				ui.Spacer(c)
				ui.Text(c, it.turn.Content).Selectable().Padding(8, 12).Radius(RadiusChip).
					Background(t.Accent.Alpha(0.10)).TextColor(t.FG).MaxWidth(520)
			})
			return
		}
		ui.Column(c).MaxWidth(760).Gap(6).Children(func() {
			markdown.View(c, ai.WithoutDraftBlocks(it.turn.Content), &markdown.Options{})
			if it.turn.Content != "" && ai.WithoutDraftBlocks(it.turn.Content) == "" {
				ui.Text(c, "（这条回复只有配置草稿，见下方预览）").FontSize(11).TextColor(t.Muted)
			}
		})
	case 1:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("回复中")
			ui.Text(c, "正在回复…").FontSize(12).TextColor(t.Muted)
		})
	case 2:
		ui.Text(c, errText).FontSize(12).TextColor(t.Danger).Selectable().Wrap()
	case 3:
		ui.Column(c).Gap(2).Children(func() {
			ui.Textf(c, "配置预览（%d 条）", len(drafts)).FontSize(12).Bold().TextColor(t.FG)
			if note != "" {
				ui.Text(c, note).FontSize(11).TextColor(t.Muted).Wrap()
			}
			ui.Text(c, "草稿不会自动生效；确认导入后仍需点「应用配置」").FontSize(11).TextColor(t.Muted)
		})
	case 4:
		v.chatDraftCard(c, t, it.draft, it.index, busy)
	case 5:
		v.chatImportBar(c, t, drafts, busy)
	}
}

// chatDraftCard draws one proposed mapping. 丢弃 removes just this card;
// 采用 opens the ordinary form prefilled, the one-sentence panel's own
// escape hatch for a draft that wants editing rather than importing.
func (v *View) chatDraftCard(c *ui.Context, t Tokens, d appt.AIDraft, i int, busy bool) {
	a := v.App
	m := d.Mapping
	listen := "待定"
	if m.ListenPort != 0 {
		listen = strconv.Itoa(m.ListenPort)
	}
	ui.Column(c).MaxWidth(760).Gap(6).Padding(10).Radius(RadiusChip).
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
			if ui.Button(c, "丢弃").FontSize(11).
				Label("丢弃草稿 " + m.Name).Clicked() {
				a.DropChatDraft(i)
			}
			if ui.Button(c, "填入表单").FontSize(11).
				Label("填入表单 " + m.Name).
				Tooltip("把这条草稿填进新建映射表单，逐项修改后保存").Clicked() {
				a.DropChatDraft(i)
				a.OpenNewFormFrom(m)
				a.Info("草稿已填入表单，确认后保存")
			}
		})
	})
}

// chatImportBar is the preview section's foot: the one button that
// writes every waiting draft into the current scene, behind the
// confirmation an import deserves.
func (v *View) chatImportBar(c *ui.Context, t Tokens, drafts []appt.AIDraft, busy bool) {
	a := v.App
	ui.Row(c).Gap(8).AlignItems(ui.Center).MaxWidth(760).Children(func() {
		ui.Textf(c, "导入到当前场景「%s」", a.ActiveWorkspaceName()).FontSize(11).TextColor(t.Muted).Grow(1)
		if ui.PrimaryButton(c, "导入到当前工作区").Disabled(busy).
			Tooltip("把上面的草稿写入当前场景，走与手动新建相同的校验；之后点「应用配置」生效").Clicked() {
			n := len(drafts)
			a.Ask(&appt.Confirm{
				Title: "导入映射",
				Body:  "把 " + strconv.Itoa(n) + " 条草稿写入当前场景，端口冲突会整批拒绝。导入后点「应用配置」才会生效。",
				Label: "导入",
				Run:   a.ImportChatDrafts,
			})
		}
	})
}

// sendChat hands the composer's text to the app and clears the field.
// The message joins the conversation the app holds; the reply arrives
// in the state the next frames read.
func (v *View) sendChat() {
	text := strings.TrimSpace(v.chatInput)
	if text == "" {
		return
	}
	v.chatInput = ""
	v.App.SendChatMessage(text)
}

// chatInputRow is the composer: the field and the send button, Enter and
// click both sending, an empty field or a reply on its way refusing.
func (v *View) chatInputRow(c *ui.Context, t Tokens, busy bool) {
	in := ui.TextInput(c, &v.chatInput).
		Placeholder("描述要转发的服务，或接着上一条继续聊").
		Grow(1).Label("消息")
	send := ui.PrimaryButton(c, "发送").Disabled(busy || v.chatInput == "")
	if (in.Submitted() || send.Clicked()) && v.chatInput != "" && !busy {
		v.sendChat()
	}
}
