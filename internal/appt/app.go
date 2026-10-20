// Package appt is the app's state and its wiring to the store, the engine
// and the background services. It knows nothing about drawing: the UI
// package reads this state every frame.
//
// Two kinds of state live here, and they are guarded differently:
//
//   - Data the user edits and the background work reads — mappings, the
//     log, the engine status — is behind a mutex, because goroutines write
//     it and the frame reads it.
//   - The UI's own transient state — which page is shown, which dialog is
//     open, what is typed into a field — is only ever touched on the main
//     thread, inside a frame or an action a frame ran. It needs no lock,
//     and adding one would invite a deadlock between the two.
package appt

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
	"github.com/lianxin255/nginx-jump-gui/internal/nginx"
	"github.com/lianxin255/nginx-jump-gui/internal/paths"
	"github.com/lianxin255/nginx-jump-gui/internal/services"
	"github.com/lianxin255/nginx-jump-gui/internal/store"
)

// Page is the page the window shows.
type Page int

// Pages.
const (
	PageMappings Page = iota
	PageLogs
	PageHistory
	PageSettings
)

// Status is a short outcome the UI shows as a toast.
type Status struct {
	Text  string
	IsErr bool
	// At is when it was set, so the view can drop it after a while.
	At time.Time
}

// ThemeChoice is the appearance the user picked.
type ThemeChoice string

// Theme choices.
const (
	ThemeSystem ThemeChoice = "system"
	ThemeLight  ThemeChoice = "light"
	ThemeDark   ThemeChoice = "dark"
)

// App is the whole application state.
type App struct {
	Paths paths.Paths
	Store *store.Store
	Eng   *nginx.Engine
	Tpl   *services.Templates
	Log   *services.Logger
	Check *services.Checker

	// OnChange asks the window for a frame. Set by the UI.
	OnChange func()

	mu sync.Mutex
	// data below is read by the frame and written from goroutines.
	mappings  []model.Mapping
	logs      []model.LogEntry
	snapshots []model.Snapshot
	whitelist []model.WhitelistEntry
	status    Status
	engine    string
	dirty     bool
	busy      bool
	// checkErr is why the last apply failed, shown next to the button.
	applyErr string

	// UI state: main thread only.
	page     Page
	formOpen bool
	formEdit *model.Mapping
	form     FormValues
	// formErrs are the fields the last save found wrong, cleared when the
	// form is saved or reopened.
	formErrs     map[string]string
	templateOpen bool
	confirm      *Confirm
	snapshotView *model.Snapshot
	theme        ThemeChoice
	hostName     string
	whitelistOn  bool
	nginxLogRead bool
	aiOpen       bool
}

// Confirm is a question the user must answer before an action runs.
type Confirm struct {
	Title       string
	Body        string
	Label       string
	Destructive bool
	Run         func() error
}

// New wires the app up: it opens the database, builds the engine and
// loads everything the first frame shows.
func New(p paths.Paths) (*App, error) {
	st, err := store.Open(p.DataDir + "/data.db")
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	if err := st.EnsureLocalHost(); err != nil {
		st.Close()
		return nil, fmt.Errorf("初始化本机主机: %w", err)
	}

	// A crash leaves nginx running with the ports taken; clear it before
	// building the engine. It is safe to do from here: no window exists
	// yet, so nothing can be applying a configuration.
	nginx.StopStale(p.PrefixDir)

	a := &App{
		Paths: p,
		Store: st,
		Eng:   nginx.NewEngine(p.NginxBinary, p.PrefixDir),
		Log:   services.NewLogger(st),
		Check: services.NewChecker(st),
	}
	a.theme = ThemeChoice(st.Setting(store.SettingTheme, string(ThemeSystem)))
	a.whitelistOn = st.WhitelistOn()
	a.nginxLogRead = st.SettingBool(store.SettingNginxLogRead, true)
	if name, err := st.HostName(); err == nil && name != "" {
		a.hostName = name
	}

	if err := a.Reload(); err != nil {
		return nil, err
	}
	// Bring nginx up with the configuration on disk, so that the mappings
	// the user had running are running again.
	a.writeConfFromStore()
	if ok, _ := a.Eng.Test(); ok {
		if a.Eng.Status() != "running" {
			if err := a.Eng.Start(); err == nil {
				a.Log.Info("启动时已按现有配置启动 nginx")
			} else {
				a.Log.Warn("启动 nginx 失败: %v", err)
			}
		}
	} else {
		a.Log.Warn("现有的 nginx 配置校验失败，请检查后重新应用")
	}
	a.engine = a.Eng.Status()

	a.Check.OnResult = func(changed []model.Mapping) {
		_ = a.Reload()
	}
	_ = st.TrimLogs(10000)
	return a, nil
}

// Close stops the background work and closes the database.
func (a *App) Close() {
	a.Check.Stop()
	_ = a.Eng.Stop()
	if a.Store != nil {
		_ = a.Store.Close()
	}
}

// Stopped reports whether nginx is running, for the frame.
func (a *App) Stopped() bool {
	return a.engStatus() != "running"
}

func (a *App) engStatus() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.engine
}

// notify asks for a frame after the state changed.
func (a *App) notify() {
	if a.OnChange != nil {
		a.OnChange()
	}
}

// setStatus shows a toast.
func (a *App) setStatus(text string, isErr bool) {
	a.mu.Lock()
	a.status = Status{Text: text, IsErr: isErr, At: time.Now()}
	a.mu.Unlock()
}

// Info shows a success toast.
func (a *App) Info(format string, args ...any) { a.setStatus(fmt.Sprintf(format, args...), false) }

// Fail shows an error toast.
func (a *App) Fail(format string, args ...any) { a.setStatus(fmt.Sprintf(format, args...), true) }

// Reload rereads everything the UI shows and asks for a frame.
func (a *App) Reload() error {
	mappings, err := a.Store.ListMappings()
	if err != nil {
		return err
	}
	logs, err := a.Store.ListLogs(500)
	if err != nil {
		return err
	}
	snaps, err := a.Store.ListSnapshots(50)
	if err != nil {
		return err
	}
	wl, err := a.Store.ListWhitelist()
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.mappings, a.logs, a.snapshots, a.whitelist = mappings, logs, snaps, wl
	a.engine = a.Eng.Status()
	a.mu.Unlock()
	a.notify()
	return nil
}

// Snapshot returns what the frame draws, copied so that the frame can walk
// it without holding the lock.
func (a *App) Snapshot() (mappings []model.Mapping, logs []model.LogEntry,
	snaps []model.Snapshot, wl []model.WhitelistEntry, engine string, dirty, busy bool, status Status) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]model.Mapping(nil), a.mappings...),
		append([]model.LogEntry(nil), a.logs...),
		append([]model.Snapshot(nil), a.snapshots...),
		append([]model.WhitelistEntry(nil), a.whitelist...),
		a.engine, a.dirty, a.busy, a.status
}

// RunBackground runs fn off the main thread and asks for a frame when it
// is done, reporting a panic as an error toast rather than crashing.
func (a *App) RunBackground(fn func() error) {
	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()
	go func() {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("内部错误: %v", r)
				}
			}()
			err = fn()
		}()
		a.mu.Lock()
		a.busy = false
		a.mu.Unlock()
		if err != nil {
			a.setStatus(err.Error(), true)
			a.Log.Error("%v", err)
		}
		_ = a.Reload()
	}()
}

// writeConfFromStore regenerates nginx.conf from the database, without
// applying it: a stopped engine just gets the file, a running one is left
// alone until the user applies.
func (a *App) writeConfFromStore() {
	mappings, err := a.Store.ListEnabledMappings()
	if err != nil {
		return
	}
	var wl []string
	if a.Store.WhitelistOn() {
		wl, _ = a.Store.EnabledWhitelistIPs()
	}
	conf := nginx.Generate(mappings, nginx.GenOptions{WhitelistIPs: wl})
	_ = a.Eng.WriteConf(conf)
}

// ErrNoMappings is what an apply with nothing enabled reports.
var ErrNoMappings = errors.New("没有启用的映射")
