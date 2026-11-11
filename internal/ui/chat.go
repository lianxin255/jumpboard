// The AI chat page: the conversation as bubbles over a list that follows
// the newest message, the drafts a reply proposed as rule cards under
// it, and the import that turns them into mappings in the current scene.
// This is the one place a mapping draft is proposed — the mappings
// page's ✨ entry is a door here, not a panel of its own.
package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/ui/icons"
)

// chatItem is one row of the page's list: a turn, the busy line, the
// error, or one piece of the preview section. The list is walked by
// index over a slice built per frame, the way a list of one kind would
// be; the kinds only differ in how they draw.
type chatItem struct {
	kind  int // 0 turn, 1 busy, 2 error, 3 preview head, 4 draft card, 5 import bar, 6 lone note
	turn  ai.ChatTurn
	draft appt.AIDraft
	index int // the draft's place in the waiting list
}

// aiChatPage draws the conversation page.
//
// The page is three bands: the header, the conversation's scroll, and the
// composer. The scroll grows into whatever room is left and the composer
// keeps its natural height — the conversation can never push the field or
// the send button off the bottom of the window, not even at the app's
// minimum size, because it scrolls inside its own bounded panel instead of
// making the page taller.
func (v *View) aiChatPage(c *ui.Context, t Tokens) {
	a := v.App
	turns, partial, streaming, busy, errText, note, drafts := a.AIChatState()
	// A pending edit is valid only for the scene and mapping version it
	// was proposed against. Compute that before drawing the cards so a
	// changed or deleted target is visibly blocked, never a silent write.
	for i := range drafts {
		if drafts[i].Operation != "update" || drafts[i].Original == nil {
			continue
		}
		current, err := a.Store.GetMapping(drafts[i].TargetID)
		if err != nil {
			drafts[i].Conflict = "原规则已不存在，请重新生成草稿，避免覆盖你的改动。"
			continue
		}
		if current.UpdatedAt != drafts[i].Original.UpdatedAt {
			drafts[i].Conflict = "原规则已在对话后被修改，请重新生成草稿，避免覆盖你的改动。"
		}
	}

	// Grow, not Fill: the page shares a column with the window bar above
	// it, and a Fill page claims the column's whole height — which puts
	// the page's own bottom, and the composer riding it, below the
	// window's edge no matter how the conversation inside is laid out.
	// Grow takes the room the bar leaves, so the page's bottom is the
	// window's.
	ui.Column(c).Grow(1).MinWidth(0).Padding(PagePadY, PagePadX).Gap(12).Children(func() {
		// The window bar carries the page's name and its 清空对话 action,
		// so the conversation starts with itself.

		// Without an endpoint and a model the page is a door that leads
		// to the settings, not a chat that would fail on every send.
		if !a.AIReady() {
			v.chatNotReady(c, t)
			return
		}

		// The plan's two columns: the conversation on the left, the
		// drafts a reply proposed on the right, so what the model said and
		// what it proposes to write are read side by side — the drafts no
		// longer scroll away under later turns. The right column shows
		// only while there is something to review; the conversation takes
		// the room back when there is not. A narrow window keeps the one
		// column the page has always had, the preview riding the list.
		w, _ := c.Size()
		if len(drafts) == 0 || w < chatTwoColumnW {
			v.chatList(c, t, turns, partial, streaming, busy, errText, note, drafts)
			v.chatComposer(c, t, busy)
			return
		}

		// Stretch: both columns take the page's whole height, so the
		// conversation's panel and the draft column end on the same line
		// — the composer rides the bottom of the page, not the middle.
		ui.Row(c).Grow(1).MinWidth(0).Gap(16).AlignItems(ui.Stretch).Children(func() {
			// The conversation's column carries the panel and the composer;
			// the panel grows into the room the composer leaves, which is
			// what keeps the scroll bounded and the field on screen.
			ui.Column(c).Grow(1).MinWidth(0).Gap(12).Children(func() {
				v.chatList(c, t, turns, partial, streaming, busy, errText, note, nil)
				v.chatComposer(c, t, busy)
			})
			v.chatDraftColumn(c, t, drafts, note, busy)
		})
	})
}

// chatTwoColumnW is the window width from which the chat page splits into
// the conversation and the draft column. Below it the two would each be
// too narrow to read an address on, so the preview stays in the list.
const chatTwoColumnW = float32(1120)

// chatDraftColumn is the right column: the drafts waiting for a decision,
// their head, and the one import that writes them into the current scene.
// It scrolls on its own, so a five-card reply never pushes the import out
// of reach — the fix the in-list preview needed and never had.
func (v *View) chatDraftColumn(c *ui.Context, t Tokens, drafts []appt.AIDraft, note string, busy bool) {
	blocked := false
	for _, draft := range drafts {
		if draft.Conflict != "" {
			blocked = true
			break
		}
	}
	ui.Column(c).Width(360).Shrink(0).Gap(10).Children(func() {
		panelShell(c, t).Grow(1).MinHeight(200).Children(func() {
			ui.Column(c).Fill().Children(func() {
				// The column's head says what the cards are and what they
				// are not: the same words the in-list preview carries, in
				// the one place this column has for them.
				ui.Column(c).Gap(2).Padding(12, 14).
					BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Textf(c, "待审草稿（%d 条）", len(drafts)).FontSize(BodySize).Bold().TextColor(t.FG)
					})
					ui.Text(c, "审阅、修改或丢弃，再导入当前场景").
						FontSize(CaptionSize).TextColor(t.Muted)
				})
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Gap(8).Padding(10).Children(func() {
						for i, d := range drafts {
							v.chatDraftCard(c, t, d, i, busy)
						}
					})
				})
				// The foot: the note the reply carried, and the one import.
				ui.Column(c).Gap(6).Padding(10, 12).
					BorderWidth(1, 0, 0, 0).BorderColor(t.Hair).Children(func() {
					if note != "" {
						ui.Text(c, note).FontSize(CaptionSize).TextColor(t.Muted).Wrap()
					}
					ui.Textf(c, "导入到当前场景「%s」", v.App.ActiveWorkspaceName()).
						FontSize(CaptionSize).TextColor(t.Muted)
					if ui.PrimaryButton(c, "导入到当前场景").Disabled(busy || blocked).FillWidth().
						Tooltip("把上面的草稿写入当前场景，走与手动新建相同的校验；之后点「应用配置」生效").Clicked() {
						n := len(drafts)
						v.App.Ask(&appt.Confirm{
							Title: "导入映射",
							Body:  "把 " + strconv.Itoa(n) + " 条草稿写入当前场景，端口冲突会整批拒绝。导入后点「应用配置」才会生效。",
							Label: "导入",
							Run:   v.App.ImportChatDrafts,
						})
					}
				})
			})
		})
	})
}

// chatNotReady is the page a user without AI configured lands on: what
// is missing, and the one button that goes where it is fixed. The card
// the settings open on is the AI one, so the landing needs no scroll.
func (v *View) chatNotReady(c *ui.Context, t Tokens) {
	ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
		emptyCard(c, t, 460).Children(func() {
			ui.Column(c).Gap(12).AlignItems(ui.Center).Children(func() {
				emptyGlyph(c, t, "✨")
				ui.Text(c, "还没有配置 AI 服务").FontSize(HeadSize).Bold().TextColor(t.FG)
				ui.Text(c, "填好接口地址和模型后，这里可以和模型对话生成映射配置。").
					FontSize(BodySize).TextColor(t.Muted)
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
	} else if note != "" {
		// A reply whose every draft was dropped leaves only its note; it
		// rides the list as its own line rather than inside a preview
		// head that would have nothing under it.
		items = append(items, chatItem{kind: 6})
	}

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

	// The panel grows into the room the header and the composer leave —
	// Grow, not Fill: a Fill panel claims the page's whole height and then
	// bargains it back from its siblings, which is how a long conversation
	// once pushed the composer flush against the window's edge. The floor
	// keeps the scroll itself usable in a window at its shortest.
	panelShell(c, t).Grow(1).MinHeight(160).Children(func() {
		ui.Column(c).Fill().Children(func() {
			// The panel's own heading, the plan's: it says what the column
			// is and what the assistant will not do — draft, never write.
			ui.Column(c).Gap(2).Padding(12, 16).
				BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "对话").FontSize(BodySize).Bold().TextColor(t.FG)
					ui.Spacer(c)
					if len(turns) > 0 {
						if ui.Button(c, "清空对话").Disabled(busy).Height(ControlHeight).
							Tooltip("只清当前场景对话和待审草稿，已保存映射不受影响").Clicked() {
							v.App.Ask(&appt.Confirm{
								Title: "清空当前场景对话",
								Body:  "仅清除「" + v.App.ActiveWorkspaceName() + "」的对话和待审草稿，已保存的映射、其他场景和配置历史不受影响。",
								Label: "清空对话",
								Run: func() error {
									v.App.ClearChat()
									return nil
								},
							})
						}
					}
				})
				ui.Textf(c, "当前场景「%s」 · 本地保存最近 50 轮 · 发送最近 %d 轮", v.App.ActiveWorkspaceName(), v.App.AIChatTurns()).
					FontSize(CaptionSize).TextColor(t.Muted)
			})
			ui.List(c, &v.chatListState, len(items), func(i int) {
				v.chatRow(c, t, items[i], errText, note, drafts, busy, streaming, partial)
			}).Grow(1).Padding(14, 16).Gap(12).Justify(ui.End).Children(func() {
				if len(items) == 0 {
					ui.Text(c, "说说你想转发的服务，比如：把内网 MySQL 和 Redis 分别暴露到 9000/9001").
						FontSize(NoteSize).TextColor(t.Muted).Margin(ui.Auto)
				}
			})
		})
	})
}

// chatRow draws one item of the conversation's list. The partial reply
// rides the list as the last assistant turn (see chatList), so streaming
// says whether that turn is still on its way.
func (v *View) chatRow(c *ui.Context, t Tokens, it chatItem, errText, note string, drafts []appt.AIDraft, busy, streaming bool, partial string) {
	switch it.kind {
	case 0:
		if it.turn.Role == ai.RoleUser {
			ui.Row(c).Children(func() {
				ui.Spacer(c)
				ui.Column(c).Gap(4).AlignItems(ui.End).Children(func() {
					ui.Text(c, it.turn.Content).Selectable().Padding(8, 12).Radius(RadiusChip).
						Background(t.Accent.Alpha(0.10)).TextColor(t.FG).MaxWidth(520)
					if ui.Text(c, "复制").FontSize(CaptionSize).TextColor(t.Muted).
						Label("复制问题").Tooltip("复制这条原问题").Clicked() {
						_ = v.App.CopyText(it.turn.Content)
					}
				})
			})
			return
		}
		ui.Column(c).MaxWidth(760).Gap(6).Children(func() {
			// The mygo chat shape: the reply on its way renders with
			// markdown.Stream, which paces what arrived and completes
			// half-written syntax; a finished turn with markdown.View,
			// whose parse carries on.
			if streaming && it.turn.Content == partial {
				markdown.Stream(c, ai.WithoutDraftBlocks(it.turn.Content), &markdown.Options{})
			} else {
				markdown.View(c, ai.WithoutDraftBlocks(it.turn.Content), &markdown.Options{})
			}
			if it.turn.Content != "" && ai.WithoutDraftBlocks(it.turn.Content) == "" {
				// This text belongs to its own historical turn. Never point at
				// the global waiting list, which may belong to a later reply.
				ui.Text(c, "（这条回复只有配置草稿）").FontSize(CaptionSize).TextColor(t.Muted)
			}
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				if ui.Text(c, "复制").FontSize(CaptionSize).TextColor(t.Muted).
					Label("复制回复").Tooltip("只复制可读正文，不包含草稿 JSON").Clicked() {
					_ = v.App.CopyText(ai.WithoutDraftBlocks(it.turn.Content))
				}
			})
		})
	case 1:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("回复中")
			ui.Text(c, "正在回复…").FontSize(NoteSize).TextColor(t.Muted)
		})
	case 2:
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, errText).FontSize(NoteSize).TextColor(t.Danger).Selectable().Wrap().Grow(1)
			if ui.Button(c, "重试").Disabled(busy).Height(ControlHeight).
				Tooltip("用原问题重试，不重复添加用户消息").Clicked() {
				v.App.RetryChat()
			}
		})
	case 3:
		// The preview's head carries the workbench's own mark, so what
		// follows reads as rules waiting for a decision rather than as
		// more chat text.
		ui.Column(c).Gap(4).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).MaxWidth(760).Children(func() {
				ui.Box(c).Size(28, 28).Radius(RadiusButton).
					Background(t.Accent.Alpha(0.14)).Border(1, t.Accent.Alpha(0.28)).Children(func() {
					ui.Column(c).Fill().Center().Children(func() {
						ui.Icon(c, icons.Connector).Size(18, 11).TextColor(t.Accent)
					})
				})
				ui.Column(c).Gap(2).Grow(1).Children(func() {
					ui.Textf(c, "待审草稿（%d 条）", len(drafts)).FontSize(NoteSize).Bold().TextColor(t.FG)
					ui.Text(c, "AI 按描述生成的转发规则：丢弃不要的，把单条填进表单改，或整批导入当前场景").
						FontSize(CaptionSize).TextColor(t.Muted).Wrap()
				})
			})
			ui.Text(c, "草稿不会自动生效；确认导入后仍需点「应用配置」").FontSize(CaptionSize).TextColor(t.Muted)
		})
	case 4:
		v.chatDraftCard(c, t, it.draft, it.index, busy)
	case 5:
		v.chatImportBar(c, t, drafts, note, busy)
	case 6:
		// The reply's note with no preview to carry it: what was dropped,
		// or what the cap cut, said on the line the cards would have led.
		ui.Text(c, note).FontSize(CaptionSize).TextColor(t.Muted).Wrap().MaxWidth(760)
	}
}

// chatDraftCard draws one proposed mapping as the rule it would become,
// in the workbench's own shape rather than a line of text: the name with
// the protocol's badge and the two actions on the head row, then the
// forwarding relation the table's rows carry — a mono 监听 chip, the
// connector mark, the upstream — with the model's explanation as the
// secondary line under it. 丢弃 removes just this card; 填入表单 opens
// the ordinary form prefilled, the escape hatch for a draft that wants
// editing rather than importing. The actions ride the head row instead
// of a row of their own: the preview must fit a small window's scroll
// without pushing the composer off screen.
func (v *View) chatDraftCard(c *ui.Context, t Tokens, d appt.AIDraft, i int, busy bool) {
	a := v.App
	m := d.Mapping
	listen, defaultPort := localListenLabel(m)
	draftCardShell(c, t).MaxWidth(760).Children(func() {
		// The rule's head: its name, the protocol's classification patch
		// — the same badge the table's proto column draws — and the two
		// decisions a draft waits for, on the right.
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.Name).FontSize(BodySize).Bold().TextColor(t.FG).SingleLine().Shrink(1)
			if d.Operation == "update" {
				ui.Text(c, "修改").FontSize(CaptionSize).TextColor(t.Accent).Shrink(0).
					Padding(2, 5).Radius(RadiusChip).Background(t.AccentSoft)
			} else {
				ui.Text(c, "新增").FontSize(CaptionSize).TextColor(t.Success).Shrink(0).
					Padding(2, 5).Radius(RadiusChip).Background(t.SuccessSoft)
			}
			v.protoCell(c, t, m)
			ui.Spacer(c)
			if DangerQuietButton(c, t, "丢弃").FontSize(CaptionSize).
				Label("丢弃草稿 " + m.Name).Clicked() {
				a.DropChatDraft(i)
			}
			if ui.Button(c, "填入表单").FontSize(CaptionSize).Disabled(d.Conflict != "").
				Label("填入表单 " + m.Name).
				Tooltip("把这条草稿填进映射表单，逐项修改后保存").Clicked() {
				a.DropChatDraft(i)
				a.OpenDraftForm(d)
				a.Info("草稿已填入表单，确认后保存")
			}
		})
		if d.Conflict != "" {
			ui.Text(c, d.Conflict).FontSize(CaptionSize).TextColor(t.Danger).Wrap()
		}
		// The forwarding relation: 监听 :端口 on the left, the upstream on
		// the right, the connector mark and its arrow between — the row
		// this draft would join, drawn before it exists.
		if d.Operation == "update" && d.Original != nil {
			v.chatDraftDiff(c, t, d)
		} else {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, listen).Font(FontMono).FontSize(NoteSize).
					TextColor(t.FG).Padding(2, 8).Radius(RadiusChip).
					Background(t.Chip).Border(1, t.Border).Tooltip("生效后在本机监听的地址")
				if defaultPort {
					ui.Text(c, "（默认）").FontSize(CaptionSize).TextColor(t.Meta).NoWrap().
						Tooltip("监听端口留空时，Web 规则用默认端口：HTTP 80，HTTPS 443")
				}
				ui.Icon(c, icons.Connector).Size(20, 12).TextColor(t.Meta).
					Label("转发-" + m.Name).Tooltip("本地端口转发到上游")
				ui.Text(c, "→").FontSize(NoteSize).TextColor(t.Meta)
				ui.Textf(c, "%s:%d", m.UpstreamHost, m.UpstreamPort).Font(FontMono).
					FontSize(NoteSize).TextColor(t.FG2).SingleLine().Tooltip("上游目标")
			})
		}
		if d.Explanation != "" {
			ui.Text(c, d.Explanation).FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		}
		if m.ExtraDirectives != "" {
			ui.Text(c, m.ExtraDirectives).Font(FontMono).FontSize(CaptionSize).TextColor(t.Muted)
		}
	})
}

// hasDraftConflicts reports whether the batch contains a stale edit.
func hasDraftConflicts(drafts []appt.AIDraft) bool {
	for _, draft := range drafts {
		if draft.Conflict != "" {
			return true
		}
	}
	return false
}

// chatDraftDiff shows only fields that actually change. Unchanged fields
// stay out of the card, so an edit cannot masquerade as a new mapping or
// hide a port change in a full configuration dump.
func (v *View) chatDraftDiff(c *ui.Context, t Tokens, d appt.AIDraft) {
	before, after := *d.Original, d.Mapping
	fields := []struct {
		name string
		old  string
		new  string
	}{
		{"名称", before.Name, after.Name},
		{"分组", before.Group, after.Group},
		{"启用状态", enabledLabel(before.Enabled), enabledLabel(after.Enabled)},
		{"协议", before.Protocol.Label(), after.Protocol.Label()},
		{"监听端口", listenAddress(before), listenAddress(after)},
		{"域名", before.ServerNames, after.ServerNames},
		{"路径", before.ListenPath, after.ListenPath},
		{"上游地址", upstreamAddress(before), upstreamAddress(after)},
		// The proxy mode decides whether the custom headers are sent at
		// all, and the extra directives carry the timeouts: a draft that
		// changes either would otherwise look unchanged on the card and
		// silently drop the rule's authentication on import.
		{"代理模式", before.ProxyMode.Label(), after.ProxyMode.Label()},
		{"附加指令", before.ExtraDirectives, after.ExtraDirectives},
	}
	for _, field := range fields {
		if field.old == field.new {
			continue
		}
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, field.name).FontSize(CaptionSize).TextColor(t.Muted)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, diffValue(field.old)).Font(FontMono).FontSize(CaptionSize).TextColor(t.Muted)
				ui.Text(c, "→").FontSize(CaptionSize).TextColor(t.Meta)
				ui.Text(c, diffValue(field.new)).Font(FontMono).FontSize(CaptionSize).TextColor(t.Accent)
			})
		})
	}
}

// diffValue shows an empty side of a change as (空) so the arrow still
// reads as "this became that" rather than pointing at nothing.
func diffValue(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(空)"
	}
	return s
}

func enabledLabel(on bool) string {
	if on {
		return "已启用"
	}
	return "已停用"
}

func listenAddress(m model.Mapping) string {
	listen, _ := localListenLabel(m)
	return listen
}

func upstreamAddress(m model.Mapping) string {
	return fmt.Sprintf("%s:%d", m.UpstreamHost, m.UpstreamPort)
}

// chatImportBar is the preview section's foot: the one button that
// writes every waiting draft into the current scene, behind the
// confirmation an import deserves.
//
// The note rides here, at the section's tail rather than under its head:
// the list follows its end, so the tail is the part the eye lands on — a
// cap's cut or a dropped draft said only in the head would sit above the
// fold of a five-card reply, read by no one.
func (v *View) chatImportBar(c *ui.Context, t Tokens, drafts []appt.AIDraft, note string, busy bool) {
	a := v.App
	ui.Column(c).Gap(6).MaxWidth(760).Children(func() {
		if note != "" {
			ui.Text(c, note).FontSize(CaptionSize).TextColor(t.Muted).Wrap()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "导入到当前场景「%s」", a.ActiveWorkspaceName()).FontSize(CaptionSize).TextColor(t.Muted).Grow(1)
			if ui.PrimaryButton(c, "导入到当前场景").Disabled(busy || hasDraftConflicts(drafts)).
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

// chatComposer is the composer: the field and the send button on one row,
// Enter and click both sending, an empty field or a reply on its way
// refusing.
//
// The row is reserved, never shared: it keeps its natural height
// (Shrink(0)), so the conversation above scrolls inside its own panel
// rather than stacking the composer under a taller page — which is what
// happened when the field and the button sat in the page's column
// directly, the field stretched full width and the button full width
// beneath it, flush with the window's bottom edge.
func (v *View) chatComposer(c *ui.Context, t Tokens, busy bool) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		in := ui.TextInput(c, &v.chatInput).
			Placeholder("描述要转发的服务，或接着上一条继续聊").
			Grow(1).Label("消息")
		send := ui.PrimaryButton(c, "发送").Disabled(busy || v.chatInput == "")
		if (in.Submitted() || send.Clicked()) && v.chatInput != "" && !busy {
			v.sendChat()
		}
	})
}
