package native

import (
	"runtime"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
)

// flyoutSize is the panel's fixed footprint: the summary and actions
// plus up to recentMax rows. A fixed panel keeps the popover the system
// draws from resizing as the data does.
const (
	flyoutWidth  = 320
	flyoutHeight = 340
	recentMax    = 5
)

// toggleFlyout opens the panel under the tray icon, or closes it if it
// is open. The tray's left click lands here on macOS and Windows; the
// right click keeps the native menu, so browsing is the panel's job and
// quitting stays one right-click away. Linux has no click signal on its
// trays, so the menu is permanent there and the panel opens from its
// first item.
func (d *Dock) toggleFlyout() {
	if d.panel != nil {
		d.panel.Close()
		return
	}
	d.panel = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:      d.tray,
		Placement: mygo.PlacementBottom,
		Gap:       4,
		Width:     flyoutWidth,
		Height:    flyoutHeight,
		Focusable: true,
		Shadow:    true,
		Popover:   true,
		Content:   ui.View(d.flyoutView),
	})
	d.panel.OnClosed(func() { d.panel = nil })
}

// flyoutSummary condenses the app's state into the panel's headline.
//
// It is a pure function of the snapshot so a test can pin the wording —
// the panel is the one surface drawn without the window, and its first
// line is what the user glances at. Health outranks dirtiness: a rule
// that is down is worse than a change that is unsaved.
func flyoutSummary(engine string, mappings []model.Mapping, dirty bool) (headline, detail string) {
	runnable, down := 0, 0
	for _, m := range mappings {
		if m.Enabled && m.ListenPort != 0 {
			runnable++
		}
		if m.HealthStatus == model.HealthUnhealthy {
			down++
		}
	}
	if engine == "running" {
		headline = "转发运行中"
	} else {
		headline = "转发已停止"
	}
	headline += " · " + pluralMappings(runnable)
	switch {
	case down > 0:
		detail = pluralMappings(down) + "健康检查失败"
	case dirty:
		detail = "有未应用的更改"
	default:
		detail = "规则与配置一致"
	}
	return headline, detail
}

func pluralMappings(n int) string {
	return strconv.Itoa(n) + " 条映射"
}

// flyoutView draws the panel: the state summary, the recent mappings,
// and the actions. All data is read through App.Snapshot, which copies
// under the app's lock, so the frame never races the background work.
//
// The panel is a glance, not a dashboard: it is drawn when it opens and
// after that only when a button asks for a frame. An action closes it —
// the flyout convention — so the summary cannot go stale while it is
// up; the next open reads the state fresh.
func (d *Dock) flyoutView(c *ui.Context) {
	t := c.Theme()
	mappings, _, _, _, engine, dirty, _, _ := d.app.Snapshot()
	headline, detail := flyoutSummary(engine, mappings, dirty)
	down := 0
	for _, m := range mappings {
		if m.HealthStatus == model.HealthUnhealthy {
			down++
		}
	}

	c.Root().Background(ui.Transparent)
	box := ui.Column(c).Fill()
	switch {
	case c.Vibrancy(): // over the popover's material on macOS
	case runtime.GOOS == "windows":
		box.Background(t.Surface)
	default:
		box.Background(t.Surface).Radius(10).Border(1, t.Border)
	}
	box.Padding(16).Gap(10).Children(func() {
		ui.Text(c, "JumpBoard").FontSize(15).Bold()
		ui.Text(c, headline).FontSize(12).TextColor(t.TextMuted)
		detailColor := t.TextMuted
		if down > 0 {
			detailColor = t.Danger
		}
		ui.Text(c, detail).FontSize(12).TextColor(detailColor)

		recent := mappings
		if len(recent) > recentMax {
			recent = recent[:recentMax]
		}
		if len(recent) == 0 {
			ui.Text(c, "还没有映射").FontSize(12).TextColor(t.TextMuted)
		}
		for i := range recent {
			m := recent[i]
			if ui.Row(c.Key("map"+strconv.Itoa(i))).Gap(8).AlignItems(ui.Center).
				Padding(6, 8).Radius(6).Children(func() {
				dot := t.Success
				if !m.Enabled {
					dot = t.TextMuted
				} else if m.HealthStatus == model.HealthUnhealthy {
					dot = t.Danger
				}
				ui.Box(c).Size(6, 6).Radius(3).Background(dot)
				ui.Text(c, m.Name).Grow(1).FontSize(12)
				if m.Enabled {
					ui.Text(c, "已启用").FontSize(11).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "已停用").FontSize(11).TextColor(t.TextMuted)
				}
			}).Clicked() {
				// Locating a row is the main window's job; the panel just
				// gets the user there.
				d.panel.Close()
				d.Show()
			}
		}

		ui.Spacer(c)
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "应用配置").OnClick(func() {
				d.panel.Close()
				d.app.RunBackground(d.app.Apply)
			})
			ui.Button(c, "主窗口").OnClick(func() {
				d.panel.Close()
				d.Show()
			})
		})
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "设置").OnClick(func() {
				d.panel.Close()
				d.app.SetPage(appt.PageSettings)
				d.Show()
			})
			ui.Button(c, "退出").OnClick(func() {
				d.panel.Close()
				mygo.App.Quit()
			})
		})
	})
}
