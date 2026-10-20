package appt

import (
	"fmt"
	"os"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
	"github.com/lianxin255/nginx-jump-gui/internal/nginx"
	"github.com/lianxin255/nginx-jump-gui/internal/services"
	"github.com/lianxin255/nginx-jump-gui/internal/store"
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
	a.formEdit = m
	if m != nil {
		a.form = FormFromMapping(*m)
	} else {
		a.form = NewForm()
	}
	a.formOpen = true
}

// OpenFormFromTemplate opens the form prefilled by a template.
func (a *App) OpenFormFromTemplate(t model.Template) {
	a.formEdit = nil
	a.form = FormFromTemplate(t)
	a.formOpen = true
}

// FormOpen reports whether the form is up, and returns its values.
func (a *App) FormState() (open bool, editing *model.Mapping, f FormValues, errs map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.formOpen, a.formEdit, a.form, a.formErrs
}

// SetForm stores the values the inputs edited.
func (a *App) SetForm(f FormValues) { a.form = f }

// CloseForm hides the form.
func (a *App) CloseForm() { a.formOpen = false; a.formEdit = nil }

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

// SetTheme stores the appearance and applies it at once.
func (a *App) SetTheme(t ThemeChoice) {
	a.theme = t
	_ = a.Store.SetSetting(store.SettingTheme, string(t))
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
func (a *App) SetWhitelistOn(on bool) {
	a.whitelistOn = on
	_ = a.Store.SetWhitelistOn(on)
	if on {
		a.Log.Info("白名单已开启，仅允许列表中的 IP 访问")
	} else {
		a.Log.Info("白名单已关闭，全部允许")
	}
	a.notify()
}

// NginxLogRead reports whether the app reads nginx's own logs.
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

// Status returns the toast to show, if any.
func (a *App) Status() (Status, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status.Text == "" {
		return Status{}, false
	}
	return a.status, true
}

// ClearStatus drops the toast.
func (a *App) ClearStatus() {
	a.mu.Lock()
	a.status = Status{}
	a.mu.Unlock()
}

// --- actions, which change data and run off the main thread ---

// SaveForm validates the form and stores the mapping.
func (a *App) SaveForm() error {
	m, errs, ok := a.form.Validate()
	a.mu.Lock()
	a.formErrs = errs
	a.mu.Unlock()
	if !ok {
		a.notify()
		return fmt.Errorf("表单有 %d 处需要修正", len(errs))
	}
	if a.formEdit != nil {
		m.ID = a.formEdit.ID
		m.HealthStatus = a.formEdit.HealthStatus
		m.CreatedAt = a.formEdit.CreatedAt
		m.SortOrder = a.formEdit.SortOrder
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
func (a *App) SetMappingEnabled(id string, on bool) error {
	m, err := a.Store.GetMapping(id)
	if err != nil {
		return err
	}
	if err := a.Store.SetMappingEnabled(id, on); err != nil {
		return err
	}
	if on {
		a.Log.Info("启用映射「%s」", m.Name)
	} else {
		a.Log.Info("停用映射「%s」", m.Name)
	}
	a.markDirty()
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

// ReorderMappings applies a new order from the table's drag.
func (a *App) ReorderMappings(ids []string) error {
	if err := a.Store.ReorderMappings(ids); err != nil {
		return err
	}
	a.markDirty()
	return nil
}

// Apply generates a configuration from the database and applies it.
//
// Failure is reported by stage: a bad configuration is the user's to fix
// and shows as a warning, while a reload that fails is the app's problem
// and shows as an error.
func (a *App) Apply() error {
	a.mu.Lock()
	a.applyErr = ""
	a.mu.Unlock()

	// A whitelist with no addresses would deny everyone, so refuse to
	// write a configuration that does.
	var wl []string
	if a.Store.WhitelistOn() {
		ips, err := a.Store.EnabledWhitelistIPs()
		if err != nil {
			return err
		}
		if len(ips) == 0 {
			return fmt.Errorf("白名单已开启但没有任何 IP，请先添加或在设置里关闭白名单")
		}
		wl = ips
	}
	mappings, err := a.Store.ListEnabledMappings()
	if err != nil {
		return err
	}
	conf := nginx.Generate(mappings, nginx.GenOptions{WhitelistIPs: wl})

	// A stopped engine only needs the file written and checked: nginx
	// loads it at the next start, and reloading would fail.
	skipReload := a.Eng.Status() != "running"
	res := a.Eng.Apply(conf, skipReload)

	switch {
	case res.OK && skipReload:
		a.setStatus("配置已写入并通过校验，启动 nginx 后生效", false)
		a.Log.Info("配置已写入（引擎未运行），%d 条映射", len(mappings))
	case res.OK:
		a.setStatus("配置已应用", false)
		a.Log.Info("配置已应用，%d 条映射", len(mappings))
	case res.Stage == "test":
		a.mu.Lock()
		a.applyErr = res.Output
		a.mu.Unlock()
		a.Log.Warn("配置校验失败: %s", res.Output)
		return fmt.Errorf("配置校验失败，已回滚")
	default:
		a.mu.Lock()
		a.applyErr = res.Output
		a.mu.Unlock()
		a.Log.Error("配置应用失败（%s）: %s", res.Stage, res.Output)
		return fmt.Errorf("应用失败（%s），已回滚", res.Stage)
	}

	if res.OK {
		_ = a.Store.AddSnapshot(conf, "reload")
		_ = a.Store.TrimSnapshots(50)
		a.mu.Lock()
		a.dirty = false
		a.mu.Unlock()
	}
	return nil
}

// ApplyError returns why the last apply failed, for the detail line the
// user can expand.
func (a *App) ApplyError() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.applyErr
}

// StartEngine starts nginx with the configuration on disk.
func (a *App) StartEngine() error {
	if err := a.Eng.Start(); err != nil {
		a.Log.Error("启动 nginx 失败: %v", err)
		return err
	}
	a.Log.Info("nginx 已启动")
	a.setStatus("nginx 已启动", false)
	return nil
}

// StopEngine stops nginx.
func (a *App) StopEngine() error {
	if err := a.Eng.Stop(); err != nil {
		return err
	}
	a.Log.Info("nginx 已停止，所有转发失效")
	a.setStatus("nginx 已停止", false)
	return nil
}

// TestConfig runs nginx -t on the file on disk.
func (a *App) TestConfig() error {
	ok, out := a.Eng.Test()
	if !ok {
		a.mu.Lock()
		a.applyErr = out
		a.mu.Unlock()
		return fmt.Errorf("配置校验失败")
	}
	a.setStatus("配置校验通过", false)
	return nil
}

// Rollback restores a snapshot.
func (a *App) RollbackSnapshot(sn model.Snapshot) error {
	skipReload := a.Eng.Status() != "running"
	res := a.Eng.Apply(sn.Content, skipReload)
	if !res.OK {
		return fmt.Errorf("回滚失败（%s）: %s", res.Stage, res.Output)
	}
	// The database and the restored file now differ; applying would bring
	// them back together, which the dirty flag says.
	a.markDirty()
	a.Log.Info("已回滚到快照 %s", sn.ID)
	a.setStatus("已回滚到所选快照；点“应用配置”会用当前映射覆盖它", false)
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

// Wipe clears every piece of data and rebuilds the engine.
func (a *App) Wipe() error {
	if a.Eng.Status() == "running" {
		_ = a.Eng.Stop()
	}
	if err := a.Store.Wipe(); err != nil {
		return err
	}
	if err := a.Store.EnsureLocalHost(); err != nil {
		return err
	}
	a.writeConfFromStore()
	a.Log.Warn("已清空所有数据")
	a.mu.Lock()
	a.dirty = false
	a.mu.Unlock()
	return nil
}

// ReloadNginxLogs reads nginx's own logs from the prefix.
func (a *App) NginxLogs(lines int) (access, errs []string) {
	if !a.nginxLogRead {
		return nil, nil
	}
	logsDir := a.Paths.PrefixDir + "/logs"
	for _, name := range []string{"access.log", "stream_access.log"} {
		if got, err := services.TailFile(logsDir+"/"+name, lines); err == nil {
			access = append(access, got...)
		}
	}
	if got, err := services.TailFile(logsDir+"/error.log", lines); err == nil {
		errs = got
	}
	return access, errs
}

// ClearNginxLogs truncates nginx's own logs.
func (a *App) ClearNginxLogs() error {
	logsDir := a.Paths.PrefixDir + "/logs"
	for _, name := range []string{"access.log", "stream_access.log", "error.log"} {
		if err := os.Truncate(logsDir+"/"+name, 0); err != nil {
			return nil // missing files are fine
		}
	}
	a.Log.Info("nginx 日志已清空")
	return nil
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
