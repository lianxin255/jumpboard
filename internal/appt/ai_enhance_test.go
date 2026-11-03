package appt

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/store"
)

// TestChatHistorySurvivesReopen checks that conversation and pending
// drafts are local data, restored when a new App opens the same store.
func TestChatHistorySurvivesReopen(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: chatReplyWithDrafts(chatPorts[0], chatPorts[1])}
	serveChat(t, a, g)
	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(a.Paths.DataDir)
	a.Close()
	// Reopen the same database through a fresh App, not an independent
	// fixture root: both the turns and pending drafts must come back.
	t.Setenv("JUMPBOARD_DATA_ROOT", root)
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a2, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a2.Close)
	turns, _, _, _, _, _, drafts := a2.AIChatState()
	if len(turns) != 2 || len(drafts) != 2 {
		t.Fatalf("the reopened conversation = %d turns, %d drafts", len(turns), len(drafts))
	}
}

// TestChatHistoryLimitControlsWireOnly checks that the configured context
// window changes the request, not the local conversation retained.
func TestChatHistoryLimitControlsWireOnly(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: "好的。"}
	serveChat(t, a, g)
	for i := 0; i < 8; i++ {
		if err := a.RunChatSync(fmt.Sprintf("问题 %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.SetAIChatTurns("4"); err != nil {
		t.Fatal(err)
	}
	if err := a.RunChatSync("只带最近轮数"); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	messages := append([]ai.ChatTurn(nil), g.msgs...)
	g.mu.Unlock()
	if len(messages) != 4 { // system + three turns, a whole newest pair + question
		t.Fatalf("wire messages = %d, want 4: %+v", len(messages), messages)
	}
	if messages[1].Content != "问题 7" || messages[2].Content != "好的。" || messages[3].Content != "只带最近轮数" {
		t.Errorf("the window must retain the newest complete pair and question: %+v", messages)
	}
	turns, _, _, _, _, _, _ := a.AIChatState()
	if len(turns) != 18 {
		t.Errorf("local history must not be truncated: %d turns", len(turns))
	}
	if err := a.SetAIChatTurns("bad"); err == nil {
		t.Error("an invalid context limit must be refused")
	}
}

// TestChatUpdateDraftEditsExistingRule verifies an update changes only
// the target mapping and does not apply to the engine by itself.
func TestChatUpdateDraftEditsExistingRule(t *testing.T) {
	a := droppedApp(t)
	m, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "MySQL 内网", Protocol: model.TCP, Enabled: true, ListenPort: 19191,
		Group: "数据库", ListenPath: "/internal", ProxyMode: model.ProxyMinimal,
		UpstreamHost: "192.168.1.10", UpstreamPort: 3306,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	reply := fmt.Sprintf("只改端口。\n```json\n{\"drafts\":[{\"operation\":\"update\",\"targetId\":%q,\"name\":\"MySQL 内网\",\"protocol\":\"tcp\",\"listenPort\":19192,\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306,\"explanation\":\"只改端口\"}]}\n```", m.ID)
	g := &chatGateway{reply: reply}
	serveChat(t, a, g)
	if err := a.RunChatSync("把 MySQL 改成 19192，其他不动"); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, _, _, drafts := a.AIChatState()
	if len(drafts) != 1 || drafts[0].Operation != "update" || drafts[0].Original == nil {
		t.Fatalf("the update draft must carry its original: %+v", drafts)
	}
	if err := a.ImportChatDrafts(); err != nil {
		t.Fatal(err)
	}
	updated, err := a.Store.GetMapping(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ListenPort != 19192 || updated.UpstreamHost != "192.168.1.10" || updated.UpstreamPort != 3306 {
		t.Errorf("the update changed the wrong fields: %+v", updated)
	}
	if updated.Group != "数据库" || updated.ListenPath != "/internal" || updated.ProxyMode != model.ProxyMinimal {
		t.Errorf("omitted fields must remain unchanged: %+v", updated)
	}
	if listening, _ := a.RuleState(m.ID); listening {
		t.Error("a reviewed edit must not apply to the engine on its own")
	}
}

// TestChatUpdateRefusesNewerMapping checks the optimistic conflict guard:
// a user's newer edit cannot be silently overwritten by an older AI draft.
func TestChatUpdateRefusesNewerMapping(t *testing.T) {
	a := droppedApp(t)
	m, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "MySQL 内网", Protocol: model.TCP, Enabled: true, ListenPort: 19193,
		UpstreamHost: "192.168.1.10", UpstreamPort: 3306,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	reply := fmt.Sprintf("```json\n{\"drafts\":[{\"operation\":\"update\",\"targetId\":%q,\"name\":\"MySQL 内网\",\"protocol\":\"tcp\",\"listenPort\":19194,\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306}]}\n```", m.ID)
	g := &chatGateway{reply: reply}
	serveChat(t, a, g)
	if err := a.RunChatSync("改成 19194"); err != nil {
		t.Fatal(err)
	}
	m.Name = "用户后来改的名称"
	if err := a.Store.UpdateMapping(m); err != nil {
		t.Fatal(err)
	}
	if err := a.ImportChatDrafts(); err == nil || !strings.Contains(err.Error(), "对话后被修改") {
		t.Fatalf("a stale draft must be refused: %v", err)
	}
	got, _ := a.Store.GetMapping(m.ID)
	if got.Name != m.Name || got.ListenPort != m.ListenPort {
		t.Errorf("the user's newer edit was overwritten: %+v", got)
	}
}

// TestChatRetryDoesNotDuplicateQuestion checks that a retry uses the
// original question rather than appending a second user turn.
func TestChatRetryDoesNotDuplicateQuestion(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: "好的。"}
	serveChat(t, a, g)
	if err := a.RunChatSync("原始问题"); err != nil {
		t.Fatal(err)
	}
	if err := a.sendChat("原始问题", true); err != nil {
		t.Fatal(err)
	}
	turns, _, _, _, _, _, _ := a.AIChatState()
	questions := 0
	for _, turn := range turns {
		if turn.Role == ai.RoleUser && turn.Content == "原始问题" {
			questions++
		}
	}
	if questions != 1 {
		t.Errorf("the original question was duplicated %d times: %+v", questions, turns)
	}
	g.mu.Lock()
	called := g.called
	g.mu.Unlock()
	if called != 2 {
		t.Errorf("a retry should make exactly one new request, called = %d", called)
	}
}

// TestChatClearOnlyAffectsScene verifies clearing a conversation cannot
// remove imported mappings or another scene's saved conversation.
func TestChatClearOnlyAffectsScene(t *testing.T) {
	a := droppedApp(t)
	if err := a.Store.AddAIChatTurn(store.DefaultWorkspaceID, ai.RoleUser, "默认场景问题", 1); err != nil {
		t.Fatal(err)
	}
	home, err := a.Store.CreateWorkspace("家里")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store.AddAIChatTurn(home.ID, ai.RoleUser, "家里场景问题", 2); err != nil {
		t.Fatal(err)
	}
	a.ClearChat()
	if turns, _ := a.Store.ListAIChatTurns(store.DefaultWorkspaceID, 50); len(turns) != 0 {
		t.Errorf("the current scene was not cleared: %+v", turns)
	}
	if turns, _ := a.Store.ListAIChatTurns(home.ID, 50); len(turns) != 1 {
		t.Errorf("another scene must keep its history: %+v", turns)
	}
}
