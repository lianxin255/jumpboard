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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/engine"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/services"
	"github.com/lianxin255/jumpboard/internal/store"
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
	Eng   *engine.Local
	Tpl   *services.Templates
	Log   *services.Logger
	Check *services.Checker

	// OnChange asks the window for a frame. Set by the UI.
	OnChange func()
	// OnTheme applies the appearance to the window's material, which the
	// sidebar shows through. Set by the UI; nil in tests.
	OnTheme func(ThemeChoice)
	// Clipboard puts text on the system clipboard. main.go points it at
	// mygo's clipboard, which needs the running app; a test leaves it nil
	// and reads Copied instead.
	Clipboard func(string)
	// SaveDialog and OpenDialog ask the user where to put a file and
	// which files to read. main.go points them at the native dialogs,
	// which need the running app; a test leaves them nil, and the UI
	// treats a nil hook as a dialog the user canceled.
	SaveDialog func(mygo.SaveDialogOptions) (string, error)
	OpenDialog func(mygo.OpenDialogOptions) ([]string, error)
	// Notify posts a desktop notification, for what is worth knowing
	// while the window sits closed in the tray: a rule set that failed to
	// apply, a service that stopped answering. main.go points it at the
	// native notifier; a test leaves it nil and nothing is sent.
	Notify func(title, body string)

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
	// applyErr is why the last apply failed, shown next to the button.
	applyErr string
	// applyErrPort is the port a bind failure named, 0 when the failure
	// was not about a port: the diagnosis uses it to find the one
	// mapping involved without parsing error text.
	applyErrPort int
	// failKind says which action failed, "apply" or "check", so the
	// diagnosis can name the operation the user actually ran.
	failKind string
	// applyWarn is what the engine could not honor, shown once.
	applyWarn string
	// ruleStatus is what each rule is doing, refreshed with the data.
	ruleStatus []engine.RuleStatus
	// copied is the last text CopyAddress put on the clipboard.
	copied string
	// bg tracks RunBackground's goroutines, so Close can wait them out.
	bg sync.WaitGroup
	// applied is set once a rule set has been started, so that the table
	// can tell "not started yet" from "started and not listening".
	applied bool

	// The AI failure diagnosis: diagBusy while the request is out,
	// diagErr when it failed, diagAdvice once it landed.
	diagBusy   bool
	diagErr    string
	diagAdvice *ai.Advice

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

	// The last draft the model proposed, kept for the AI panel.
	lastAIDraft *model.Mapping
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

	a := &App{
		Paths: p,
		Store: st,
		Eng:   engine.NewLocal(),
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
	// Bring the rules up, so that the mappings the user had are forwarding
	// again. A port that cannot be bound is reported rather than fatal: the
	// window still opens, and the rule shows its error.
	spec, err := a.spec()
	if err != nil {
		return nil, err
	}
	if _, err := a.Eng.Apply(spec); err != nil {
		a.Log.Warn("启动时应用规则失败: %v", err)
	}
	a.mu.Lock()
	a.engine = string(a.Eng.Status())
	a.mu.Unlock()

	a.Check.OnResult = func(changed []services.HealthChange) {
		_ = a.Reload()
		a.notifyHealthChanges(changed)
	}
	_ = st.TrimLogs(10000)
	return a, nil
}

// Close stops the background work and closes the database.
//
// The background tasks are waited for first: an Apply still binding ports
// or an Import still writing rows would otherwise race the store closing
// underneath them at quit.
func (a *App) Close() {
	a.Check.Stop()
	_ = a.Eng.Stop()
	a.bg.Wait()
	if a.Store != nil {
		_ = a.Store.Close()
	}
}

// refreshEngine copies the engine's state into what the frame reads.
func (a *App) refreshEngine() {
	status := string(a.Eng.Status())
	rules := a.Eng.Rules()
	a.mu.Lock()
	a.engine = status
	a.ruleStatus = rules
	a.mu.Unlock()
}

// EngineApplied reports whether the rules have been started at least once,
// so that a row's "not listening" state means something.
func (a *App) EngineApplied() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.applied
}

// Copied returns the text of the last successful copy, for the tests.
func (a *App) Copied() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.copied
}

// RuleState reports whether a mapping is listening, for its row.
func (a *App) RuleState(mappingID string) (bool, string) {
	return a.Eng.RuleState(mappingID)
}

// RuleInfo returns the counters of a mapping's rule, for its row.
func (a *App) RuleInfo(mappingID string) engine.RuleStatus {
	a.mu.Lock()
	rules := a.ruleStatus
	a.mu.Unlock()
	for _, r := range rules {
		if r.MappingID == mappingID {
			return r
		}
	}
	return engine.RuleStatus{}
}

// RuleStatus returns what each rule is doing, for the table.
func (a *App) RuleStatus() []engine.RuleStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]engine.RuleStatus(nil), a.ruleStatus...)
}

// notify asks for a frame after the state changed.
func (a *App) notify() {
	if a.OnChange != nil {
		a.OnChange()
	}
}

// notifyUser posts a desktop notification. The hook is nil in tests, and
// the notifier itself stays silent on a desktop that has none, so this is
// always safe to call.
func (a *App) notifyUser(title, body string) {
	if a.Notify != nil {
		a.Notify(title, body)
	}
}

// notifyHealthChanges tells the user about mappings that were healthy and
// stopped answering — the one health transition that is news. A mapping
// that was never checked turning unhealthy is the first look, not a
// change, and recovery is visible in the table when the window opens.
// One notification names them all: a switch that drops half the rules
// should not fire half a dozen banners.
func (a *App) notifyHealthChanges(changed []services.HealthChange) {
	var down []string
	for _, ch := range changed {
		if ch.From == model.HealthHealthy && ch.To == model.HealthUnhealthy {
			down = append(down, ch.Mapping.Name)
		}
	}
	if len(down) == 0 {
		return
	}
	a.Log.Warn("健康检查失败：%s", strings.Join(down, "、"))
	if len(down) == 1 {
		a.notifyUser("JumpBoard", fmt.Sprintf("「%s」健康检查失败，上游没有响应", down[0]))
		return
	}
	a.notifyUser("JumpBoard", fmt.Sprintf("%d 条映射健康检查失败：%s", len(down), strings.Join(down, "、")))
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
	a.mu.Unlock()
	a.refreshEngine()
	a.notify()
	return nil
}

// Spec exposes the rule set the engine would run, for the export.
func (a *App) Spec() (engine.Spec, error) { return a.spec() }

// spec builds the engine's rule set from the database.
//
// A whitelist that is on with no addresses is refused: it would block
// every client, including the administrator.
func (a *App) spec() (engine.Spec, error) {
	var wl []string
	if a.Store.WhitelistOn() {
		ips, err := a.Store.EnabledWhitelistIPs()
		if err != nil {
			return engine.Spec{}, err
		}
		if len(ips) == 0 {
			return engine.Spec{}, fmt.Errorf("白名单已开启但没有任何 IP，请先添加或在设置里关闭白名单")
		}
		wl = ips
	}
	mappings, err := a.Store.ListEnabledMappings()
	if err != nil {
		return engine.Spec{}, err
	}
	// Rules without a port cannot listen; the table shows why.
	runnable := make([]model.Mapping, 0, len(mappings))
	for _, m := range mappings {
		if m.ListenPort == 0 {
			continue
		}
		runnable = append(runnable, m)
	}
	return engine.Spec{
		Mappings:     runnable,
		WhitelistIPs: wl,
		LogDir:       a.Paths.LogsDir,
	}, nil
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
	a.bg.Add(1)
	defer a.bg.Done()
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
