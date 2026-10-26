package native

import (
	"testing"

	"github.com/egoist/mygo"
)

// roles collects the roles of a submenu, for the assertions that walk the
// items a role item was expanded into.
func roles(items []*mygo.MenuItem) map[mygo.MenuRole]bool {
	got := map[mygo.MenuRole]bool{}
	for _, it := range items {
		got[it.Role] = true
	}
	return got
}

// TestAppMenuStructure pins the bar itself: the app menu macOS needs for
// quitting, the one custom entry, and the edit and window menus. The
// default submenus matter as much as the roles — the app menu carries
// 退出 ⌘Q and the edit menu carries the clipboard a text input needs —
// so the test checks mygo expanded them rather than trusting the role
// alone. Structural only: NewMenu builds from a template and needs no
// running app.
func TestAppMenuStructure(t *testing.T) {
	menu := appMenu(MenuActions{OpenSettings: func() {}})
	items := menu.Items()
	if len(items) != 4 {
		t.Fatalf("menu has %d items, want 4 (app, settings, edit, window)", len(items))
	}

	if items[0].Role != mygo.RoleAppMenu {
		t.Errorf("first item role = %q, want app menu", items[0].Role)
	}
	if !roles(items[0].Submenu)[mygo.RoleQuit] {
		t.Error("the app menu's default submenu must carry quit (Cmd+Q)")
	}

	settings := items[1]
	if settings.Role != "" {
		t.Errorf("settings item has role %q, want a plain custom item", settings.Role)
	}
	if settings.Label != "设置…" {
		t.Errorf("settings label = %q, want %q", settings.Label, "设置…")
	}
	if settings.Accelerator != "CmdOrCtrl+," {
		t.Errorf("settings accelerator = %q, want %q", settings.Accelerator, "CmdOrCtrl+,")
	}
	if settings.Click == nil {
		t.Error("settings item has no click handler")
	}

	if items[2].Role != mygo.RoleEditMenu {
		t.Errorf("third item role = %q, want edit menu", items[2].Role)
	}
	edit := roles(items[2].Submenu)
	for _, r := range []mygo.MenuRole{mygo.RoleCut, mygo.RoleCopy, mygo.RolePaste, mygo.RoleSelectAll} {
		if !edit[r] {
			t.Errorf("the edit menu's default submenu is missing %q, which text inputs need", r)
		}
	}

	if items[3].Role != mygo.RoleWindowMenu {
		t.Errorf("fourth item role = %q, want window menu", items[3].Role)
	}
	if !roles(items[3].Submenu)[mygo.RoleMinimize] {
		t.Error("the window menu's default submenu must carry minimize")
	}
}

// TestAppMenuSettingsClick checks the one custom entry dispatches to the
// action it was built with, so the wiring's OpenSettings is what ⌘, runs.
func TestAppMenuSettingsClick(t *testing.T) {
	called := false
	item := appMenu(MenuActions{OpenSettings: func() { called = true }}).Items()[1]
	item.Click(item, nil)
	if !called {
		t.Error("choosing settings did not call OpenSettings")
	}
}
