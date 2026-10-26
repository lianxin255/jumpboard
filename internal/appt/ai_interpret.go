package appt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/nginx"
)

// interpretFileMaxBytes bounds the file one interpretation may read. A
// bigger file is refused rather than truncated: a conf or a log cut in
// half yields a confident explanation of a file that does not exist, so
// the honest answer is the limit, not a summary of the first 256 KB.
const interpretFileMaxBytes = 256 << 10

// ConfImport names the last dropped .conf that merged in, for the bar on
// the mappings page. It is an entry's display record only: the path is
// kept, not the bytes, so a file moved away before the interpretation
// reads it fails as an error rather than being explained from a copy the
// user cannot see anymore.
type ConfImport struct {
	Path  string
	Name  string
	Count int
}

// errAINotReady is the refusal every interpretation entry answers before
// any local precondition, so a user without AI configured hears that
// rather than "the log is empty" or "nothing was imported".
func (a *App) errAINotReady() error {
	if a.AIReady() {
		return nil
	}
	return fmt.Errorf("还没有配置 AI 服务，请先在设置里填写")
}

// InterpretFile interprets one file the user picked: .conf, .log or .txt
// — an external nginx error.log enters here. kind "" infers from the
// name.
func (a *App) InterpretFile(kind, path string) error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	return a.interpretPath(kind, path, "panel")
}

// InterpretText interprets content already in memory — the engine's own
// access log — and is the path the logs page takes.
func (a *App) InterpretText(kind, name, content string) error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("没有可解读的内容")
	}
	return a.interpret(kind, name, content, "logs")
}

// InterpretDroppedConf interprets the last dropped .conf that merged in,
// from the bar the import left on the mappings page.
func (a *App) InterpretDroppedConf() error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	a.mu.Lock()
	imp := a.confImport
	a.mu.Unlock()
	if imp == nil {
		return fmt.Errorf("还没有刚导入的配置可解读")
	}
	return a.interpretPath("conf", imp.Path, "import")
}

// InterpretAccessLog interprets the engine's current access.log, the
// logs page's one interpretation.
func (a *App) InterpretAccessLog() error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	if !a.NginxLogRead() {
		return fmt.Errorf("访问日志读取已关闭，请先在日志页开启")
	}
	access, _ := a.NginxLogs(200)
	if len(access) == 0 {
		return fmt.Errorf("访问日志还没有内容")
	}
	return a.InterpretText("access", "access.log", strings.Join(access, "\n"))
}

// interpretPath reads one file (capped), infers nothing itself, and runs
// the interpretation. kind "" means infer from the file's name.
func (a *App) interpretPath(kind, path, source string) error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	if kind == "" {
		kind = interpretKindForPath(path)
	}
	name := filepath.Base(path)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败：%w", name, err)
	}
	if info.Size() > interpretFileMaxBytes {
		return fmt.Errorf("%s 超过 256 KB，不能解读", name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败：%w", name, err)
	}
	return a.interpret(kind, name, string(data), source)
}

// interpret is the shared core of every interpretation entry: the ready
// check, the busy gate, the one request with its deadline, and the state
// the card reads. The error also returns, so RunBackground shows it as a
// toast beside the card. A conf is parsed locally here, before anything
// travels — who listens and where it forwards is this machine's answer,
// and the model explains it rather than counting it.
func (a *App) interpret(kind, name, content, source string) error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	// The busy check and the busy flip are one locked step: two entries
	// asking at once must not start two requests.
	a.mu.Lock()
	if a.interpretBusy {
		a.mu.Unlock()
		return fmt.Errorf("解读还在进行中")
	}
	a.interpretBusy, a.interpretErr, a.interpretation = true, "", nil
	a.interpretTarget, a.interpretSource = name, source
	a.mu.Unlock()

	in := ai.InterpretInput{Kind: kind, Name: name, Content: content}
	if kind == "conf" {
		in.ConfSummary, in.Notes = confSummaryOf(content)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := a.aiClient().Interpret(ctx, in)

	a.mu.Lock()
	a.interpretBusy = false
	a.interpretTarget, a.interpretSource = name, source
	if err != nil {
		a.interpretErr, a.interpretation = err.Error(), nil
	} else {
		a.interpretErr, a.interpretation = "", &out
	}
	a.mu.Unlock()
	if err != nil {
		a.Log.Warn("AI 解读失败: %v", err)
	} else {
		a.Log.Info("AI 解读完成：%s", name)
	}
	a.notify()
	return err
}

// confSummaryOf is the deterministic half of a conf's interpretation: one
// mappingSummary line per server block the local parse found, at most 40
// lines with the count of the rest. A conf the parser cannot read is not
// a dead end — the raw text still travels, with a note that says so.
func confSummaryOf(conf string) (lines []string, notes []string) {
	mappings, err := nginx.Parse(conf)
	if err != nil {
		return nil, []string{"本地解析失败：" + ai.Truncate(err.Error(), 200) + "，按原文解读"}
	}
	for i, m := range mappings {
		if i == 40 {
			lines = append(lines, fmt.Sprintf("（其余 %d 个 server 块未列入摘要）", len(mappings)-40))
			break
		}
		lines = append(lines, mappingSummary(m))
	}
	return lines, nil
}

// interpretKindForPath says what a file is by its name: a .conf is an
// nginx configuration, a name mentioning error is an error log, and
// anything else is read as an access log — the kind the engine writes
// most of.
func interpretKindForPath(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".conf") {
		return "conf"
	}
	if strings.Contains(strings.ToLower(filepath.Base(path)), "error") {
		return "error"
	}
	return "access"
}

// Interpretation returns what the interpretation is doing, for the card
// on the page that asked.
func (a *App) Interpretation() (busy bool, errText string, interp *ai.Interpretation, target, source string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.interpretBusy, a.interpretErr, a.interpretation, a.interpretTarget, a.interpretSource
}

// DismissInterpretation folds the interpretation card away, back to the
// entry that opened it.
func (a *App) DismissInterpretation() {
	a.mu.Lock()
	a.interpretErr, a.interpretation = "", nil
	a.interpretTarget, a.interpretSource = "", ""
	a.mu.Unlock()
	a.notify()
}

// LastConfImport returns the last dropped .conf that merged in, for the
// import bar.
func (a *App) LastConfImport() (ConfImport, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.confImport == nil {
		return ConfImport{}, false
	}
	return *a.confImport, true
}

// DismissConfImport closes the import bar; another drop opens it again.
func (a *App) DismissConfImport() {
	a.mu.Lock()
	a.confImport = nil
	a.mu.Unlock()
	a.notify()
}

// setConfImport records a merged .conf for the bar. The path is what it
// keeps: the file may be gone by the time the user asks for an
// interpretation, and that read failure is then reported as an error.
func (a *App) setConfImport(imp ConfImport) {
	a.mu.Lock()
	a.confImport = &imp
	a.mu.Unlock()
	a.notify()
}
