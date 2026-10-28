// The AI chat page's orchestration: the conversation the app holds, the
// request that extends it, and the import that turns the drafts a reply
// proposed into mappings in the current scene. The one-shot panel in
// ai.go stays as it is; this is the conversation's own state, written by
// its request goroutine and read by every frame.
package appt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/ai"
)

// chatTimeout bounds one conversation round, the same room the one-shot
// paths give themselves.
const chatTimeout = 60 * time.Second

// AIChatState returns what the chat page draws, copied so the frame can
// walk it without holding the lock: the conversation so far, the reply
// on its way, and the drafts waiting for a decision.
func (a *App) AIChatState() (turns []ai.ChatTurn, partial string, streaming, busy bool, errText, note string, drafts []AIDraft) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]ai.ChatTurn(nil), a.chatTurns...),
		a.chatPartial, a.chatStreaming, a.chatBusy, a.chatErr, a.chatNote,
		append([]AIDraft(nil), a.chatDrafts...)
}

// SendChatMessage appends the user's message and asks for the reply off
// the main thread. The conversation has its own busy flag rather than
// the app-wide one: a reply on its way must not lock 应用配置, which
// touches a different world than the chat does.
func (a *App) SendChatMessage(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		_ = a.sendChat(text)
	}()
}

// RunChatSync is sendChat on the calling thread, for the tests: the
// state lands before the call returns, the way RunAISync stands in for
// the panel's ask.
func (a *App) RunChatSync(text string) error {
	return a.sendChat(strings.TrimSpace(text))
}

// sendChat is the conversation's core: the ready check, the busy gate,
// the request with its deadline, and the state the page reads. A reply
// that fails mid-stream keeps what arrived as the assistant's turn, the
// error beside it — the same contract the interpretation's card has.
func (a *App) sendChat(text string) error {
	if err := a.errAINotReady(); err != nil {
		return err
	}
	// The busy check and the busy flip are one locked step, so two sends
	// racing cannot start two requests.
	a.mu.Lock()
	if a.chatBusy {
		a.mu.Unlock()
		return fmt.Errorf("上一条回复还在路上")
	}
	a.chatBusy, a.chatErr = true, ""
	a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: ai.RoleUser, Content: text})
	turns := append([]ai.ChatTurn(nil), a.chatTurns...)
	a.mu.Unlock()
	a.notify()

	// Each fragment extends the reply the page reads, under the same
	// lock that guards the rest of the state.
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()
	reply, err := a.aiClient().ChatStream(ctx, turns, func(frag string) {
		a.mu.Lock()
		a.chatStreaming = true
		a.chatPartial += frag
		a.mu.Unlock()
		a.notify()
	})

	a.mu.Lock()
	a.chatBusy, a.chatStreaming, a.chatPartial = false, false, ""
	if err != nil {
		a.chatErr = err.Error()
		// What arrived before a mid-stream failure stays as the
		// assistant's turn, shown beside the error rather than over it.
		if strings.TrimSpace(reply) != "" {
			a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: ai.RoleAssistant, Content: reply})
		}
	} else {
		a.chatTurns = append(a.chatTurns, ai.ChatTurn{Role: ai.RoleAssistant, Content: reply})
		a.chatDrafts, a.chatNote = chatDraftsOf(reply)
	}
	a.mu.Unlock()
	if err != nil {
		a.Log.Warn("AI 对话失败: %v", err)
	} else {
		a.Log.Info("AI 对话回复：%d 字，%d 条草稿", len([]rune(reply)), len(a.chatDrafts))
	}
	a.notify()
	return err
}

// chatDraftsOf sanitizes the drafts a reply carries, the way AskAIMulti
// does: each goes through Draft.Mapping's validators, and one that
// cannot become a mapping is dropped with a note rather than shown as a
// card whose import could only fail.
func chatDraftsOf(reply string) (drafts []AIDraft, note string) {
	var dropped []string
	for _, d := range ai.DraftsFrom(reply) {
		m, explanation, err := d.Mapping()
		if err != nil {
			dropped = append(dropped, err.Error())
			continue
		}
		drafts = append(drafts, AIDraft{Mapping: m, Explanation: explanation})
	}
	if len(drafts) == 0 && len(dropped) > 0 {
		note = fmt.Sprintf("已丢弃 %d 条不完整的草稿：%s", len(dropped), strings.Join(dropped, "；"))
	} else if len(dropped) > 0 {
		note = fmt.Sprintf("已丢弃 %d 条不完整的草稿", len(dropped))
	}
	return drafts, note
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
	}
	a.mu.Unlock()
	a.notify()
}

// ClearChat empties the conversation and the drafts waiting in it. The
// store is not touched: a conversation is talk, and what it produced
// either was imported or was not.
func (a *App) ClearChat() {
	a.mu.Lock()
	a.chatTurns = nil
	a.chatPartial, a.chatStreaming, a.chatBusy = "", false, false
	a.chatErr, a.chatNote, a.chatDrafts = "", "", nil
	a.mu.Unlock()
	a.notify()
}

// ImportChatDrafts writes the waiting drafts into the current scene as
// new mappings — the same thing 保存 does for a hand-typed rule, and no
// more: the engine is not applied, and nothing starts listening until
// the user says 应用配置.
//
// The port check runs over every draft before anything is stored, and
// one conflict refuses the whole batch: a half-imported set would leave
// the user to work out which of the cards they clicked actually landed.
// The reason names each refused draft, so the fix — a port to free or a
// card to drop — is on the screen.
func (a *App) ImportChatDrafts() error {
	a.mu.Lock()
	drafts := append([]AIDraft(nil), a.chatDrafts...)
	a.mu.Unlock()
	if len(drafts) == 0 {
		return fmt.Errorf("没有可导入的草稿")
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
		if why, _ := a.probeListenPort(m.ListenPort, m.Protocol, ""); why != "" {
			conflicts = append(conflicts, fmt.Sprintf("「%s」（监听 %d）：%s", m.Name, m.ListenPort, why))
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("有 %d 条草稿的监听端口不可用，本次都不导入：%s", len(conflicts), strings.Join(conflicts, "；"))
	}

	ws := a.ActiveWorkspace()
	for _, d := range drafts {
		if _, err := a.Store.CreateMapping(ws, d.Mapping); err != nil {
			return err
		}
		a.Log.Info("AI 对话导入映射「%s」", d.Mapping.Name)
	}
	a.mu.Lock()
	a.chatDrafts, a.chatNote = nil, ""
	a.mu.Unlock()
	a.setStatus(fmt.Sprintf("已导入 %d 条映射，点“应用配置”后生效", len(drafts)), false)
	a.markDirty()
	a.notify()
	return nil
}
