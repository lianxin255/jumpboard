package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/store"
)

// TestHotkeyCardEditsTheBinding walks the shortcut card on the settings
// page: it shows the default binding, a typed accelerator is saved and
// shown, and a refused one surfaces its error while the old binding stays.
//
// The test app has no desktop, so appt.ApplyHotkey stays nil and a save
// persists only — the same seam the appt tests use. What the desktop
// refuses is driven here by the hook, which is exactly the error the card
// shows.
func TestHotkeyCardEditsTheBinding(t *testing.T) {
	a := testApp(t)
	// The card sits low on the settings page; the window is tall so the
	// field is visible without scrolling.
	tt := tester(t, a, 1200, 1400)

	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(appt.DefaultHotkey) {
		t.Fatalf("the card should show the default binding: %v", tt.Texts())
	}
	if !tt.HasText("⌘N") || !tt.HasText("新建映射") {
		t.Fatalf("the in-app shortcuts should stay on the card: %v", tt.Texts())
	}

	// A new accelerator is saved and the card shows it.
	fillField(t, tt, "新快捷键", "CmdOrCtrl+Shift+Alt+K")
	if err := tt.Click("保存快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("CmdOrCtrl+Shift+Alt+K") {
		t.Fatalf("the card should show the saved binding: %v", tt.Texts())
	}
	if got := a.Hotkey(); got != "CmdOrCtrl+Shift+Alt+K" {
		t.Fatalf("the store holds %q, want the saved accelerator", got)
	}

	// A binding the desktop refuses: the hook's error shows on the card
	// and the stored value stays. The field keeps what was typed, so the
	// user can edit it.
	appt.ApplyHotkey = func(oldAcc, newAcc string) error {
		return errHotkeyTaken
	}
	t.Cleanup(func() { appt.ApplyHotkey = nil })
	fillField(t, tt, "新快捷键", "CmdOrCtrl+Shift+X")
	if err := tt.Click("保存快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(errHotkeyTaken.Error()) {
		t.Fatalf("the refusal should show on the card: %v", tt.Texts())
	}
	if got := a.Store.Setting(store.SettingHotkey, ""); got != "CmdOrCtrl+Shift+Alt+K" {
		t.Fatalf("a refused save should keep the old binding, the store holds %q", got)
	}
}

// errHotkeyTaken is what the test's desktop hook says when the
// combination is taken.
var errHotkeyTaken = hotkeyTakenError("the combination is taken")

type hotkeyTakenError string

func (e hotkeyTakenError) Error() string { return string(e) }
