package appt

import (
	"errors"
	"testing"

	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/store"
)

// loginApp builds an app on a throwaway data directory, with no desktop
// behind the login-item hook, like the other appt tests.
func loginApp(t *testing.T) *App {
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
	return a
}

// TestOpenAtLoginStartsOff pins the default: nothing starts at login until
// the user asks, and an unset key reads as off.
func TestOpenAtLoginStartsOff(t *testing.T) {
	a := loginApp(t)
	if a.OpenAtLogin() {
		t.Error("a fresh app should not start at login")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "" {
		t.Errorf("the key should be unset, got %q", got)
	}
}

// TestSetOpenAtLoginRecordsTheChoice walks the switch both ways with a nil
// hook: there is no desktop to register with, so the choice is only
// recorded, in the field the switch reads and in the store.
func TestSetOpenAtLoginRecordsTheChoice(t *testing.T) {
	a := loginApp(t)
	if err := a.SetOpenAtLogin(true); err != nil {
		t.Fatalf("turning it on: %v", err)
	}
	if !a.OpenAtLogin() {
		t.Error("the switch should read on")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "1" {
		t.Errorf("store = %q, want 1", got)
	}
	if err := a.SetOpenAtLogin(false); err != nil {
		t.Fatalf("turning it off: %v", err)
	}
	if a.OpenAtLogin() {
		t.Error("the switch should read off")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "0" {
		t.Errorf("store = %q, want 0", got)
	}
}

// TestSetOpenAtLoginSameValueIsNoOp pins the whitelist precedent: a call
// that changes nothing writes nothing, so the log keeps no phantom flip.
// A sentinel sits behind the field's back to catch the write.
func TestSetOpenAtLoginSameValueIsNoOp(t *testing.T) {
	a := loginApp(t)
	if err := a.SetOpenAtLogin(true); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.SetSetting(store.SettingOpenAtLogin, "sentinel"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetOpenAtLogin(true); err != nil {
		t.Fatalf("a same-value call should succeed: %v", err)
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "sentinel" {
		t.Errorf("a same-value call wrote %q; it should write nothing", got)
	}
}

// TestSetOpenAtLoginHookFailureChangesNothing walks the refusal a
// development build gives — no app bundle to register — which must leave
// the switch and the store as they were.
func TestSetOpenAtLoginHookFailureChangesNothing(t *testing.T) {
	a := loginApp(t)
	a.OnOpenAtLogin = func(bool) error { return errors.New("no bundle") }
	if err := a.SetOpenAtLogin(true); err == nil {
		t.Fatal("the desktop's refusal should come back")
	}
	if a.OpenAtLogin() {
		t.Error("the switch should stay off")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "" {
		t.Errorf("the store should be untouched, got %q", got)
	}
}

// TestSyncOpenAtLoginFollowsTheDesktop checks the startup sync: what the
// desktop's login items say wins over whatever the store held, and a
// report that agrees with the mirror writes nothing.
func TestSyncOpenAtLoginFollowsTheDesktop(t *testing.T) {
	a := loginApp(t)
	a.SyncOpenAtLogin(true)
	if !a.OpenAtLogin() {
		t.Error("the mirror should follow the desktop")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "1" {
		t.Errorf("store = %q, want 1", got)
	}
	// A second report of the same state is stable.
	a.SyncOpenAtLogin(true)
	if !a.OpenAtLogin() {
		t.Error("the mirror should still read on")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "1" {
		t.Errorf("store = %q, want 1", got)
	}
	// The desktop can also drop the entry behind the app's back.
	a.SyncOpenAtLogin(false)
	if a.OpenAtLogin() {
		t.Error("the mirror should follow the desktop back off")
	}
	if got := a.Store.Setting(store.SettingOpenAtLogin, ""); got != "0" {
		t.Errorf("store = %q, want 0", got)
	}
}
