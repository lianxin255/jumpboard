// The menu bar, the menu the app owns rather than a surface it borrows:
// the bar at the top of the screen on macOS, the bar of a window without
// its own menu on Windows and Linux. SetMenu is one call that holds on
// every platform, so this file is deliberately free of platform branches —
// the roles below resolve to the platform's own items.
package native

import (
	"github.com/egoist/mygo"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// MenuActions is what the app menu does. The menu's shape is fixed —
// appMenu below owns it — so the wiring only has to fill in the doing.
type MenuActions struct {
	// OpenSettings brings the settings page up in the main window.
	OpenSettings func()
}

// appMenu builds the menu bar: the app's own menu first, then the one
// custom entry, then the system's Edit and Window menus.
//
// The role items keep their default submenus — mygo expands a role that
// comes without a Submenu of its own into the platform's usual items, the
// same shape its own default menu is built from — because macOS needs
// them as they are: the app menu carries 退出 ⌘Q, hide and services, and
// the edit menu is what makes cut, copy and paste reach a text input.
// Rewriting either in Chinese would change how the app quits and break
// the keyboard, and their labels already follow the app's language
// (SetLanguages in main.go), so only the custom item needs a label. The
// settings entry is custom for the same reason: mygo has no settings
// role, only the ⌘, convention to hang it on.
func appMenu(actions MenuActions) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) {
			actions.OpenSettings()
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	})
}

// SetupAppMenu installs the menu bar. It runs inside WhenReady, after
// Setup has returned the dock: its one action is the tray's 打开设置 over
// again — the page switches, then the window comes forward — through the
// same Show that tolerates a window something else closed for good.
func SetupAppMenu(d *Dock) {
	mygo.App.SetMenu(appMenu(MenuActions{
		OpenSettings: func() {
			d.app.SetPage(appt.PageSettings)
			d.Show()
		},
	}))
}
