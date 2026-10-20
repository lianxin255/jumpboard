package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
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
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
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
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
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

// homeDir returns the user's home, for the save dialog's default folder.
func homeDir() string {
	dir, err := mygo.App.Path(mygo.PathHome)
	if err != nil {
		return "."
	}
	return dir
}
