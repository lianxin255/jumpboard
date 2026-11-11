// The AI chat page's orchestration: the conversation the app holds, the
// request that extends it, and the import that turns the drafts a reply
// proposed into mappings in the current scene. The conversation is the
// one place a draft is proposed now; this is its own state, written by
// its request goroutine and read by every frame.
package appt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/store"
)

// chatTimeout bounds one conversation round, the same room the one-shot
// paths give themselves.
const chatTimeout = 60 * time.Second

// AIChatState returns what the chat page draws, copied so the frame can
// walk it without holding the lock: the conversation so far, the reply
// on its way, and the drafts waiting for a decision.
func (a *App) AIChatState() (turns []ai.ChatTurn, partial string, streaming, busy bool, errText, note string, drafts []AIDraft) {
	a.ensureChatScene()
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]ai.ChatTurn(nil), a.chatTurns...),
		a.chatPartial, a.chatStreaming, a.chatBusy, a.chatErr, a.chatNote,
		append([]AIDraft(nil), a.chatDrafts...)
}

// ensureChatScene loads the selected scene's saved history and drafts
// before the page draws or a message is sent. It is intentionally outside
// the mutex while reading SQLite, and reloads only on a scene change.
func (a *App) ensureChatScene() {
	a.mu.Lock()
	scene := a.activeWS
	needs := a.chatScene != scene
	a.mu.Unlock()
	if !needs {
		return
	}
	savedTurns, _ := a.Store.ListAIChatTurns(scene, 50)
	savedDrafts, _ := a.Store.ListAIChatDrafts(scene)
	a.mu.Lock()
	defer a.mu.Unlock()
	// A switch may have won the race while SQLite was read; never replace
	// a scene that already started loading or sending.
	if a.chatScene == scene {
		return
	}
	a.chatScene = scene
	a.chatTurns = nil
	for _, turn := range savedTurns {
		a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: turn.Role, Content: turn.Content})
	}
	a.chatDrafts = nil
	for _, savedDraft := range savedDrafts {
		var proposed, original model.Mapping
		if err := json.Unmarshal(savedDraft.Proposed, &proposed); err != nil {
			continue
		}
		draft := AIDraft{Mapping: proposed, Explanation: savedDraft.Explanation, Operation: savedDraft.Operation, TargetID: savedDraft.TargetID}
		if savedDraft.Original != nil {
			if err := json.Unmarshal(*savedDraft.Original, &original); err == nil {
				draft.Original = &original
			}
		}
		a.chatDrafts = append(a.chatDrafts, draft)
	}
}
func (a *App) SendChatMessage(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		_ = a.sendChat(text, false)
	}()
}

// RetryChat asks again with the exact last question, without appending a
// second user turn. A successful retry replaces the failed answer and its
// drafts; an earlier good conversation stays intact if it fails again.
func (a *App) RetryChat() {
	a.mu.Lock()
	text := a.chatLastRequest
	a.mu.Unlock()
	if text == "" {
		return
	}
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		_ = a.sendChat(text, true)
	}()
}

// RunChatSync is sendChat on the calling thread, for the tests: the
// state lands before the call returns, so a test reads what the page
// will draw without driving the goroutine.
func (a *App) RunChatSync(text string) error {
	return a.sendChat(strings.TrimSpace(text), false)
}

// sendChat is the conversation's core: the ready check, the busy gate,
// the request with its deadline, and the state the page reads. A reply
// that fails mid-stream keeps what arrived as the assistant's turn, the
// error beside it — the same contract the interpretation's card has.
func (a *App) sendChat(text string, retry bool) error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	a.ensureChatScene()
	// The busy check and the busy flip are one locked step, so two sends
	// racing cannot start two requests.
	a.mu.Lock()
	if a.chatBusy {
		a.mu.Unlock()
		return fmt.Errorf("上一条回复还在路上")
	}
	scene := a.activeWS
	a.chatBusy, a.chatErr = true, ""
	a.chatLastRequest = text
	if !retry {
		a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: ai.RoleUser, Content: text})
		if err := a.Store.AddAIChatTurn(scene, ai.RoleUser, text, time.Now().UnixMilli()); err != nil {
			// The busy flag was already set for a request that never
			// starts: leaving it up would answer every later message with
			// "上一条回复还在路上" until the app restarts.
			a.chatBusy = false
			a.mu.Unlock()
			return err
		}
	}
	turns := append([]ai.ChatTurn(nil), a.chatTurns...)
	if limit := a.AIChatTurns(); len(turns) > limit {
		// Keep the newest whole user/assistant pair within the cap rather
		// than dropping a turn just because the requested count is odd.
		if limit%2 == 1 {
			limit++
		}
		turns = turns[len(turns)-limit:]
		for len(turns) > 0 && turns[0].Role != ai.RoleUser {
			turns = turns[1:]
		}
	}
	if len(a.mappings) > 0 {
		// The summary is a redacted projection, never the mappings
		// themselves: the full struct carries ProxyHeaders, ProxyPassURL
		// and the TLS paths, and a rule's proxy_set_header may hold a
		// bearer token. The settings page promises the summary never
		// contains a key, so only the fields a draft has to name go out.
		summary, err := json.Marshal(mappingSummaries(a.mappings))
		if err == nil {
			turns = append([]ai.ChatTurn{{Role: ai.RoleUser, Content: "当前场景的映射摘要（仅用于识别可修改规则；字段与 id 以此为准，不要猜）：" + string(summary)}}, turns...)
		}
	}
	a.mu.Unlock()
	a.notify()

	// Each fragment extends the reply the page reads, under the same
	// lock that guards the rest of the state.
	//
	// The client's own timeout is raised to the conversation's budget:
	// http.Client.Timeout covers reading the whole streamed body, so the
	// 30s one-shot default would cut a long answer short before the
	// context deadline below could matter.
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()
	reply, err := a.aiClient().WithTimeout(chatTimeout).ChatStream(ctx, turns, func(frag string) {
		a.mu.Lock()
		a.chatStreaming = true
		a.chatPartial += frag
		a.mu.Unlock()
		a.notify()
	})

	draftCount := 0
	a.mu.Lock()
	a.chatBusy, a.chatStreaming, a.chatPartial = false, false, ""
	if err != nil {
		a.chatErr = err.Error()
		// What arrived before a mid-stream failure stays as the
		// assistant's turn, shown beside the error rather than over it.
		if strings.TrimSpace(reply) != "" {
			a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: ai.RoleAssistant, Content: reply})
			_ = a.Store.AddAIChatTurn(scene, ai.RoleAssistant, reply, time.Now().UnixMilli())
		}
	} else {
		a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: ai.RoleAssistant, Content: reply})
		_ = a.Store.AddAIChatTurn(scene, ai.RoleAssistant, reply, time.Now().UnixMilli())
		a.chatDrafts, a.chatNote = a.chatDraftsOf(scene, reply)
		_ = a.saveChatDrafts(scene, a.chatDrafts)
		draftCount = len(a.chatDrafts)
	}
	_ = a.Store.TrimAIChatTurns(scene, 50)
	a.mu.Unlock()
	if err != nil {
		a.Log.Warn("AI 对话失败: %v", err)
	} else {
		a.Log.Info("AI 对话回复：%d 字，%d 条草稿", len([]rune(reply)), draftCount)
	}
	a.notify()
	return err
}

// chatDraftsOf sanitizes the drafts a reply carries: each goes through
// Draft.Mapping's validators, and one that
// cannot become a mapping is dropped with a note rather than shown as a
// card whose import could only fail. A reply that parsed to more drafts
// than the preview keeps says the cut too — the tail the cap dropped is
// as much the user's business as the draft a validator refused.
//
// The scene is a parameter, not a.chatScene: the caller already holds it,
// and chatScene is only set once the page has loaded that scene's history —
// reading it here would scope the lookup to "" on a fresh app and reject
// every update draft as "target rule gone".
func (a *App) chatDraftsOf(scene, reply string) (drafts []AIDraft, note string) {
	parsed, overCap, explicitlyEmpty := ai.ParseChatDrafts(reply)
	var dropped []string
	for _, d := range parsed {
		m, explanation, err := d.Mapping()
		if err != nil {
			dropped = append(dropped, err.Error())
			continue
		}
		if d.Operation == "update" {
			if d.TargetID == "" {
				dropped = append(dropped, "修改草稿缺少目标规则 ID")
				continue
			}
			// Scoped to the scene the conversation belongs to: a draft may
			// only update a rule of its own scene, never one whose id it
			// guessed from another.
			original, err := a.Store.GetMappingIn(scene, d.TargetID)
			if err != nil {
				dropped = append(dropped, "目标规则已不存在，不能应用旧修改草稿")
				continue
			}
			// An update starts from the full original, not from zero values:
			// omitted fields mean "unchanged", not "clear this setting".
			//
			// Name is checked with Has, not against "": Mapping() already
			// substitutes "AI 映射" for a missing name, so an empty string
			// here never means "the model said nothing" — it would rename
			// the rule to that placeholder.
			if !d.Has("name") {
				m.Name = original.Name
			}
			// ListenPort has the same shape: 0 means the model left it out,
			// and writing it back would move the rule off its port.
			if !d.Has("listenPort") {
				m.ListenPort = original.ListenPort
			}
			if !d.Has("group") {
				m.Group = original.Group
			}
			if !d.Has("serverNames") {
				m.ServerNames = original.ServerNames
			}
			if !d.Has("listenPath") {
				m.ListenPath = original.ListenPath
			}
			if !d.Has("proxyMode") {
				m.ProxyMode = original.ProxyMode
			}
			if !d.Has("extraDirectives") {
				m.ExtraDirectives = original.ExtraDirectives
			}
			m.TLSCertPath, m.TLSKeyPath = original.TLSCertPath, original.TLSKeyPath
			m.ProxyHeaders, m.ProxyPassURL = original.ProxyHeaders, original.ProxyPassURL
			m.RedirectHTTP = original.RedirectHTTP
			m.HealthCheckEnabled, m.HealthCheckIntervalSec = original.HealthCheckEnabled, original.HealthCheckIntervalSec
			m.HealthCheckTargetPath, m.HealthCheckExpectedCode = original.HealthCheckTargetPath, original.HealthCheckExpectedCode
			m.HealthStatus, m.LastCheckedAt = original.HealthStatus, original.LastCheckedAt
			if d.Enabled != nil {
				m.Enabled = *d.Enabled
			} else {
				m.Enabled = original.Enabled
			}
			m.ID = original.ID
			m.CreatedAt, m.UpdatedAt, m.SortOrder = original.CreatedAt, original.UpdatedAt, original.SortOrder
			drafts = append(drafts, AIDraft{Mapping: m, Explanation: explanation, Operation: "update", TargetID: original.ID, Original: &original})
			continue
		}
		drafts = append(drafts, AIDraft{Mapping: m, Explanation: explanation, Operation: "create"})
	}
	var notes []string
	if len(dropped) > 0 {
		// Keep the validators' concrete reasons beside usable cards too; a
		// count alone does not tell the user what to ask the model to fix.
		notes = append(notes, fmt.Sprintf("已丢弃 %d 条不完整的草稿：%s", len(dropped), strings.Join(dropped, "；")))
	}
	if explicitlyEmpty {
		notes = append(notes, "没有可用草稿，可继续描述服务、上游地址和端口")
	}
	if overCap {
		notes = append(notes, "草稿仅保留前 5 条")
	}
	note = strings.Join(notes, "；")
	return drafts, note
}

// saveChatDrafts persists only sanitized proposals. A corrupt saved draft
// is skipped on restore rather than allowed to change a rule.
func (a *App) saveChatDrafts(scene string, drafts []AIDraft) error {
	saved := make([]store.AIChatDraft, 0, len(drafts))
	for _, draft := range drafts {
		proposed, err := json.Marshal(draft.Mapping)
		if err != nil {
			continue
		}
		item := store.AIChatDraft{WorkspaceID: scene, Operation: draft.Operation, TargetID: draft.TargetID, Proposed: proposed, Explanation: draft.Explanation}
		if item.Operation == "" {
			item.Operation = "create"
		}
		if draft.Original != nil {
			original, err := json.Marshal(draft.Original)
			if err == nil {
				raw := json.RawMessage(original)
				item.Original = &raw
			}
		}
		saved = append(saved, item)
	}
	return a.Store.SaveAIChatDrafts(scene, saved)
}

// DropChatDraft removes one draft from the waiting list; the rest keep
// their places. The slice is rebuilt rather than shifted in place — the
// frame holds a snapshot that shares the backing array, and a shift
// would redraw its tail as duplicates for one frame.
func (a *App) DropChatDraft(i int) {
	a.mu.Lock()
	if i >= 0 && i < len(a.chatDrafts) {
		next := make([]AIDraft, 0, len(a.chatDrafts)-1)
		next = append(next, a.chatDrafts[:i]...)
		next = append(next, a.chatDrafts[i+1:]...)
		a.chatDrafts = next
		_ = a.saveChatDrafts(a.chatScene, a.chatDrafts)
	}
	a.mu.Unlock()
	a.notify()
}

// ClearChat empties only the current scene's conversation and pending
// drafts. Imported mappings and every other scene stay untouched.
func (a *App) ClearChat() {
	a.mu.Lock()
	scene := a.activeWS
	a.chatTurns = nil
	a.chatPartial, a.chatStreaming, a.chatBusy = "", false, false
	a.chatErr, a.chatNote, a.chatDrafts = "", "", nil
	a.chatLastRequest = ""
	a.mu.Unlock()
	_ = a.Store.ClearAIChatTurns(scene)
	_ = a.Store.ClearAIChatDrafts(scene)
	a.notify()
}

// ImportChatDrafts writes the waiting drafts into the current scene as
// new mappings or reviewed updates — never as an engine apply. Every edit
// is checked against its original version and the target scene; one
// conflict refuses the whole batch rather than leaving a half-written set.
func (a *App) ImportChatDrafts() error {
	a.mu.Lock()
	drafts := append([]AIDraft(nil), a.chatDrafts...)
	scene := a.activeWS
	a.mu.Unlock()
	if len(drafts) == 0 {
		return fmt.Errorf("没有可导入的草稿")
	}
	if scene == "" || a.chatScene != scene {
		return fmt.Errorf("场景已切换，请在新场景重新生成草稿")
	}

	var conflicts []string
	seen := make(map[int]bool, len(drafts))
	for _, d := range drafts {
		m := d.Mapping
		if m.ListenPort == 0 {
			continue // left to the protocol's default, the form's own rule
		}
		if seen[m.ListenPort] {
			conflicts = append(conflicts, fmt.Sprintf("「%s」与其它草稿都要监听 %d", m.Name, m.ListenPort))
			continue
		}
		seen[m.ListenPort] = true
		exclude := ""
		if d.Operation == "update" {
			exclude = d.TargetID
		}
		if why, _ := a.probeListenPort(m.ListenPort, m.Protocol, exclude); why != "" {
			conflicts = append(conflicts, fmt.Sprintf("「%s」（监听 %d）：%s", m.Name, m.ListenPort, why))
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("有 %d 条草稿的监听端口不可用，本次都不写入：%s", len(conflicts), strings.Join(conflicts, "；"))
	}

	for _, d := range drafts {
		if d.Operation != "update" {
			continue
		}
		if d.Original == nil || d.TargetID == "" {
			return fmt.Errorf("「%s」缺少原规则版本，不能修改", d.Mapping.Name)
		}
		// Scoped to the scene: an id from another scene is not a rule this
		// conversation may rewrite, even though the id exists.
		current, err := a.Store.GetMappingIn(scene, d.TargetID)
		if err != nil {
			return fmt.Errorf("「%s」的原规则已不存在，请重新生成草稿", d.Mapping.Name)
		}
		if current.UpdatedAt != d.Original.UpdatedAt {
			return fmt.Errorf("「%s」已在对话后被修改，请重新生成草稿，避免覆盖你的改动", d.Mapping.Name)
		}
	}

	for _, d := range drafts {
		if d.Operation == "update" {
			if err := a.Store.UpdateMappingIfUnchanged(d.Mapping, *d.Original); err != nil {
				return fmt.Errorf("「%s」写入冲突：%w", d.Mapping.Name, err)
			}
			a.Log.Info("AI 对话修改映射「%s」", d.Mapping.Name)
			continue
		}
		if _, err := a.Store.CreateMapping(scene, d.Mapping); err != nil {
			return err
		}
		a.Log.Info("AI 对话导入映射「%s」", d.Mapping.Name)
	}
	a.mu.Lock()
	a.chatDrafts, a.chatNote = nil, ""
	_ = a.saveChatDrafts(a.chatScene, nil)
	a.mu.Unlock()
	a.setStatus(fmt.Sprintf("已写入 %d 条草稿，点“应用配置”后生效", len(drafts)), false)
	a.markDirty()
	a.notify()
	return nil
}
