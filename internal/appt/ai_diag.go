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
	mappings, err := a.Store.ListMappings(a.ActiveWorkspace())
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

// chatMappingSummary is one rule as the conversation's summary carries it.
//
// It is a deliberate allow-list, not the mapping minus a few fields: the
// conversation needs the id it may copy back and the fields a draft can
// name, and nothing else. ProxyHeaders, ProxyPassURL, TLSCertPath,
// TLSKeyPath and ExtraDirectives stay here — a proxy_set_header may hold
// a bearer token, and the settings page promises the summary never
// contains a key. Adding a field to model.Mapping must not add it to what
// leaves this machine; it has to be added below on purpose.
type chatMappingSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Group        string `json:"group,omitempty"`
	Enabled      bool   `json:"enabled"`
	Protocol     string `json:"protocol"`
	ListenPort   int    `json:"listenPort"`
	ServerNames  string `json:"serverNames,omitempty"`
	ListenPath   string `json:"listenPath,omitempty"`
	UpstreamHost string `json:"upstreamHost"`
	UpstreamPort int    `json:"upstreamPort"`
	ProxyMode    string `json:"proxyMode,omitempty"`
}

// mappingSummaries projects the scene's rules onto what the model may
// see. The order is the table's, so a reply's ids line up with the rows.
func mappingSummaries(ms []model.Mapping) []chatMappingSummary {
	out := make([]chatMappingSummary, 0, len(ms))
	for _, m := range ms {
		out = append(out, chatMappingSummary{
			ID:           m.ID,
			Name:         m.Name,
			Group:        m.Group,
			Enabled:      m.Enabled,
			Protocol:     string(m.Protocol),
			ListenPort:   m.ListenPort,
			ServerNames:  m.ServerNames,
			ListenPath:   m.ListenPath,
			UpstreamHost: m.UpstreamHost,
			UpstreamPort: m.UpstreamPort,
			ProxyMode:    string(m.ProxyMode),
		})
	}
	return out
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
	// ExtraDirectives are deliberately absent: they are the one field a
	// user writes raw nginx in, so a proxy_set_header there may carry a
	// credential. The diagnosis needs the rule's shape, not its secrets.
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
