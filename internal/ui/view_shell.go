package ui

import (
	"github.com/egoist/mygo/ui"
)

// The window's global chrome, apart from the pages it frames. The window
// bar's own actions — the engine's state, 预检, 应用配置 — live in
// page_header.go, drawn by every page that can apply. What is left here is
// what those actions leave behind: the one conditional notice area the
// applying pages share, so 未应用, an error, a warning and an outside
// edit of the configuration each occupy a content row only while they are
// true. The rail and the engine card it carries still live in view.go,
// next to the pages.

// pageNotices is the applying pages' one conditional block: the amber
// pending strip, an apply failure (with the diagnosis that can follow it),
// a warning, and the outside edit of the exported configuration. Nothing
// here draws unless it is true, so a calm page is only its tools and its
// table.
func (v *View) pageNotices(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	if a.Dirty() {
		v.pendingBar(c, t)
	}
	if msg := a.ApplyError(); msg != "" {
		fill, line := statusTint(t.Danger)
		ui.Box(c).FillWidth().Padding(8, 12).Radius(RadiusChip).
			Background(fill).Border(1, line).Children(func() {
			ui.Text(c, msg).FontSize(CaptionSize).TextColor(t.Danger).Selectable()
		})
		v.diagSection(c, t, busy)
	}
	if msg := a.ApplyWarning(); msg != "" {
		fill, line := statusTint(t.Warning)
		ui.Box(c).FillWidth().Padding(8, 12).Radius(RadiusChip).
			Background(fill).Border(1, line).Children(func() {
			ui.Text(c, msg).FontSize(CaptionSize).TextColor(t.Warning).Selectable()
		})
	}
	if a.ConfChanged() {
		v.confChangedBar(c, t, busy)
	}
}

// engineLabel is the engine state as a word, the rail's foot and the
// window bar's lead both say it.
func engineLabel(s string) string {
	if s == "running" {
		return "运行中"
	}
	return "已停止"
}

// engineStatus is the engine's state as the window bar leads with it:
// the same dot and word the rail's foot carries, so the page states its
// engine without the eye going to the rail. It is text, not a control —
// the rail's foot keeps the start and stop buttons.
func (v *View) engineStatus(c *ui.Context, t Tokens, engine string) {
	dot := t.Success
	if engine != "running" {
		dot = t.Muted
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(7, 7).Radius(4).Background(dot)
		ui.Text(c, "引擎 "+engineLabel(engine)).FontSize(NoteSize).Bold().TextColor(t.FG2)
	})
}

// confChangedBar reports that the exported configuration was edited outside
// the app. It offers to merge the file in — the same path a dropped .conf
// takes — or to leave it alone. It never reloads on its own: the file and
// the rules disagree, and the person decides which one is right.
func (v *View) confChangedBar(c *ui.Context, t Tokens, busy bool) {
	a := v.App
	fill, line := statusTint(t.Warning)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(8, 12).
		Radius(RadiusChip).
		Background(fill).Border(1, line).Children(func() {
		ui.Text(c, "配置文件被外部修改").FontSize(NoteSize).TextColor(t.Warning).Grow(1)
		if ui.Button(c, "导入合并").FontSize(CaptionSize).Disabled(busy).Clicked() {
			a.RunBackground(a.ImportChangedConf)
		}
		if ui.Button(c, "忽略").FontSize(CaptionSize).Clicked() {
			a.DismissConfChange()
		}
	})
}

// pendingBar is the amber strip for a configuration that has not been
// applied yet. It stays in the page flow so a short window does not cover
// the rows, and it says 未应用 because that is the word the tests look
// for. It carries no button of its own: the window bar above holds the
// one 应用配置, and the bar only states what is pending.
func (v *View) pendingBar(c *ui.Context, t Tokens) {
	ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MaxWidth(840).Gap(12).AlignItems(ui.Center).
			Padding(8, 10, 8, 16).Radius(RadiusFloat).
			Background(t.AmberSoft).Border(1, t.AmberLine).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(t.Warning)
			ui.Text(c, "有未生效的改动").FontSize(NoteSize).Bold().TextColor(t.AmberText)
			ui.Text(c, "未应用 · 应用配置后对外生效").FontSize(CaptionSize).TextColor(t.AmberText)
		})
	})
}
