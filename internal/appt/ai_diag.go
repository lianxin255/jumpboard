package appt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/services"
)

// DiagnoseFailure asks the model why the last apply or check failed.
//
// The context it sends is the failure itself, the one mapping involved
// and the tails of two logs — never the whole table. It never runs on
// its own, never executes a command the model suggests and changes no
// configuration: the advice is text under the failure bar, for a human
// to judge.
func (a *App) DiagnoseFailure(kind string) error {
	if !a.AIReady() {
		return fmt.Errorf("还没有配置 AI 服务，请先在设置里填写")
	}
	// The failure check, the busy check and the busy flip are one locked
	// step: two clicks on the button must not start two requests, and a
	// failure that lands between them must not be missed.
	a.mu.Lock()
	errText := a.applyErr
	if errText == "" {
		a.mu.Unlock()
		return fmt.Errorf("还没有可诊断的失败")
	}
	if a.diagBusy {
		a.mu.Unlock()
		return fmt.Errorf("诊断还在进行中")
	}
	a.diagBusy, a.diagErr, a.diagAdvice = true, "", nil
	port := a.applyErrPort
	a.mu.Unlock()

	in := ai.DiagnoseInput{Action: "应用配置", Error: errText}
	if kind == "check" {
		in.Action = "预检"
	}
	if port > 0 {
		in.Rule = a.involvedRule(port)
	}
	in.OpLog = a.Log.Tail(30)
	lines, err := services.TailFile(filepath.Join(a.Paths.LogsDir, "access.log"), 50)
	switch {
	case err == nil:
		in.AccessLog = lines
	case os.IsNotExist(err):
		// No traffic yet is normal, not a degradation worth saying.
	default:
		in.Notes = append(in.Notes,
			"access.log 读取失败："+ai.Truncate(err.Error(), 200))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	adv, err := a.diagClient().Diagnose(ctx, in)

	a.mu.Lock()
	a.diagBusy = false
	if err != nil {
		a.diagErr, a.diagAdvice = err.Error(), nil
	} else {
		a.diagErr, a.diagAdvice = "", &adv
	}
	a.mu.Unlock()
	if err != nil {
		a.Log.Warn("AI 诊断失败: %v", err)
	} else {
		a.Log.Info("AI 诊断完成")
	}
	a.notify()
	return err
}

// involvedRule summarizes the enabled mappings listening on the failed
// port — at most two, joined. A bind failure is about one port, so this
// is the entire mapping context the model gets; the rest of the table
// stays home.
func (a *App) involvedRule(port int) string {
	mappings, err := a.Store.ListMappings()
	if err != nil {
		return ""
	}
	var rules []string
	for _, m := range mappings {
		if !m.Enabled || m.ListenPort != port {
			continue
		}
		rules = append(rules, ai.Truncate(mappingSummary(m), 512))
		if len(rules) == 2 {
			break
		}
	}
	return strings.Join(rules, "\n")
}

// mappingSummary is one mapping as a person reads it, only the fields
// that are set. It is all the model ever learns about the mapping table:
// one summary of one rule, never the list.
func mappingSummary(m model.Mapping) string {
	state := "停用"
	if m.Enabled {
		state = "启用"
	}
	parts := []string{
		"名称：" + m.Name,
		"状态：" + state,
		"协议：" + m.Protocol.Label(),
	}
	if m.ListenPort != 0 {
		parts = append(parts, fmt.Sprintf("监听端口：%d", m.ListenPort))
	}
	if m.ServerNames != "" {
		parts = append(parts, "域名："+m.ServerNames)
	}
	if m.ListenPath != "" {
		parts = append(parts, "路径："+m.ListenPath)
	}
	parts = append(parts, fmt.Sprintf("上游：%s:%d", m.UpstreamHost, m.UpstreamPort))
	if m.ExtraDirectives != "" {
		parts = append(parts, "附加指令："+m.ExtraDirectives)
	}
	return strings.Join(parts, "；")
}

// Diagnosis returns what the failure diagnosis is doing, for the card
// under the apply-failure bar.
func (a *App) Diagnosis() (busy bool, errText string, advice *ai.Advice) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.diagBusy, a.diagErr, a.diagAdvice
}

// DismissDiagnosis folds the advice card away, back to the button.
func (a *App) DismissDiagnosis() {
	a.mu.Lock()
	a.diagErr, a.diagAdvice = "", nil
	a.mu.Unlock()
	a.notify()
}

// FailKind says which action the stored failure came from, so the
// diagnose button asks about the operation the user actually ran.
func (a *App) FailKind() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.failKind
}
