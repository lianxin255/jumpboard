package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/ui/icons"
)

// The window bar: the one header every page mounts under, in place of the
// title rows the pages each used to draw. It says where the user is — the
// page's mark, its name, one line of what it is for — and carries the
// actions that belong to the window rather than the page: the engine's
// state and, on the pages that can apply, 预检 and 应用配置. A page that
// has an action of its own (the chat's 清空对话) hands it to the same
// trailing slot, so no page grows a second title row.

// headerHeight is the bar's height. It is chrome, not content: tall enough
// for the 20px title over its caption, short enough that the page below
// keeps the window.
const headerHeight = float32(56)

// pageSpec is what the window bar says about the page showing.
type pageSpec struct {
	icon     *ui.SVG
	title    string
	subtitle string
	// apply: the page can apply a configuration, so the bar carries the
	// engine's state and the 预检 / 应用配置 pair.
	apply bool
}

// pageSpecFor is the spec of the page showing. The mappings page's
// subtitle is its rule count, the one number the page is about.
func (v *View) pageSpecFor() pageSpec {
	a := v.App
	switch a.Page() {
	case appt.PageLogs:
		return pageSpec{icon: icons.Logs, title: "日志",
			subtitle: "操作记录与内置引擎的访问日志", apply: true}
	case appt.PageHistory:
		return pageSpec{icon: icons.History, title: "配置历史",
			subtitle: "每次成功应用存一份快照，回滚立即生效", apply: true}
	case appt.PageTerminal:
		return pageSpec{icon: icons.Terminal, title: "终端",
			subtitle: "本地 shell、转发端口探测与串口，各开一个标签"}
	case appt.PageAIChat:
		return pageSpec{icon: icons.Chat, title: "AI 对话",
			subtitle: "聊端口映射，聊出来的配置确认后导入当前场景"}
	case appt.PageSettings:
		return pageSpec{icon: icons.Settings, title: "设置",
			subtitle: "AI 配置、外观、访问控制与数据管理"}
	default:
		mappings, _, _, _, _, _, _, _ := a.Snapshot()
		sub := "把内网服务端口从这台机器转出去"
		if len(mappings) > 0 {
			enabled := 0
			for _, m := range mappings {
				if m.Enabled {
					enabled++
				}
			}
			sub = uiCountLine(len(mappings), enabled)
		}
		return pageSpec{icon: icons.Relay, title: "映射", subtitle: sub, apply: true}
	}
}

// uiCountLine is the mappings subtitle's count, the one place the page's
// size is said.
func uiCountLine(total, enabled int) string {
	return fmt.Sprintf("%d 条规则 · %d 启用 · %d 停用", total, enabled, total-enabled)
}

// windowHeader draws the bar and mounts nothing else: the page below is
// the caller's. The trailing actions come from the page's spec and the
// trailing hook a page with its own action passes — the chat page's
// 清空对话 — so every action the window offers sits in this one row.
func (v *View) windowHeader(c *ui.Context, t Tokens, trailing func(c *ui.Context, t Tokens)) {
	a := v.App
	spec := v.pageSpecFor()
	_, _, _, _, engine, _, busy, _ := a.Snapshot()

	ui.Row(c).Height(headerHeight).Shrink(0).AlignItems(ui.Center).
		Padding(0, PagePadX).BorderWidth(0, 0, 1, 0).BorderColor(t.Hair).Children(func() {
		// The mark and the name are the window's own furniture: dragging
		// them moves the window, as a title bar does.
		head := ui.Row(c).Gap(12).AlignItems(ui.Center).DragWindow()
		head.Children(func() {
			ui.Box(c).Size(30, 30).Radius(RadiusChip).
				Background(t.Accent.Alpha(0.10)).Border(1, t.Accent.Alpha(0.24)).Children(func() {
				ui.Column(c).Fill().Center().Children(func() {
					ui.Icon(c, spec.icon).Size(16, 16).TextColor(t.Accent)
				})
			})
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, spec.title).FontSize(TitleSize).Bold().TextColor(t.FG)
				ui.Text(c, spec.subtitle).FontSize(CaptionSize).TextColor(t.Meta).SingleLine()
			})
		})
		ui.Spacer(c).DragWindow()

		if spec.apply {
			v.headerApplyActions(c, t, engine, busy)
		}
		if trailing != nil {
			trailing(c, t)
		}
	})
}

// headerApplyActions is the trailing pair the applying pages carry: the
// engine's state with what is listening, then 预检 and 应用配置. The
// engine's own control — start and stop — stays in the rail's foot, where
// it has always been; this states, it does not act on the engine.
func (v *View) headerApplyActions(c *ui.Context, t Tokens, engine string, busy bool) {
	a := v.App
	rules := a.RuleStatus()
	listening, conns := 0, int64(0)
	for _, r := range rules {
		if r.Listening {
			listening++
			conns += r.Connections
		}
	}
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		v.engineStatus(c, t, engine)
		if len(rules) > 0 {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.Activity).Size(12, 12).TextColor(t.Meta)
				ui.Textf(c, "监听 %d/%d · %d 连接", listening, len(rules), conns).
					FontSize(NoteSize).TextColor(t.Meta).FontFeatures("tnum")
			})
		}
		if busy {
			ui.Spinner(c).Label("正在处理")
		}
		// The check binds each port for a moment and resolves the
		// upstreams: it answers "would applying work", without touching
		// what is running.
		if ui.Button(c, "预检").Disabled(busy).
			Tooltip("检查端口是否被占用、上游地址能否解析，不会改动正在跑的规则").Clicked() {
			a.RunBackground(a.TestConfig)
		}
		if ui.PrimaryButton(c, "应用配置").Disabled(busy).
			Tooltip("让改动生效（⌘R）").Clicked() {
			a.RunBackground(a.Apply)
		}
	})
}
