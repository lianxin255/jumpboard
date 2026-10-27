package ui

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

// exportBackup asks where to save a backup and writes what the app built.
func (v *View) exportBackup() {
	a := v.App
	data, n, err := a.ExportBackupJSON()
	if err != nil {
		a.Fail("导出失败：%v", err)
		return
	}
	if a.SaveDialog == nil {
		return
	}
	name := "jumpboard-backup-" + time.Now().Format("2006-01-02") + ".json"
	path, err := a.SaveDialog(mygo.SaveDialogOptions{
		Title:       "导出备份",
		DefaultPath: filepath.Join(homeDir(), "Downloads", name),
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || path == "" {
		return
	}
	if err := a.WriteExportFile(path, data); err != nil {
		a.Fail("写入失败：%v", err)
		return
	}
	a.Info("已导出 %d 条映射到 %s", n, filepath.Base(path))
}

// importBackup asks for a file and replaces the data with it.
func (v *View) importBackup() {
	a := v.App
	if a.OpenDialog == nil {
		return
	}
	paths, err := a.OpenDialog(mygo.OpenDialogOptions{
		Title:   "导入备份",
		Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	data, err := a.ReadImportFile(paths[0])
	if err != nil {
		a.Fail("读取失败：%v", err)
		return
	}
	a.RunBackground(func() error { return a.Import(data) })
}

// exportNginx writes the current rules as an nginx.conf, for someone who
// wants to run them on a real nginx.
func (v *View) exportNginx() {
	a := v.App
	conf, n, err := a.NginxExport()
	if err != nil {
		a.Fail("导出失败：%v", err)
		return
	}
	if a.SaveDialog == nil {
		return
	}
	path, err := a.SaveDialog(mygo.SaveDialogOptions{
		Title:       "导出 nginx 配置",
		DefaultPath: filepath.Join(homeDir(), "Downloads", "nginx.conf"),
		Filters:     []mygo.FileFilter{{Name: "nginx.conf", Extensions: []string{"conf"}}},
	})
	if err != nil || path == "" {
		return
	}
	if err := a.WriteExportFile(path, []byte(conf)); err != nil {
		a.Fail("写入失败：%v", err)
		return
	}
	a.Info("已导出 %d 条规则的 nginx 配置", n)
}

// exportAppLogs asks where to save the operation log and writes it.
func (v *View) exportAppLogs() {
	a := v.App
	data, n, err := a.AppLogExport()
	if err != nil {
		a.Fail("导出失败：%v", err)
		return
	}
	if a.SaveDialog == nil {
		return
	}
	name := "jumpboard-logs-" + time.Now().Format("2006-01-02") + ".txt"
	path, err := a.SaveDialog(mygo.SaveDialogOptions{
		Title:       "导出操作日志",
		DefaultPath: filepath.Join(homeDir(), "Downloads", name),
		Filters:     []mygo.FileFilter{{Name: "Text", Extensions: []string{"txt", "log"}}},
	})
	if err != nil || path == "" {
		return
	}
	if err := a.WriteExportFile(path, data); err != nil {
		a.Fail("写入失败：%v", err)
		return
	}
	a.Info("已导出 %d 条操作日志", n)
}

// exportAccessLog asks where to save the engine's access log and writes it.
func (v *View) exportAccessLog() {
	a := v.App
	data, err := a.AccessLogExport()
	if err != nil {
		a.Fail("读取访问日志失败：%v", err)
		return
	}
	if a.SaveDialog == nil {
		return
	}
	path, err := a.SaveDialog(mygo.SaveDialogOptions{
		Title:       "导出访问日志",
		DefaultPath: filepath.Join(homeDir(), "Downloads", "jumpboard-access.log"),
		Filters:     []mygo.FileFilter{{Name: "Log", Extensions: []string{"log", "txt"}}},
	})
	if err != nil || path == "" {
		return
	}
	if err := a.WriteExportFile(path, data); err != nil {
		a.Fail("写入失败：%v", err)
		return
	}
	a.Info("已导出访问日志（%s）", filesizeLabel(len(data)))
}

// filesizeLabel renders a byte count the way the rest of the app does.
func filesizeLabel(n int) string {
	const kb, mb = 1 << 10, 1 << 20
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// homeDir returns the user's home, for the save dialog's default folder.
func homeDir() string {
	dir, err := mygo.App.Path(mygo.PathHome)
	if err != nil {
		return "."
	}
	return dir
}

// importDropped hands the dropped paths to the app's importer on the
// background goroutine: the decision of what each file is, and the
// summary of what happened, live with the app — the view only owns the
// gesture.
func (v *View) importDropped(paths []string) {
	a := v.App
	a.RunBackground(func() error { return a.ImportDroppedFiles(paths) })
}
