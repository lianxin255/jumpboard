package appt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/engine"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
	"github.com/lianxin255/jumpboard/internal/services"
	"github.com/lianxin255/jumpboard/internal/store"
)

// --- pages and dialogs, called from the frame (main thread) ---

// Page returns the page shown.
func (a *App) Page() Page { return a.page }

// SetPage switches the page and refreshes what it shows.
func (a *App) SetPage(p Page) {
	a.page = p
	a.notify()
}

// OpenForm opens the mapping form, empty or filled from m.
func (a *App) OpenForm(m *model.Mapping) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.formEdit = m
	if m != nil {
		a.form = FormFromMapping(*m)
	} else {
		a.form = NewForm()
	}
	a.formErrs = nil
	a.formOpen = true
}

// OpenNewFormFrom opens an empty form prefilled from an existing value.
//
// The draft from a model is untrusted and has no ID: it starts a new
// mapping, going through the same validation a typed one does. Opening it
// as an edit would try to update a row that does not exist.
func (a *App) OpenNewFormFrom(m model.Mapping) {
	m.ID = ""
	a.mu.Lock()
	defer a.mu.Unlock()
	a.formEdit = nil
	a.form = FormFromMapping(m)
	a.formErrs = nil
	a.formOpen = true
}

// OpenFormFromTemplate opens the form prefilled by a template.
func (a *App) OpenFormFromTemplate(t model.Template) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.formEdit = nil
	a.form = FormFromTemplate(t)
	a.formErrs = nil
	a.formOpen = true
}

// FormOpen reports whether the form is up, and returns its values.
func (a *App) FormState() (open bool, editing *model.Mapping, f FormValues, errs map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.formOpen, a.formEdit, a.form, a.formErrs
}

// FormOpen reports whether the mapping form is up, for the shortcuts.
func (a *App) FormOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.formOpen
}

// SetForm stores the values the inputs edited.
//
// The frame writes the form it just bound the inputs to, so that the next
// frame builds from what the user typed. Saving reads it from another
// goroutine, hence the lock.
func (a *App) SetForm(f FormValues) {
	a.mu.Lock()
	a.form = f
	a.mu.Unlock()
}

// CloseForm hides the form.
func (a *App) CloseForm() {
	a.mu.Lock()
	a.formOpen, a.formEdit = false, nil
	a.mu.Unlock()
}

// TemplatePickerOpen reports whether the template picker is up.
func (a *App) TemplatePickerOpen() bool { return a.templateOpen }

// SetTemplatePicker shows or hides the template picker.
func (a *App) SetTemplatePicker(open bool) { a.templateOpen = open }

// ConfirmState returns the pending question.
func (a *App) ConfirmState() *Confirm { return a.confirm }

// Ask shows a question, whose action runs when the user accepts.
func (a *App) Ask(c *Confirm) { a.confirm = c }

// ResolveConfirm answers the question; ok runs its action.
func (a *App) ResolveConfirm(ok bool) {
	c := a.confirm
	a.confirm = nil
	if !ok || c == nil || c.Run == nil {
		a.notify()
		return
	}
	a.RunBackground(c.Run)
}

// Theme returns the chosen appearance.
func (a *App) Theme() ThemeChoice { return a.theme }

// SetTheme stores the appearance and applies it at once, to the widgets on
// the next frame and to the window's material now.
func (a *App) SetTheme(t ThemeChoice) {
	a.theme = t
	_ = a.Store.SetSetting(store.SettingTheme, string(t))
	if a.OnTheme != nil {
		a.OnTheme(t)
	}
	a.notify()
}

// HostName returns the local host's name.
func (a *App) HostName() string { return a.hostName }

// SetHostName renames the local host.
func (a *App) SetHostName(name string) {
	if name == "" {
		return
	}
	a.hostName = name
	_ = a.Store.SetHostName(name)
	a.notify()
}

// WhitelistOn reports whether access is limited to the whitelist.
func (a *App) WhitelistOn() bool { return a.whitelistOn }

// SetWhitelistOn turns the whitelist on or off.
//
// Setting it to what it already is does nothing: the switch already
// reports only real changes, and a repeated call would write "已开启" into
// the operation log for a flip that never happened, which reads like an
// access-control change.
func (a *App) SetWhitelistOn(on bool) error {
	if a.whitelistOn == on {
		return nil
	}
	prev := a.whitelistOn
	a.whitelistOn = on
	// An access-control switch that reports success without reaching the
	// database is worse than an error: the window would say "已开启" while
	// the next launch — and any other reader of the store — sees it off,
	// allowing every address. A failed write rolls the mirror back.
	if err := a.Store.SetWhitelistOn(on); err != nil {
		a.whitelistOn = prev
		a.Fail("白名单设置未保存：%v", err)
		return err
	}
	if on {
		a.Log.Info("白名单已开启，仅允许列表中的 IP 访问")
	} else {
		a.Log.Info("白名单已关闭，全部允许")
	}
	a.notify()
	return nil
}

// NginxLogRead reports whether the app reads the engine's access log.
func (a *App) NginxLogRead() bool { return a.nginxLogRead }

// SetNginxLogRead stores the setting.
func (a *App) SetNginxLogRead(on bool) {
	a.nginxLogRead = on
	v := "0"
	if on {
		v = "1"
	}
	_ = a.Store.SetSetting(store.SettingNginxLogRead, v)
	a.notify()
}

// SnapshotView returns the snapshot opened for reading.
func (a *App) SnapshotView() *model.Snapshot { return a.snapshotView }

// SetSnapshotView opens or closes the snapshot reader.
func (a *App) SetSnapshotView(s *model.Snapshot) {
	a.snapshotView = s
	a.notify()
}

// ConfViewOpen reports whether the read-only current-configuration
// viewer is open on the settings page.
func (a *App) ConfViewOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.confViewOpen
}

// SetConfViewOpen opens or closes the configuration viewer. The config it
// shows is generated fresh each frame from the rules as they are, so
// nothing is cached here.
func (a *App) SetConfViewOpen(open bool) {
	a.mu.Lock()
	a.confViewOpen = open
	a.mu.Unlock()
	a.notify()
}

// Status returns the toast to show, if any.
func (a *App) Status() (Status, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status.Text == "" {
		return Status{}, false
	}
	return a.status, true
}

// ClearStatusSoon drops a status after its time, unless a newer one was
// shown meanwhile: a message with a countdown of its own.
func (a *App) ClearStatusSoon(at time.Time) {
	go func() {
		time.Sleep(6 * time.Second)
		a.mu.Lock()
		if a.status.At.Equal(at) {
			a.status = Status{}
		}
		a.mu.Unlock()
		a.notify()
	}()
}

// --- actions, which change data and run off the main thread ---

// SaveForm validates the form and stores the mapping.
func (a *App) SaveForm() error {
	// Work on a copy: the frame replaces the form on every keystroke.
	a.mu.Lock()
	f := a.form
	edit := a.formEdit
	wasOpen := a.formOpen
	a.mu.Unlock()
	if !wasOpen {
		return nil
	}

	m, errs, ok := f.Validate()
	a.mu.Lock()
	a.formErrs = errs
	a.mu.Unlock()
	if !ok {
		a.notify()
		return fmt.Errorf("表单有 %d 处需要修正", len(errs))
	}
	if edit != nil {
		m.ID = edit.ID
		m.HealthStatus = edit.HealthStatus
		m.CreatedAt = edit.CreatedAt
		m.SortOrder = edit.SortOrder
		if err := a.Store.UpdateMapping(m); err != nil {
			return err
		}
		a.Log.Info("修改映射「%s」", m.Name)
	} else {
		if _, err := a.Store.CreateMapping(m); err != nil {
			return err
		}
		a.Log.Info("新建映射「%s」", m.Name)
	}
	a.mu.Lock()
	a.formErrs = nil
	a.mu.Unlock()
	a.CloseForm()
	a.setStatus(fmt.Sprintf("已保存「%s」，点“应用配置”后生效", m.Name), false)
	a.markDirty()
	return nil
}

// SaveFormAndApply saves the form and immediately applies the configuration,
// which is the one-step path for "make this forward live now".
func (a *App) SaveFormAndApply() error {
	if err := a.SaveForm(); err != nil {
		return err
	}
	return a.Apply()
}

// markDirty records that the database and the running configuration
// differ, which the apply button shows.
func (a *App) markDirty() {
	a.mu.Lock()
	a.dirty = true
	a.mu.Unlock()
}

// Dirty reports whether a configuration is waiting to be applied.
func (a *App) Dirty() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dirty
}

// DeleteMapping removes a mapping.
func (a *App) DeleteMapping(id string) error {
	m, err := a.Store.GetMapping(id)
	if err != nil {
		return err
	}
	if err := a.Store.DeleteMapping(id); err != nil {
		return err
	}
	a.Log.Info("删除映射「%s」", m.Name)
	a.markDirty()
	return nil
}

// SetMappingEnabled turns one mapping on or off.
//
// Dirty is marked before the operation log write: the store change is
// what makes the running configuration stale, and a waiter that only
// watches Enabled must not observe the flip before Dirty is set
// (Windows scheduling made that race fail TestToggleMapping).
func (a *App) SetMappingEnabled(id string, on bool) error {
	m, err := a.Store.GetMapping(id)
	if err != nil {
		return err
	}
	if err := a.Store.SetMappingEnabled(id, on); err != nil {
		return err
	}
	a.markDirty()
	if on {
		a.Log.Info("启用映射「%s」", m.Name)
	} else {
		a.Log.Info("停用映射「%s」", m.Name)
	}
	return nil
}

// MoveMapping swaps one mapping with its neighbour, up or down, so the
// table's order is what the user arranges it to be. Only the sort order
// changes: the rules themselves are the same, so the engine and the
// configuration it generates are untouched and nothing is marked dirty.
func (a *App) MoveMapping(id string, up bool) error {
	m, err := a.Store.GetMapping(id)
	if err != nil {
		return err
	}
	ids, err := a.Store.MappingIDs()
	if err != nil {
		return err
	}
	at := -1
	for i, v := range ids {
		if v == id {
			at = i
			break
		}
	}
	if at < 0 {
		return fmt.Errorf("映射「%s」不在排序列表中", m.Name)
	}
	to := at + 1
	if up {
		to = at - 1
	}
	if to < 0 || to >= len(ids) {
		// Already at the edge: nothing to do, and no toast either — the
		// action cannot fail, only do nothing.
		return nil
	}
	ids[at], ids[to] = ids[to], ids[at]
	if err := a.Store.ReorderMappings(ids); err != nil {
		return err
	}
	dir := "下移"
	if up {
		dir = "上移"
	}
	a.Log.Info("%s映射「%s」", dir, m.Name)
	return nil
}

// SetMappingsEnabled turns every listed mapping on or off in one pass.
//
// It stops on the first failure so a partial batch leaves a clear toast;
// successful flips already mark the configuration dirty.
func (a *App) SetMappingsEnabled(ids []string, on bool) error {
	if len(ids) == 0 {
		return nil
	}
	n := 0
	for _, id := range ids {
		if err := a.SetMappingEnabled(id, on); err != nil {
			if n > 0 {
				a.markDirty()
			}
			return fmt.Errorf("已处理 %d 条，第 %d 条失败：%w", n, n+1, err)
		}
		n++
	}
	if on {
		a.setStatus(fmt.Sprintf("已启用 %d 条规则", n), false)
	} else {
		a.setStatus(fmt.Sprintf("已停用 %d 条规则", n), false)
	}
	return nil
}

// CopyAddress puts a mapping's public address on the clipboard, which is
// what someone sharing a rule actually needs.
func (a *App) CopyAddress(m model.Mapping) error {
	addr := nginx.HumanizeListen(m)
	if strings.HasPrefix(addr, ":") {
		addr = "本机" + addr
	}
	a.mu.Lock()
	a.copied = addr
	a.mu.Unlock()
	if a.Clipboard != nil {
		a.Clipboard(addr)
	}
	a.setStatus("已复制 "+addr, false)
	return nil
}

// CopyText puts arbitrary text on the clipboard — a diagnostic command,
// say — by the same path CopyAddress takes.
func (a *App) CopyText(text string) error {
	if text == "" {
		return nil
	}
	a.mu.Lock()
	a.copied = text
	a.mu.Unlock()
	if a.Clipboard != nil {
		a.Clipboard(text)
	}
	// A command can run long; the toast keeps to one line's worth.
	a.setStatus("已复制 "+truncateNotice(text, 60), false)
	return nil
}

// CloneMapping copies a mapping under a new name.
func (a *App) CloneMapping(id string) error {
	m, err := a.Store.GetMapping(id)
	if err != nil {
		return err
	}
	m.ID = ""
	m.Name += " 副本"
	m.Enabled = false
	// The listen port would collide with the original's, so clear it and
	// let the user pick one.
	m.ListenPort = 0
	if _, err := a.Store.CreateMapping(m); err != nil {
		return err
	}
	a.Log.Info("克隆映射「%s」", m.Name)
	a.markDirty()
	return nil
}

// Apply builds the rule set from the database and starts it.
//
// The engine applies the whole set at once: it opens every new listener
// before closing the ones it replaces, so a port that cannot be bound
// leaves the running rules untouched. That makes the operation
// transactional without a configuration file, a test run or a rollback.
func (a *App) Apply() error {
	a.mu.Lock()
	a.applyErr, a.applyWarn = "", ""
	a.applyErrPort, a.failKind = 0, ""
	a.mu.Unlock()

	spec, err := a.spec()
	if err != nil {
		// A rule set that cannot even be built — the whitelist refusing
		// to run with no addresses — is as much a failed apply as an
		// engine error, and just as invisible with the window closed.
		a.notifyUser("JumpBoard", "应用规则失败："+truncateNotice(err.Error(), 120))
		return err
	}
	res, err := a.Eng.Apply(spec)
	if err != nil {
		a.mu.Lock()
		a.applyErr = err.Error()
		a.applyErrPort, a.failKind = bindPort(err), "apply"
		// A new failure retires the diagnosis of the old one: advice about
		// a failure that no longer exists must not ride along under the
		// fresh red bar.
		a.diagErr, a.diagAdvice = "", nil
		a.mu.Unlock()
		a.Log.Error("应用规则失败: %v", err)
		a.refreshEngine()
		// The window may be closed in the tray when an apply fails — the
		// tray menu can run one too — so the failure is a notification,
		// not just a toast.
		a.notifyUser("JumpBoard", "应用规则失败："+truncateNotice(err.Error(), 120))
		return err
	}

	if len(res.Warnings) > 0 {
		a.mu.Lock()
		a.applyWarn = strings.Join(res.Warnings, "；")
		a.mu.Unlock()
		for _, w := range res.Warnings {
			a.Log.Warn("%s", w)
		}
	}
	a.refreshEngine()
	a.mu.Lock()
	a.applied = true
	a.mu.Unlock()
	if len(spec.Mappings) == 0 {
		a.setStatus("没有启用的映射，所有监听已关闭", false)
	} else {
		a.setStatus(fmt.Sprintf("已应用 %d 条映射", len(spec.Mappings)), false)
		// Worth a notification only when something is listening: an empty
		// rule set is a shutdown, and the tray menu's 应用配置 is the path
		// that runs without the window.
		a.notifyUser("JumpBoard", fmt.Sprintf("已应用 %d 条映射", len(spec.Mappings)))
	}
	a.Log.Info("规则已应用：%d 条映射", len(spec.Mappings))

	// A snapshot keeps the configuration the rules describe, so that it can
	// be exported to a real nginx or compared later.
	conf := nginx.Generate(spec.Mappings, nginx.GenOptions{WhitelistIPs: spec.WhitelistIPs})
	_ = a.Store.AddSnapshot(conf, "apply")
	_ = a.Store.TrimSnapshots(50)
	// The export is written for anyone who wants the same rules in nginx.
	// The watch takes the file's mtime as the app's own, so this write is
	// never reported as a foreign edit.
	_ = os.WriteFile(filepath.Join(a.Paths.ExportDir, "nginx.conf"), []byte(conf), 0o644)
	if a.conf != nil {
		a.conf.noteSelfWrite()
	}

	a.mu.Lock()
	a.dirty = false
	a.mu.Unlock()
	return nil
}

// truncateNotice keeps a notification's body to one banner's worth: the
// full error stays in the log and the window either way.
func truncateNotice(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// ApplyError returns why the last apply failed, for the detail line the
// user can expand.
func (a *App) ApplyError() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.applyErr
}

// bindPort pulls the listen port out of a bind failure, so the diagnosis
// can name the mapping involved without parsing error text.
func bindPort(err error) int {
	var bind *engine.ErrBind
	if errors.As(err, &bind) {
		return bind.Port
	}
	return 0
}

// ApplyWarning returns what the engine could not honor.
func (a *App) ApplyWarning() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.applyWarn
}

// StartEngine re-applies the rules: the engine has no separate start, and
// a stopped engine is one with no listeners.
func (a *App) StartEngine() error { return a.Apply() }

// StopEngine closes every listener.
func (a *App) StopEngine() error {
	if err := a.Eng.Stop(); err != nil {
		return err
	}
	a.refreshEngine()
	a.Log.Info("已停止，所有转发失效")
	a.setStatus("已停止，所有转发失效", false)
	return nil
}

// TestConfig applies the rule set without keeping it, which is what the
// old nginx -t did: a port that is taken is reported before the running
// rules change.
func (a *App) TestConfig() error {
	spec, err := a.spec()
	if err != nil {
		return err
	}
	if err := a.Eng.Check(spec); err != nil {
		a.mu.Lock()
		a.applyErr = err.Error()
		a.applyErrPort, a.failKind = bindPort(err), "check"
		// As with a failed apply: the diagnosis belongs to the failure that
		// asked for it, not to whichever red bar shows next.
		a.diagErr, a.diagAdvice = "", nil
		a.mu.Unlock()
		return fmt.Errorf("预检失败：%v", err)
	}
	a.setStatus("预检通过：端口可用，上游地址可解析", false)
	return nil
}

// Rollback restores a snapshot's configuration into the rule set.
//
// A snapshot holds nginx directives, not the app's own model, so the
// mappings it names are imported back into the database and then applied:
// the snapshot becomes the truth for both.
func (a *App) RollbackSnapshot(sn model.Snapshot) error {
	mappings, err := nginx.Parse(sn.Content)
	if err != nil {
		return fmt.Errorf("快照无法解析：%v", err)
	}
	// Keep a record of what the rollback replaces, so the history grows
	// instead of being the thing a rollback throws away. The snapshot of
	// the rules as they stand now is what a second rollback returns to.
	if current, _, genErr := a.NginxExport(); genErr != nil {
		a.Log.Warn("回滚前保存当前配置失败：%v", genErr)
	} else if err := a.Store.AddSnapshot(current, "rollback"); err != nil {
		return fmt.Errorf("保存回滚前的配置失败：%v", err)
	}
	if err := a.Store.ReplaceAllMappings(mappings); err != nil {
		return err
	}
	if err := a.Reload(); err != nil {
		return err
	}
	if err := a.Apply(); err != nil {
		return err
	}
	a.Log.Info("已回滚到快照 %s（%d 条映射已恢复）", sn.ID, len(mappings))
	a.setStatus(fmt.Sprintf("已回滚：%d 条映射已恢复", len(mappings)), false)
	return nil
}

// AddWhitelist allows an address.
func (a *App) AddWhitelist(ip, note string) error {
	ip = model.StripInject(ip)
	if !model.ValidIPOrCIDR(ip) {
		return fmt.Errorf("IP 格式不正确，应为 1.2.3.4 或 10.0.0.0/24")
	}
	if _, err := a.Store.AddWhitelist(ip, note); err != nil {
		return err
	}
	a.Log.Info("白名单添加 %s", ip)
	a.markDirty()
	return nil
}

// DeleteWhitelist removes an address.
func (a *App) DeleteWhitelist(id int64) error {
	if err := a.Store.DeleteWhitelist(id); err != nil {
		return err
	}
	a.markDirty()
	return nil
}

// ToggleWhitelist turns one address on or off.
func (a *App) ToggleWhitelist(id int64, on bool) error {
	if err := a.Store.SetWhitelistEnabled(id, on); err != nil {
		return err
	}
	a.markDirty()
	return nil
}

// Example shows how an export looks: the app keeps the format compatible
// with the Electron build's backups.
func (a *App) Export() (services.Payload, error) { return services.Export(a.Store) }

// ExportBackupJSON is the backup the UI saves. The dialog stays in the UI;
// the payload and its formatting live here.
func (a *App) ExportBackupJSON() ([]byte, int, error) {
	payload, err := a.Export()
	if err != nil {
		return nil, 0, err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return data, len(payload.Mappings), nil
}

// WriteExportFile writes bytes the user chose a path for.
func (a *App) WriteExportFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// ReadImportFile reads a backup the user chose.
func (a *App) ReadImportFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// NginxExport is the current rules as a standard nginx.conf, for someone
// who wants to run them on a real nginx.
// NginxExportOrEmpty is NginxExport's shape for a frame that only wants
// the text: the error is not worth failing a read-only view over.
func (a *App) NginxExportOrEmpty() string {
	conf, _, err := a.NginxExport()
	if err != nil {
		return ""
	}
	return conf
}

func (a *App) NginxExport() (string, int, error) {
	spec, err := a.Spec()
	if err != nil {
		return "", 0, err
	}
	conf := nginx.Generate(spec.Mappings, nginx.GenOptions{WhitelistIPs: spec.WhitelistIPs})
	return conf, len(spec.Mappings), nil
}

// Import replaces the data with a backup.
func (a *App) Import(data []byte) error {
	res, err := services.Import(a.Store, data)
	if err != nil {
		return err
	}
	a.Log.Info("导入完成：%d 条映射，%d 个快照，跳过 %d 条", res.MappingsImported, res.SnapshotsImported, res.Skipped)
	a.markDirty()
	return nil
}

// importKind names what a dropped file is by its extension: a JumpBoard
// backup, an nginx configuration, or nothing this app can use.
func importKind(ext string) string {
	switch strings.ToLower(ext) {
	case ".json":
		return "json"
	case ".conf":
		return "conf"
	default:
		return ""
	}
}

// ImportDroppedFiles takes the files dropped on the window and imports
// each one by what it is: a .json goes through the backup import, which
// replaces the rules with the backup's; a .conf's server blocks merge in
// as new mappings. Anything else is named and skipped — a drop is a
// gesture, not a dialog, so refusals are a status line, never a crash,
// and one bad file does not stop the ones behind it.
//
// It runs on the caller's background goroutine; the summary it leaves in
// the status line covers the whole drop at once.
func (a *App) ImportDroppedFiles(paths []string) error {
	var imported, skippedFiles []string
	for _, p := range paths {
		base := filepath.Base(p)
		data, err := os.ReadFile(p)
		if err != nil {
			a.Fail("读取 %s 失败：%v", base, err)
			continue
		}
		switch importKind(filepath.Ext(p)) {
		case "json":
			if err := a.Import(data); err != nil {
				a.Fail("导入 %s 失败：%v", base, err)
				continue
			}
			imported = append(imported, base+"（备份替换）")
		case "conf":
			n, err := a.ImportNginxConf(data)
			if err != nil {
				a.Fail("导入 %s 失败：%v", base, err)
				continue
			}
			// The bar remembers the path, not the bytes: by the time the
			// user asks for an interpretation the file may already be gone,
			// and that read failure is then reported as an error rather
			// than explained from a copy the user cannot see. A dropped
			// backup replaces the rules and is no interpretation entry, so
			// it records nothing.
			a.setConfImport(ConfImport{Path: p, Name: base, Count: n})
			imported = append(imported, fmt.Sprintf("%s（合并 %d 条）", base, n))
		default:
			skippedFiles = append(skippedFiles, base)
		}
	}
	if len(imported) > 0 {
		a.Info("已导入 %d 个文件：%s", len(imported), strings.Join(imported, "、"))
	}
	if len(skippedFiles) > 0 {
		a.Fail("已跳过不支持的文件：%s（支持 .json 备份与 .conf 配置）", strings.Join(skippedFiles, "、"))
	}
	return nil
}

// ImportNginxConf merges an nginx.conf's server blocks in as new
// mappings, for a conf dragged onto the window. Unlike Import — a
// backup, which replaces everything — a conf is additions.
func (a *App) ImportNginxConf(data []byte) (int, error) {
	added, skipped, err := services.ImportNginxConf(a.Store, data)
	if err != nil {
		return 0, err
	}
	a.Log.Info("从 nginx 配置导入 %d 条映射，跳过 %d 块", added, skipped)
	a.markDirty()
	return added, nil
}

// UpstreamHostSuggestions lists the upstream hosts already in use, for
// the form's autocomplete: an internal network has a handful of servers
// and typing their names by hand is the form's most repetitive step.
func (a *App) UpstreamHostSuggestions() []string {
	mappings, err := a.Store.ListMappings()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, 16)
	for _, m := range mappings {
		h := m.UpstreamHost
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

// Wipe clears every piece of data and rebuilds the engine.
func (a *App) Wipe() error {
	_ = a.Eng.Stop()
	if err := a.Store.Wipe(); err != nil {
		return err
	}
	if err := a.Store.EnsureLocalHost(); err != nil {
		return err
	}
	a.refreshEngine()
	a.Log.Warn("已清空所有数据")
	// The settings live in the store and are mirrored in memory; a wipe
	// resets the store, so the mirror has to be reloaded or the window
	// keeps showing the old theme and, worse, an "enabled" whitelist that
	// the next apply reads as off — which would allow every address.
	a.theme = ThemeChoice(a.Store.Setting(store.SettingTheme, string(ThemeSystem)))
	a.whitelistOn = a.Store.WhitelistOn()
	a.nginxLogRead = a.Store.SettingBool(store.SettingNginxLogRead, true)
	if name, err := a.Store.HostName(); err == nil {
		a.hostName = name
	}
	a.mu.Lock()
	a.dirty = false
	a.mu.Unlock()
	return nil
}

// NginxLogs reads the engine's access log.
func (a *App) NginxLogs(lines int) (access, errs []string) {
	if !a.nginxLogRead {
		return nil, nil
	}
	got, err := services.TailFile(filepath.Join(a.Paths.LogsDir, "access.log"), lines)
	if err == nil {
		return got, nil
	}
	// "No log yet" and "cannot read the log" are different answers: the
	// first is an engine that has served nothing, the second is a problem
	// the log page exists to reveal. A missing file is the first; anything
	// else (permissions, a directory in its place) reaches the page.
	if os.IsNotExist(err) {
		return nil, nil
	}
	return nil, []string{"读取访问日志失败：" + err.Error()}
}

// ClearNginxLogs truncates the access log.
func (a *App) ClearNginxLogs() error {
	path := filepath.Join(a.Paths.LogsDir, "access.log")
	if err := os.Truncate(path, 0); err != nil {
		// A log that was never written is already empty; other failures
		// must say so, or the user clears a problem and believes it gone.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	a.Log.Info("访问日志已清空")
	return nil
}

// AppLogExport is the operation log as plain text, oldest first, for the
// export button. The page shows the newest five hundred; a file someone
// keeps should hold everything there is.
func (a *App) AppLogExport() ([]byte, int, error) {
	logs, err := a.Store.ListLogs(10000)
	if err != nil {
		return nil, 0, err
	}
	var b strings.Builder
	for i := len(logs) - 1; i >= 0; i-- {
		l := logs[i]
		fmt.Fprintf(&b, "%s %s %s\n",
			time.UnixMilli(l.Timestamp).Format("2006-01-02 15:04:05"),
			strings.ToUpper(string(l.Level)), l.Message)
	}
	return []byte(b.String()), len(logs), nil
}

// AccessLogExport is the engine's access log as it stands on disk. A log
// that was never written exports as an empty file — that is what it is.
func (a *App) AccessLogExport() ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(a.Paths.LogsDir, "access.log"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// ClearAppLogs empties the operation log.
func (a *App) ClearAppLogs() error {
	if err := a.Store.ClearLogs(); err != nil {
		return err
	}
	a.Log.Info("操作日志已清空")
	return nil
}

// CheckNow probes one mapping at once.
func (a *App) CheckNow(m model.Mapping) error {
	status := services.CheckMapping(m)
	if err := a.Store.SetMappingHealth(m.ID, status, time.Now().UnixMilli()); err != nil {
		return err
	}
	a.setStatus(fmt.Sprintf("「%s」%s", m.Name, healthLabel(status)), status == model.HealthHealthy)
	return nil
}

// healthLabel says a status the way the list does.
func healthLabel(s model.HealthStatus) string {
	switch s {
	case model.HealthHealthy:
		return "健康"
	case model.HealthUnhealthy:
		return "不可达"
	}
	return "未知"
}
