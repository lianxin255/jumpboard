package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/ai"
	"github.com/lianxin255/jumpboard/internal/model"
)

// TestAIChatUpdateDiffAndConflict verifies the actual UI contract of an
// edit: old/new values are shown, and a stale target disables its import.
func TestAIChatUpdateDiffAndConflict(t *testing.T) {
	a := testApp(t)
	m, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "MySQL 内网", Protocol: model.TCP, Enabled: true, ListenPort: 19195,
		UpstreamHost: "192.168.1.10", UpstreamPort: 3306,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	reply := fmt.Sprintf("只改端口。\n```json\n{\"drafts\":[{\"operation\":\"update\",\"targetId\":%q,\"name\":\"MySQL 内网\",\"protocol\":\"tcp\",\"listenPort\":19196,\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306}]}\n```", m.ID)
	srv := fakeOpenAI(t, reply)
	a.SetAI(srv.URL, "chat-model")
	tt := tester(t, a, 1280, 800)
	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	fillField(t, tt, "消息", "把 MySQL 的端口改成 19196")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the update card", func() bool {
		return tt.HasText("修改") && tt.HasText("127.0.0.1:19195") && tt.HasText("127.0.0.1:19196")
	})
	// A user edit after the proposal marks the target as conflicted in the
	// next frame, and the whole batch cannot be imported silently.
	m.Name = "用户后来改的名称"
	if err := a.Store.UpdateMapping(m); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("原规则已在对话后被修改") {
		t.Errorf("the stale target should be explained: %v", tt.Texts())
	}
	if a.ConfirmState() != nil {
		t.Error("a stale edit must not open the import confirmation")
	}
}

// TestAIChatCopyAndClear verifies readable clipboard content and the
// panel-local clear confirmation, not a destructive window-bar action.
func TestAIChatCopyAndClear(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, chatReply())
	a.SetAI(srv.URL, "chat-model")
	var copied string
	a.Clipboard = func(text string) { copied = text }
	tt := tester(t, a, 1280, 800)
	if err := tt.Click("AI 对话"); err != nil {
		t.Fatal(err)
	}
	fillField(t, tt, "消息", "把内网 MySQL 和 Redis 分别暴露到 19105/19106")
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the reply", func() bool { return tt.HasText("两条映射如下") })
	if err := tt.Click("复制回复"); err != nil {
		t.Fatal(err)
	}
	if copied == "" || strings.Contains(copied, "```json") || !strings.Contains(copied, "两条映射如下") {
		t.Errorf("copy must be readable text without draft JSON: %q", copied)
	}
	if err := tt.Click("清空对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("清空当前场景对话") || !tt.HasText("其他场景和配置历史不受影响") {
		t.Errorf("clearing should explain its scope: %v", tt.Texts())
	}
	confirm(t, tt, "清空对话")
	turns, _, _, _, _, _, drafts := a.AIChatState()
	if len(turns) != 0 || len(drafts) != 0 {
		t.Errorf("the current scene should be empty: %d turns, %d drafts", len(turns), len(drafts))
	}
}

// TestAIHistoryTurnsSetting checks the context control in the AI settings,
// rather than only its application getter.
func TestAIHistoryTurnsSetting(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1280, 800)
	openSettings(t, tt, "")
	if !tt.HasText("发送给模型的对话轮数") || !tt.HasText("对话历史轮数") {
		t.Fatalf("the history control is missing: %v", tt.Texts())
	}
	if a.AIChatTurns() != 12 {
		t.Errorf("default context = %d, want 12", a.AIChatTurns())
	}
	if err := a.SetAIChatTurns("20"); err != nil {
		t.Fatal(err)
	}
	if a.AIChatTurns() != 20 {
		t.Errorf("stored context = %d, want 20", a.AIChatTurns())
	}
	_ = ai.RoleUser
}
