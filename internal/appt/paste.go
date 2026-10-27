package appt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lianxin255/jumpboard/internal/nginx"
	"github.com/lianxin255/jumpboard/internal/services"
)

// pasteKind names what a clipboard text is: a JumpBoard backup, an nginx
// configuration, or nothing this app can use. It is the same distinction
// importKind makes for a file's extension, but made from the text itself,
// because a clipboard has no extension.
type pasteKind int

const (
	pasteUnknown pasteKind = iota
	pasteBackup
	pasteConf
)

// PastePreview is what a paste-import would do, shown before it is done.
type PastePreview struct {
	Kind    pasteKind
	Detail  string
	Replace bool // a backup replaces the whole rule set
	Count   int  // mappings a backup would restore, or a conf would add
	Data    []byte
}

// PastePreview returns the pending paste-import, or nil when none is.
func (a *App) PastePreview() *PastePreview {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pastePending == nil {
		return nil
	}
	p := *a.pastePending
	return &p
}

// StartPasteImport reads the clipboard and decides what the text is. The
// preview is only set when it is something the app can import: the other
// answers are a plain refusal, which is what a handful of copied prose
// deserves.
func (a *App) StartPasteImport() error {
	if a.ReadClipboard == nil {
		return fmt.Errorf("剪贴板不可用")
	}
	text, err := a.ReadClipboard()
	if err != nil {
		return fmt.Errorf("读取剪贴板失败：%v", err)
	}
	pv, ok := sniffPaste(text)
	if !ok {
		return fmt.Errorf("剪贴板内容无法识别：支持 JumpBoard 备份（JSON）或 nginx 配置")
	}

	a.mu.Lock()
	a.pastePending = pv
	a.mu.Unlock()
	a.notify()
	return nil
}

// CancelPasteImport folds the preview away without importing.
func (a *App) CancelPasteImport() {
	a.mu.Lock()
	a.pastePending = nil
	a.mu.Unlock()
	a.notify()
}

// ConfirmPasteImport does what the preview described. A backup replaces
// the rule set; a conf's server blocks merge in as additions.
func (a *App) ConfirmPasteImport() error {
	a.mu.Lock()
	pv := a.pastePending
	a.mu.Unlock()
	if pv == nil {
		return fmt.Errorf("没有待导入的剪贴板内容")
	}
	defer a.CancelPasteImport()

	switch pv.Kind {
	case pasteBackup:
		res, err := services.Import(a.Store, pv.Data)
		if err != nil {
			return err
		}
		a.Log.Info("从剪贴板导入备份：%d 条映射，%d 个快照，跳过 %d 条",
			res.MappingsImported, res.SnapshotsImported, res.Skipped)
	case pasteConf:
		added, skipped, err := services.ImportNginxConf(a.Store, pv.Data)
		if err != nil {
			return err
		}
		a.Log.Info("从剪贴板导入 nginx 配置：新增 %d 条映射，跳过 %d 块", added, skipped)
	default:
		return fmt.Errorf("剪贴板内容无法识别")
	}
	a.markDirty()
	// The paste lands in the table the user is looking at, so the frame
	// has to show it: markDirty alone records the state, Reload refreshes
	// what the table reads.
	return a.Reload()
}

// sniffPaste decides what a clipboard text is. The order matters: a JSON
// backup is tried first, because an nginx.conf is never JSON while a
// backup could theoretically be mistaken for one — and the JSON parse is
// the stricter test, requiring the version the app writes.
func sniffPaste(text string) (*PastePreview, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false
	}
	if pv, ok := sniffBackup([]byte(text)); ok {
		return pv, true
	}
	if pv, ok := sniffConf([]byte(text)); ok {
		return pv, true
	}
	return nil, false
}

// sniffBackup reads a clipboard text as the backup the app writes.
func sniffBackup(data []byte) (*PastePreview, bool) {
	var p services.Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, false
	}
	// The version is the one thing a JSON object has to carry for it to
	// be a backup of this app; without it, whatever JSON this is, it is
	// not ours.
	if p.Version != 2 && p.Version != 1 {
		return nil, false
	}
	return &PastePreview{
		Kind:    pasteBackup,
		Replace: true,
		Count:   len(p.Mappings),
		Data:    data,
		Detail:  fmt.Sprintf("将用备份中的 %d 条映射替换现有全部规则", len(p.Mappings)),
	}, true
}

// sniffConf reads a clipboard text as an nginx configuration.
func sniffConf(data []byte) (*PastePreview, bool) {
	parsed, err := nginx.Parse(string(data))
	if err != nil || len(parsed) == 0 {
		return nil, false
	}
	return &PastePreview{
		Kind:   pasteConf,
		Count:  len(parsed),
		Data:   data,
		Detail: fmt.Sprintf("将新增 %d 条映射（与现有规则合并）", len(parsed)),
	}, true
}
