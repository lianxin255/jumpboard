package appt

import (
	"errors"
	"testing"

	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/store"
)

// hotkeyApp builds an app on a throwaway data directory, with the native
// hotkey hook nil: without a desktop there is nothing to re-register,
// which is also what the settings card runs against in the e2e tests.
func hotkeyApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("JUMPBOARD_DATA_ROOT", t.TempDir())
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	ApplyHotkey = nil
	t.Cleanup(func() { ApplyHotkey = nil })
	return a
}

// TestHotkeyDefaultsToTheOldBinding checks that an unset setting keeps the
// binding the app has always had: the migration must change nothing for a
// user who never opens the card.
func TestHotkeyDefaultsToTheOldBinding(t *testing.T) {
	a := hotkeyApp(t)
	if DefaultHotkey != "CmdOrCtrl+Shift+J" {
		t.Fatalf("DefaultHotkey = %q; it is the historical binding and must not drift", DefaultHotkey)
	}
	if got := a.Hotkey(); got != DefaultHotkey {
		t.Fatalf("Hotkey() = %q, want the default %q", got, DefaultHotkey)
	}
}

// TestSetHotkeyRefusesEmpty checks the one value that cannot be stored:
// clearing the shortcut would strand a window parked in the tray.
func TestSetHotkeyRefusesEmpty(t *testing.T) {
	a := hotkeyApp(t)
	for _, blank := range []string{"", "   "} {
		if err := a.SetHotkey(blank); err == nil {
			t.Fatalf("SetHotkey(%q) should be refused", blank)
		}
	}
	if got := a.Hotkey(); got != DefaultHotkey {
		t.Fatalf("a refused save changed nothing; Hotkey() = %q", got)
	}
}

// TestSetHotkeyTrimsAndPersists checks the round trip with no hook: the
// value is trimmed, stored, and read back by Hotkey.
func TestSetHotkeyTrimsAndPersists(t *testing.T) {
	a := hotkeyApp(t)
	if err := a.SetHotkey("  CmdOrCtrl+Shift+Alt+K  "); err != nil {
		t.Fatal(err)
	}
	if got := a.Hotkey(); got != "CmdOrCtrl+Shift+Alt+K" {
		t.Fatalf("Hotkey() = %q, want the trimmed accelerator", got)
	}
	if v := a.Store.Setting(store.SettingHotkey, ""); v != "CmdOrCtrl+Shift+Alt+K" {
		t.Fatalf("the store holds %q, want the trimmed accelerator", v)
	}
}

// TestSetHotkeyAppliesThroughTheHook checks the desktop hand-off: the hook
// is called once with the old and the new accelerator, and a swap that
// succeeds is kept.
func TestSetHotkeyAppliesThroughTheHook(t *testing.T) {
	a := hotkeyApp(t)
	if err := a.SetHotkey("CmdOrCtrl+Alt+J"); err != nil {
		t.Fatal(err)
	}
	var seen [][2]string
	ApplyHotkey = func(oldAcc, newAcc string) error {
		seen = append(seen, [2]string{oldAcc, newAcc})
		return nil
	}
	if err := a.SetHotkey("CmdOrCtrl+Shift+Alt+K"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != [2]string{"CmdOrCtrl+Alt+J", "CmdOrCtrl+Shift+Alt+K"} {
		t.Fatalf("the hook saw %v, want one swap CmdOrCtrl+Alt+J -> CmdOrCtrl+Shift+Alt+K", seen)
	}
	if got := a.Hotkey(); got != "CmdOrCtrl+Shift+Alt+K" {
		t.Fatalf("Hotkey() = %q after a successful swap", got)
	}
}

// TestSetHotkeyRevertsWhenTheDesktopRefuses checks the failure path: when
// the desktop will not take the new combination, the error surfaces and
// the store goes back to the old binding, so what the card shows is what
// is registered.
func TestSetHotkeyRevertsWhenTheDesktopRefuses(t *testing.T) {
	a := hotkeyApp(t)
	if err := a.SetHotkey("CmdOrCtrl+Alt+J"); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("the combination is taken")
	ApplyHotkey = func(oldAcc, newAcc string) error { return refused }
	if err := a.SetHotkey("CmdOrCtrl+Shift+K"); err == nil {
		t.Fatal("the refusal should surface")
	} else if !errors.Is(err, refused) {
		t.Fatalf("the error should be the hook's, got %v", err)
	}
	if got := a.Hotkey(); got != "CmdOrCtrl+Alt+J" {
		t.Fatalf("the store should hold the old binding, got %q", got)
	}
}

// TestSetHotkeySameValueDoesNothing checks that saving what is already
// bound never reaches the hook: there is nothing to re-register, and a
// call would rewrite the operation log for a change that never happened.
func TestSetHotkeySameValueDoesNothing(t *testing.T) {
	a := hotkeyApp(t)
	if err := a.SetHotkey("CmdOrCtrl+Alt+J"); err != nil {
		t.Fatal(err)
	}
	ApplyHotkey = func(oldAcc, newAcc string) error {
		t.Errorf("the hook ran for a no-change save: %s -> %s", oldAcc, newAcc)
		return nil
	}
	if err := a.SetHotkey("CmdOrCtrl+Alt+J"); err != nil {
		t.Fatal(err)
	}
	if got := a.Hotkey(); got != "CmdOrCtrl+Alt+J" {
		t.Fatalf("Hotkey() = %q, want the untouched binding", got)
	}
}
