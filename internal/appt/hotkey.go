package appt

import (
	"errors"
	"strings"

	"github.com/lianxin255/jumpboard/internal/store"
)

// DefaultHotkey is the global show/hide shortcut registered when the
// setting is unset — the binding the app has always had, kept as the
// default so an upgrade changes nothing until the user changes it.
const DefaultHotkey = "CmdOrCtrl+Shift+J"

// ApplyHotkey swaps the registered global shortcut from oldAcc to newAcc,
// returning an error when the desktop refuses the new one — most often
// because the combination is already taken. native.Setup points it at the
// real registration; it stays nil everywhere the desktop does not exist,
// which is every test, and a nil hook means the setting persists and the
// next launch registers it.
//
// It is a package variable rather than a field beside App.Clipboard and
// App.Notify, because the App struct is declared in app.go and this slice
// leaves that file alone. The variable buys the same nil-in-tests seam at
// the cost of being one registration per process instead of per app.
var ApplyHotkey func(oldAcc, newAcc string) error

// Hotkey returns the global show/hide accelerator: the stored setting, or
// the default when it is unset or blank.
func (a *App) Hotkey() string {
	if v := strings.TrimSpace(a.Store.Setting(store.SettingHotkey, "")); v != "" {
		return v
	}
	return DefaultHotkey
}

// SetHotkey stores the global show/hide accelerator and applies it at once.
//
// An empty accelerator is refused: the shortcut cannot be unassigned,
// because clearing it would strand a window parked in the tray with no key
// to bring it back. When the desktop refuses the new combination — the
// hook's error — the stored value rolls back to the old one, so what the
// settings card shows is always what is registered.
func (a *App) SetHotkey(acc string) error {
	acc = strings.TrimSpace(acc)
	if acc == "" {
		return errors.New("快捷键不能留空：清空后窗口将无法唤起")
	}
	old := a.Hotkey()
	if acc == old {
		return nil
	}
	if err := a.Store.SetSetting(store.SettingHotkey, acc); err != nil {
		return err
	}
	if ApplyHotkey != nil {
		if err := ApplyHotkey(old, acc); err != nil {
			// The desktop kept the old binding; the store must say so too,
			// or the next launch would register the combination this one
			// refused. Reverting can only fail by losing the database, in
			// which case the error names both.
			if rerr := a.Store.SetSetting(store.SettingHotkey, old); rerr != nil {
				return errors.New("快捷键未保存：" + err.Error() + "；回滚旧值也失败：" + rerr.Error())
			}
			return err
		}
	}
	a.Log.Info("全局快捷键已改为 %s", acc)
	a.notify()
	return nil
}
