// Package native is the desktop integration: the tray icon, the
// close-to-tray behaviour, the global shortcut and the desktop notifier.
//
// All of it needs the running mygo app, so it is wired from main.go's
// WhenReady and never from a test: the app package holds the behaviour
// behind hooks (App.Notify, App.OnChange) and this package only binds
// those hooks to the desktop. A machine — or a CI run — that never calls
// Setup simply has no tray, which is the graceful degradation.
package native

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"runtime"

	"github.com/egoist/mygo"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// Notify posts a desktop notification.
//
// Notifications need a packaged app (mygo build): in development the
// system has no bundle to attribute them to, and on macOS the user may
// have denied them outright. Either way the app's own record — the toast,
// the log — already holds the event, so the failure is logged and
// swallowed rather than shown.
func Notify(title, body string) {
	if !mygo.NotificationsSupported() {
		return
	}
	if err := mygo.NewNotification(mygo.NotificationOptions{
		ID:    "jumpboard.notice",
		Title: title,
		Body:  body,
	}).Show(); err != nil {
		log.Printf("通知未送达（%v）：%s - %s", err, title, body)
	}
}

// Dock is the tray integration once Setup has run: the tray with its
// menu, the flyout panel a left click opens, and the window they steer.
type Dock struct {
	app      *appt.App
	win      *mygo.Window
	tray     *mygo.Tray
	panel    *mygo.Window // the open flyout, or nil
	quitting bool
}

// Setup binds the app to the desktop: a tray icon, closing the window
// into the tray instead of quitting, and a global show/hide shortcut. It
// runs inside WhenReady, where the mygo app exists. appIcon is the app's
// full-colour mark (resources/icon.png): the menu bar gets a black
// template of its own, the taskbar gets the mark itself.
//
// The tray is the app's second surface: with the window closed, it can
// still show the window, apply the rules, open settings and quit. If the
// desktop refuses the tray — the platform has none, or the icon was
// rejected — the window keeps its ordinary close-and-quit behaviour and
// the shortcut keeps working, so the app degrades to what it was.
func Setup(a *appt.App, win *mygo.Window, appIcon []byte) *Dock {
	d := &Dock{app: a, win: win}

	// macOS re-tints a template icon for its bar; everywhere else a black
	// glyph would vanish on a dark taskbar, so the coloured mark goes
	// as-is and the system scales it.
	icon, template := trayIcon(), true
	if runtime.GOOS != "darwin" && len(appIcon) > 0 {
		icon, template = appIcon, false
	}
	// The menu is the fallback surface: quick, native, and the only one
	// Linux has. Its first item opens the panel everywhere, so the
	// browsing surface is reachable from the keyboard and from a tray
	// that reports no clicks.
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开面板", Click: func(*mygo.MenuItem, *mygo.Window) { d.toggleFlyout() }},
		{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { d.Show() }},
		{Label: "隐藏主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { win.Hide() }},
		mygo.Separator(),
		{Label: "应用配置", Click: func(*mygo.MenuItem, *mygo.Window) {
			// Apply binds ports; the menu must not wait for it.
			a.RunBackground(a.Apply)
		}},
		{Label: "打开设置", Click: func(*mygo.MenuItem, *mygo.Window) {
			a.SetPage(appt.PageSettings)
			d.Show()
		}},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	if tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "jumpboard.tray",
		Icon:           icon,
		IconIsTemplate: template,
		ToolTip:        "JumpBoard",
	}); err != nil {
		log.Printf("托盘不可用：%v", err)
	} else {
		d.tray = tray
		// Left click browses — the panel holds the state and the recent
		// rules — and right click opens the menu, where quitting lives.
		// AppIndicator-based Linux trays tell nothing of clicks, so there
		// the menu is permanent instead.
		if runtime.GOOS == "linux" {
			tray.SetMenu(menu)
		} else {
			tray.OnClick(d.toggleFlyout)
			tray.OnRightClick(func() {
				if d.panel != nil {
					d.panel.Close()
				}
				// The menu for this popup only, so clicks keep toggling
				// the panel.
				tray.SetMenu(menu)
				defer tray.SetMenu(nil)
				tray.PopUpMenu()
			})
		}
	}

	// Clicking a desktop notification brings the main window forward.
	mygo.App.OnNotificationClick(func(string) {
		d.Show()
	})

	// macOS reopen: clicking the Dock icon when the window is hidden brings it back.
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			d.Show()
		}
	})

	// The global show/hide shortcut is a setting the settings card can
	// change while the app runs; appt holds the stored value and this
	// package holds the registration. Register the stored accelerator —
	// appt.Hotkey defaults to the binding the app has always had — and
	// hand the card the swap through appt.ApplyHotkey, so a change fails
	// loudly in the card (the combination may be taken) without the app
	// package importing this one.
	acc := a.Hotkey()
	if err := mygo.GlobalShortcut.Register(acc, func() { d.Toggle() }); err != nil {
		// Most often the combination is taken; the tray still toggles the
		// window, so this is a note rather than a failure.
		log.Printf("全局快捷键 %s 注册失败：%v", acc, err)
	}
	appt.ApplyHotkey = d.setHotkey

	// Closing the window parks it in the tray: the rules keep forwarding
	// and the tray menu brings it back. A real quit — the tray's 退出,
	// Cmd+Q — sets the flag first so the close passes through.
	win.OnClose(func(e *mygo.CloseEvent) {
		if !d.quitting {
			e.PreventDefault()
			win.Hide()
		}
	})

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { d.quitting = true })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		// Give the shortcut back before the process goes; the tray dies
		// with it either way, but a leaked hotkey would outlive the app
		// in the system's table on some platforms.
		mygo.GlobalShortcut.UnregisterAll()
		if d.tray != nil {
			d.tray.Destroy()
		}
	})
	return d
}

// Show brings the window back. Setup only ever hides the window — the
// tray parks it rather than destroying it — so a destroyed window here
// means something outside this package closed it for good; there is
// nothing to show and the call is a no-op.
func (d *Dock) Show() {
	if d.win.IsDestroyed() {
		return
	}
	d.win.Show()
	d.win.Focus()
}

// Toggle hides a visible window and shows a hidden one.
func (d *Dock) Toggle() {
	if d.win.IsDestroyed() {
		return
	}
	if d.win.IsVisible() {
		d.win.Hide()
		return
	}
	d.Show()
}

// setHotkey rebinds the global show/hide shortcut, as the settings card
// asks through appt.ApplyHotkey.
//
// The new binding is registered before the old one is given up, so a
// refusal — the combination is taken, the string is not an accelerator —
// leaves the working binding untouched and needs no rollback; the caller
// (App.SetHotkey) reverts the stored setting on the error. Unregistering
// first would leave a span with no binding at all, and a span where the
// old one is gone but the new one never came. Unregister of an
// accelerator that was never registered — the stored value was garbage
// and Setup's Register failed — is a no-op, which is exactly right.
func (d *Dock) setHotkey(oldAcc, newAcc string) error {
	if err := mygo.GlobalShortcut.Register(newAcc, func() { d.Toggle() }); err != nil {
		return err
	}
	mygo.GlobalShortcut.Unregister(oldAcc)
	return nil
}

// trayIcon draws the 32x32 template image the menu bar shows: the app
// mark — two pads and the hop between them — in solid black, which macOS
// re-tints for light and dark bars. A template icon over the full-colour
// resources/icon.png, which was drawn for the Dock's size and would read
// as a muddy 16-pixel blob up there.
//
// The glyph is drawn thick on purpose: a 6px-wide pad at @2x reads as a
// hairline's 3px at the bar's 1x scale, and a thin icon is what the bar
// renders as near-invisible in its overflow manager. Both pads stand
// 15px tall and the connector is 5px thick, so the mark keeps its weight
// wherever macOS puts it.
func trayIcon() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	black := color.NRGBA{A: 255}
	// Set with a rect helper so the pads read as rounded at this size.
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				img.SetNRGBA(x, y, black)
			}
		}
	}
	// The pads: the local port on the left, the upstream on the right,
	// both 6 wide and 15 tall with the corners clipped to read as round.
	pad := func(x0 int) {
		fill(x0, 10, x0+5, 22)
		// Clip the outer corners: the pad reads rounded without a radius.
		img.SetNRGBA(x0, 10, color.NRGBA{})
		img.SetNRGBA(x0, 22, color.NRGBA{})
	}
	pad(3)
	pad(23)
	// The connector: the forwarding itself, a thick shaft meeting an
	// arrowhead that lands inside the right pad's height.
	fill(9, 14, 15, 18)
	// The arrowhead: a triangle whose tip is the far edge of the shaft.
	for dy := -5; dy <= 5; dy++ {
		for x := 15; x <= 20-(abs(dy)); x++ {
			img.SetNRGBA(x, 16+dy, black)
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// abs is the integer absolute value, for the arrowhead's slope.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
