package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"github.com/lianxin255/nginx-jump-gui/internal/nginx"
)

// exportBackup asks where to save a backup and writes it.
func (v *View) exportBackup() {
	a := v.App
	payload, err := a.Export()
	if err != nil {
		a.Fail("导出失败：%v", err)
		return
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		a.Fail("导出失败：%v", err)
		return
	}
	name := "nginx-jump-backup-" + time.Now().Format("2006-01-02") + ".json"
	if a.SaveDialog == nil {
		return
	}
	path, err := a.SaveDialog(mygo.SaveDialogOptions{
		Title:       "导出备份",
		DefaultPath: filepath.Join(homeDir(), "Downloads", name),
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || path == "" {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		a.Fail("写入失败：%v", err)
		return
	}
	a.Info("已导出 %d 条映射到 %s", len(payload.Mappings), filepath.Base(path))
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
	data, err := os.ReadFile(paths[0])
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
	spec, err := a.Spec()
	if err != nil {
		a.Fail("导出失败：%v", err)
		return
	}
	conf := nginx.Generate(spec.Mappings, nginx.GenOptions{WhitelistIPs: spec.WhitelistIPs})
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
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		a.Fail("写入失败：%v", err)
		return
	}
	a.Info("已导出 %d 条规则的 nginx 配置", len(spec.Mappings))
}

// homeDir returns the user's home, for the save dialog's default folder.
func homeDir() string {
	dir, err := mygo.App.Path(mygo.PathHome)
	if err != nil {
		return "."
	}
	return dir
}
