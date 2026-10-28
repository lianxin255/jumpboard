package appt

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lianxin255/jumpboard/internal/ai"
)

// chatGateway is the chat page's stand-in model: it records the message
// list each request carried and answers with reply.
type chatGateway struct {
	mu     sync.Mutex
	msgs   []ai.ChatTurn
	reply  string
	called int
}

func (g *chatGateway) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.called++
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(b, &req); err != nil {
		g.mu.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	g.msgs = nil
	for _, m := range req.Messages {
		g.msgs = append(g.msgs, ai.ChatTurn{Role: m.Role, Content: m.Content})
	}
	reply := g.reply
	g.mu.Unlock()
	data, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": reply}}},
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// serveChat wires the gateway in as the app's AI endpoint.
func serveChat(t *testing.T, a *App, g *chatGateway) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "chat-model"); err != nil {
		t.Fatal(err)
	}
}

// chatPorts are the ports the chat tests' drafts listen on, picked off
// the usual ranges so nothing on the machine is in their way.
var chatPorts = [2]int{19101, 19102}

// chatReplyWithDrafts is the shape the chat prompt asks for: words
// around one fenced json block holding two drafts on the given ports.
func chatReplyWithDrafts(p1, p2 int) string {
	return "两条映射如下：\n\n```json\n{\"drafts\":[" +
		`{"name":"内网 MySQL","protocol":"tcp","listenPort":` + itoa(p1) + `,"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"MySQL"},` +
		`{"name":"内网 Redis","protocol":"tcp","listenPort":` + itoa(p2) + `,"upstreamHost":"192.168.1.11","upstreamPort":6379,"explanation":"Redis"}]}` +
		"\n```\n\n导入前可再调整端口。"
}

// chatAsk is the sentence a test asks with, on the drafts' own ports.
func chatAsk() string {
	return "把内网 MySQL 和 Redis 分别暴露到 " + itoa(chatPorts[0]) + "/" + itoa(chatPorts[1])
}

// TestChatRoundKeepsTurnsAndDrafts pins the round: the user's message
// and the reply both join the conversation, the drafts come out
// sanitized, and nothing is stored by talking.
func TestChatRoundKeepsTurnsAndDrafts(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: chatReplyWithDrafts(chatPorts[0], chatPorts[1])}
	serveChat(t, a, g)

	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}
	turns, partial, streaming, busy, errText, note, drafts := a.AIChatState()
	if len(turns) != 2 || turns[0].Role != ai.RoleUser || turns[1].Role != ai.RoleAssistant {
		t.Fatalf("turns = %+v", turns)
	}
	if turns[0].Content != chatAsk() {
		t.Errorf("the user's message must survive: %q", turns[0].Content)
	}
	if busy || streaming || partial != "" || errText != "" {
		t.Errorf("the round must end settled: busy=%v streaming=%v partial=%q err=%q", busy, streaming, partial, errText)
	}
	if note != "" {
		t.Errorf("note = %q, want none", note)
	}
	if len(drafts) != 2 || drafts[0].Mapping.Name != "内网 MySQL" || drafts[0].Mapping.ListenPort != chatPorts[0] {
		t.Fatalf("drafts: %+v", drafts)
	}
	if !drafts[0].Mapping.Enabled {
		t.Errorf("a draft imports as enabled, the form's own default: %+v", drafts[0].Mapping)
	}
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Errorf("talking must store nothing: %+v", list)
	}
}

// TestChatSendsHistoryAcrossRounds checks the multi-turn half: the
// second round's request carries the first round's turns too, so the
// model reads the conversation rather than the last line alone. Each
// round costs two requests — the stream probe, which this gateway
// answers as plain JSON, and the one-shot fallback.
func TestChatSendsHistoryAcrossRounds(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: "好的。"}
	serveChat(t, a, g)

	if err := a.RunChatSync("第一条"); err != nil {
		t.Fatal(err)
	}
	if err := a.RunChatSync("第二条"); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	msgs, called := g.msgs, g.called
	g.mu.Unlock()
	if called != 4 {
		t.Fatalf("called = %d, want 4 (a probe and a fallback per round)", called)
	}
	if len(msgs) != 4 {
		t.Fatalf("messages = %d, want system + 3 turns: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "system" {
		t.Errorf("the system prompt must lead: %+v", msgs[0])
	}
	if msgs[1].Content != "第一条" || msgs[2].Content != "好的。" || msgs[3].Content != "第二条" {
		t.Errorf("the history must travel whole: %+v", msgs)
	}
}

// TestChatRefusesWithoutAI checks the gate: no endpoint, no request.
func TestChatRefusesWithoutAI(t *testing.T) {
	a := droppedApp(t)
	if err := a.RunChatSync("你好"); err == nil || !strings.Contains(err.Error(), "还没有配置 AI 服务") {
		t.Fatalf("without AI: %v", err)
	}
	if turns, _, _, _, _, _, drafts := a.AIChatState(); len(turns) != 0 || len(drafts) != 0 {
		t.Fatalf("a refused ask must leave nothing: %+v", turns)
	}
}

// TestChatKeepsTheErrorInTheState checks the failure contract: a dead
// endpoint leaves the user's message in place, the error in the state
// the page reads, and no drafts.
func TestChatKeepsTheErrorInTheState(t *testing.T) {
	a := droppedApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "chat-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.RunChatSync("你好"); err == nil {
		t.Fatal("a dead endpoint must be an error")
	}
	turns, _, _, busy, errText, _, drafts := a.AIChatState()
	if len(turns) != 1 || turns[0].Role != ai.RoleUser {
		t.Fatalf("the user's message stays: %+v", turns)
	}
	if busy {
		t.Error("the round must end not busy")
	}
	if errText == "" {
		t.Error("the error must land in the state the page reads")
	}
	if len(drafts) != 0 {
		t.Errorf("no drafts out of a failed round: %+v", drafts)
	}
}

// TestImportChatDraftsStoresIntoCurrentScene pins the import: every
// waiting draft becomes a mapping in the active scene, dirty is marked,
// the engine is not applied, and the waiting list empties.
func TestImportChatDraftsStoresIntoCurrentScene(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: chatReplyWithDrafts(chatPorts[0], chatPorts[1])}
	serveChat(t, a, g)
	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}

	ws := a.ActiveWorkspace()
	if err := a.ImportChatDrafts(); err != nil {
		t.Fatal(err)
	}
	list, err := a.Store.ListMappings(ws)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Name != "内网 MySQL" || list[0].ListenPort != chatPorts[0] || list[0].UpstreamHost != "192.168.1.10" {
		t.Errorf("stored wrongly: %+v", list[0])
	}
	if !a.Dirty() {
		t.Error("an import marks the configuration dirty, like a save")
	}
	// A rule that was imported but not applied must not be listening:
	// the import never touches the engine.
	if listening, _ := a.RuleState(list[0].ID); listening {
		t.Error("an imported draft must not start forwarding on its own")
	}
	if _, _, _, _, _, _, drafts := a.AIChatState(); len(drafts) != 0 {
		t.Errorf("the waiting list must empty: %+v", drafts)
	}
	if err := a.ImportChatDrafts(); err == nil || !strings.Contains(err.Error(), "没有可导入的草稿") {
		t.Fatalf("a second import must refuse: %v", err)
	}
}

// TestImportChatDraftsRefusesPortConflicts checks the port gate: a draft
// whose listen port something on the machine already holds refuses the
// whole batch — nothing lands, and the reason names the draft. The form
// gate is the same one: a port another mapping merely has saved, with
// nothing listening on it, is not a conflict until it is applied.
func TestImportChatDraftsRefusesPortConflicts(t *testing.T) {
	a := droppedApp(t)
	held := heldPort(t)
	g := &chatGateway{reply: chatReplyWithDrafts(held, chatPorts[1])}
	serveChat(t, a, g)
	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}

	err := a.ImportChatDrafts()
	if err == nil || !strings.Contains(err.Error(), "监听端口不可用") || !strings.Contains(err.Error(), "内网 MySQL") {
		t.Fatalf("the conflict must refuse with the draft named: %v", err)
	}
	list, _ := a.Store.ListMappings(a.ActiveWorkspace())
	if len(list) != 0 {
		t.Fatalf("a refused import must store nothing: %+v", list)
	}
	if _, _, _, _, _, _, drafts := a.AIChatState(); len(drafts) != 2 {
		t.Errorf("a refused import keeps the drafts waiting: %+v", drafts)
	}
}

// TestImportChatDraftsRefusesBatchDuplicates checks the one conflict the
// form cannot have and an import can: two drafts of one reply naming the
// same listen port.
func TestImportChatDraftsRefusesBatchDuplicates(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: chatReplyWithDrafts(chatPorts[0], chatPorts[0])}
	serveChat(t, a, g)
	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}

	err := a.ImportChatDrafts()
	if err == nil || !strings.Contains(err.Error(), "都要监听") {
		t.Fatalf("a batch's own duplicate must refuse: %v", err)
	}
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Fatalf("nothing lands: %+v", list)
	}
}

// TestDropChatDraftAndClear check the waiting list's own two moves.
func TestDropChatDraftAndClear(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{reply: chatReplyWithDrafts(chatPorts[0], chatPorts[1])}
	serveChat(t, a, g)
	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}
	a.DropChatDraft(0)
	_, _, _, _, _, _, drafts := a.AIChatState()
	if len(drafts) != 1 || drafts[0].Mapping.Name != "内网 Redis" {
		t.Fatalf("after the drop: %+v", drafts)
	}
	a.DropChatDraft(99) // out of range is a no-op, not a panic
	a.ClearChat()
	turns, _, _, _, _, _, drafts := a.AIChatState()
	if len(turns) != 0 || len(drafts) != 0 {
		t.Fatalf("after the clear: turns=%d drafts=%d", len(turns), len(drafts))
	}
}

// TestChatDraftsDropUnusableItems checks the sanitizer's half: a draft
// the mapping validators refuse is dropped with a note, its usable
// neighbor survives.
func TestChatDraftsDropUnusableItems(t *testing.T) {
	a := droppedApp(t)
	g := &chatGateway{}
	g.reply = "```json\n{\"drafts\":[" +
		`{"name":"内网 MySQL","protocol":"tcp","listenPort":` + itoa(chatPorts[0]) + `,"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"x"},` +
		`{"name":"缺上游","protocol":"tcp","listenPort":` + itoa(chatPorts[1]) + `,"upstreamHost":"","upstreamPort":80,"explanation":"没有地址"}]}` +
		"\n```"
	serveChat(t, a, g)
	if err := a.RunChatSync(chatAsk()); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, _, note, drafts := a.AIChatState()
	if len(drafts) != 1 || drafts[0].Mapping.Name != "内网 MySQL" {
		t.Fatalf("drafts: %+v", drafts)
	}
	if !strings.Contains(note, "已丢弃 1 条") {
		t.Errorf("note should say what was dropped: %q", note)
	}
}
