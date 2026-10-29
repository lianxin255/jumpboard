package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// chatGateway stands in for the model the way the other tests' fakes do,
// and records the request body so a test can read the message list the
// conversation sent.
type chatGateway struct {
	mu    sync.Mutex
	body  []byte
	reply string
	calls int
	// sse says the reply travels as server-sent events.
	sse bool
}

func (g *chatGateway) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.body = b
	g.calls++
	reply, sse := g.reply, g.sse
	g.mu.Unlock()
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		for _, frag := range strings.SplitAfter(reply, " ") {
			data, _ := json.Marshal(map[string]any{
				"choices": []any{map[string]any{"delta": map[string]any{"content": frag}}},
			})
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			fl.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		return
	}
	data, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": reply}}},
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// serveChat wires the gateway in as a client's endpoint.
func serveChat(t *testing.T, g *chatGateway) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	return New(Settings{BaseURL: srv.URL, Model: "chat-model", Timeout: 5 * time.Second})
}

// request reads the message list the gateway saw.
func (g *chatGateway) request(t *testing.T) []openai.ChatCompletionMessage {
	t.Helper()
	g.mu.Lock()
	body := g.body
	g.mu.Unlock()
	if body == nil {
		t.Fatal("the gateway saw no request")
	}
	var req struct {
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	return req.Messages
}

// TestChatSendsTheConversation pins the wire shape: the system prompt
// first, then every turn in order, and the reply comes back as it was
// written.
func TestChatSendsTheConversation(t *testing.T) {
	g := &chatGateway{reply: "好的，9000 没问题。"}
	c := serveChat(t, g)
	turns := []ChatTurn{
		{Role: RoleUser, Content: "把内网 MySQL 暴露到 9000"},
		{Role: RoleAssistant, Content: "好的，9000 没问题。"},
		{Role: RoleUser, Content: "再加一条 Redis 到 9001"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, err := c.Chat(ctx, turns)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "好的，9000 没问题。" {
		t.Fatalf("reply = %q", reply)
	}
	msgs := g.request(t)
	if len(msgs) != 4 {
		t.Fatalf("messages = %d, want system + 3 turns", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != chatSystemPrompt {
		t.Errorf("the first message must be the chat system prompt")
	}
	wantRoles := []string{"user", "assistant", "user"}
	for i, role := range wantRoles {
		if msgs[i+1].Role != role {
			t.Errorf("message %d role = %q, want %q", i+1, msgs[i+1].Role, role)
		}
	}
	if msgs[3].Content != "再加一条 Redis 到 9001" {
		t.Errorf("the newest turn must travel whole: %q", msgs[3].Content)
	}
}

// TestChatStreamUsesPlainJSONResponse checks the conversation through a
// gateway that ignores stream:true: the first plain-JSON reply arrives whole,
// costs one request, and keeps its raw prose rather than being sliced by
// the one-shot paths' extractJSON.
func TestChatStreamUsesPlainJSONResponse(t *testing.T) {
	g := &chatGateway{}
	g.reply = "两条映射如下：\n\n```json\n{\"drafts\":[{\"name\":\"内网 MySQL\",\"protocol\":\"tcp\",\"listenPort\":9000,\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306,\"explanation\":\"x\"}]}\n```\n\n导入前可改端口。"
	c := serveChat(t, g)
	var got []string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, err := c.ChatStream(ctx, []ChatTurn{{Role: RoleUser, Content: "两条"}}, func(frag string) {
		got = append(got, frag)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a one-shot fallback must deliver no fragments: %v", got)
	}
	g.mu.Lock()
	calls := g.calls
	g.mu.Unlock()
	if calls != 1 {
		t.Errorf("plain JSON chat used %d requests, want 1", calls)
	}
	if !strings.HasPrefix(reply, "两条映射如下") || !strings.HasSuffix(reply, "导入前可改端口。") {
		t.Fatalf("the prose reply must travel whole: %q", reply)
	}
	if drafts := DraftsFrom(reply); len(drafts) != 1 || drafts[0].Name != "内网 MySQL" {
		t.Errorf("drafts from the fallback reply: %+v", drafts)
	}
}

// TestChatStreamDeliversDeltas checks the streaming half: fragments
// arrive in order and the whole text comes back.
func TestChatStreamDeliversDeltas(t *testing.T) {
	g := &chatGateway{sse: true}
	g.reply = "好的 两条 映射"
	c := serveChat(t, g)
	var got []string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, err := c.ChatStream(ctx, []ChatTurn{{Role: RoleUser, Content: "两条"}}, func(frag string) {
		got = append(got, frag)
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "好的 两条 映射" {
		t.Fatalf("reply = %q", reply)
	}
	if len(got) == 0 {
		t.Fatal("the deltas never arrived")
	}
	if strings.Join(got, "") != reply {
		t.Fatalf("deltas must join into the reply: %q vs %q", strings.Join(got, ""), reply)
	}
}

// TestWireTurnsTrimsTheWindow checks the history the wire carries: the
// newest chatTurnsMax turns survive, the window starts on a user turn,
// and a turn with a role the wire has no meaning for never travels.
func TestWireTurnsTrimsTheWindow(t *testing.T) {
	var turns []ChatTurn
	for i := 0; i < chatTurnsMax+3; i++ {
		turns = append(turns, ChatTurn{Role: RoleUser, Content: "问 " + itochat(i)})
		turns = append(turns, ChatTurn{Role: RoleAssistant, Content: "答 " + itochat(i)})
	}
	turns = append(turns, ChatTurn{Role: "system", Content: "system 不该混进来"})
	msgs := wireTurns(turns)
	if len(msgs) != chatTurnsMax+1 {
		t.Fatalf("messages = %d, want system + %d", len(msgs)-1, chatTurnsMax)
	}
	if msgs[0].Role != "system" {
		t.Fatalf("the system prompt must lead")
	}
	if msgs[1].Role != RoleUser {
		t.Fatalf("the window must open on a user turn, got %q", msgs[1].Role)
	}
	for _, m := range msgs {
		if m.Content == "system 不该混进来" {
			t.Error("a turn with a foreign role must not travel")
		}
	}
	last := turns[len(turns)-2] // the newest user turn before the intruder
	if msgs[len(msgs)-1].Content != last.Content {
		t.Errorf("the newest turn must survive: %q vs %q", msgs[len(msgs)-1].Content, last.Content)
	}
}

// itochat renders i for the window test's labels.
func itochat(i int) string {
	return strings.Repeat("·", 0) + string(rune('0'+i%10))
}

// TestDraftsFromFencedBlock reads drafts out of the shape the prompt asks
// for: prose around a fenced json block.
func TestDraftsFromFencedBlock(t *testing.T) {
	reply := "两条映射如下：\n\n```json\n{\"drafts\":[" +
		"{\"name\":\"内网 MySQL\",\"protocol\":\"tcp\",\"listenPort\":9000,\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306,\"explanation\":\"MySQL 到 9000\"}," +
		"{\"name\":\"内网 Redis\",\"protocol\":\"tcp\",\"listenPort\":9001,\"upstreamHost\":\"192.168.1.11\",\"upstreamPort\":6379,\"explanation\":\"Redis 到 9001\"}]}\n```\n\n导入前可再调整端口。"
	drafts := DraftsFrom(reply)
	if len(drafts) != 2 {
		t.Fatalf("drafts = %d, want 2", len(drafts))
	}
	if drafts[0].Name != "内网 MySQL" || drafts[0].ListenPort != 9000 {
		t.Errorf("first: %+v", drafts[0])
	}
	if drafts[1].UpstreamPort != 6379 {
		t.Errorf("second: %+v", drafts[1])
	}
}

// TestDraftsFromUnfencedReply checks the fallback: a model that skipped
// the fence still has its drafts read, through the same extractJSON the
// one-shot paths use.
func TestDraftsFromUnfencedReply(t *testing.T) {
	reply := "好的，这条就够：{\"drafts\":[{\"name\":\"SSH\",\"protocol\":\"tcp\",\"listenPort\":9002,\"upstreamHost\":\"10.0.0.5\",\"upstreamPort\":22,\"explanation\":\"SSH\"}]}"
	drafts := DraftsFrom(reply)
	if len(drafts) != 1 || drafts[0].Name != "SSH" {
		t.Fatalf("drafts: %+v", drafts)
	}
}

// TestDraftsFromProseIsNil checks that a conversational answer with no
// drafts reads as one — nil, not an error-shaped empty list.
func TestDraftsFromProseIsNil(t *testing.T) {
	for _, reply := range []string{
		"9000 端口是这台机器监听的端口，3306 是内网 MySQL 的端口。",
		"",
		"```bash\nlsof -i :9000\n```",
	} {
		if drafts := DraftsFrom(reply); drafts != nil {
			t.Errorf("DraftsFrom(%q) = %+v, want nil", reply, drafts)
		}
	}
}

// TestDraftsFromCapsAtMax checks the cap holds across blocks too.
func TestDraftsFromCapsAtMax(t *testing.T) {
	var blocks []string
	for i := 0; i < maxDrafts+2; i++ {
		blocks = append(blocks, "```json\n{\"drafts\":[{\"name\":\"服务"+string(rune('0'+i))+
			"\",\"protocol\":\"tcp\",\"listenPort\":910"+string(rune('0'+i))+
			",\"upstreamHost\":\"10.0.0.1\",\"upstreamPort\":3306,\"explanation\":\"x\"}]}\n```")
	}
	if drafts := DraftsFrom(strings.Join(blocks, "\n\n")); len(drafts) != maxDrafts {
		t.Fatalf("drafts = %d, want the cap %d", len(drafts), maxDrafts)
	}
}

// capDraftItems builds n usable draft objects for the cap's signal test.
func capDraftItems(n int) []string {
	items := make([]string, n)
	for i := 0; i < n; i++ {
		items[i] = `{"name":"服务` + itochat(i) + `","protocol":"tcp","listenPort":910` + itochat(i) +
			`,"upstreamHost":"10.0.0.1","upstreamPort":3306,"explanation":"x"}`
	}
	return items
}

// TestDraftsOverCap pins the signal behind the cap's note: a reply that
// parses to more than maxDrafts drafts says so — in one block, across
// blocks, and in the unfenced fallback — and nothing else does.
func TestDraftsOverCap(t *testing.T) {
	fenced := func(n int) string {
		return "```json\n{\"drafts\":[" + strings.Join(capDraftItems(n), ",") + "]}\n```"
	}
	bare := func(n int) string {
		return "{\"drafts\":[" + strings.Join(capDraftItems(n), ",") + "]}"
	}
	if got := len(DraftsFrom(fenced(maxDrafts + 1))); got != maxDrafts {
		t.Fatalf("DraftsFrom keeps the cap, got %d drafts", got)
	}
	for _, tc := range []struct {
		name  string
		reply string
		want  bool
	}{
		{"one over the cap", fenced(maxDrafts + 1), true},
		{"exactly the cap", fenced(maxDrafts), false},
		{"under the cap", fenced(2), false},
		{"blocks summing over", fenced(3) + "\n\n" + fenced(3), true},
		{"blocks summing under", fenced(2) + "\n\n" + fenced(2), false},
		{"unfenced over the cap", "好的，这样配：" + bare(maxDrafts+1), true},
		{"unfenced under", "好的，这样配：" + bare(2), false},
		{"prose", "9000 是本机监听端口。", false},
		{"a fence that is not drafts", "```bash\nlsof -i :9000\n```", false},
		{"empty", "", false},
	} {
		if got := DraftsOverCap(tc.reply); got != tc.want {
			t.Errorf("%s: DraftsOverCap = %v, want %v", tc.name, got, tc.want)
		}
		kept, overCap, empty := ParseChatDrafts(tc.reply)
		if overCap != tc.want || len(kept) != len(DraftsFrom(tc.reply)) || empty {
			t.Errorf("%s: shared parse = %d drafts, over=%v empty=%v", tc.name, len(kept), overCap, empty)
		}
	}
}

// TestParseChatDraftsExplicitEmpty distinguishes an explicit empty array
// from prose and strips both its fenced and bare machine-only displays.
func TestParseChatDraftsExplicitEmpty(t *testing.T) {
	for _, reply := range []string{
		`{"drafts":[]}`,
		"```json\n{\"drafts\":[]}\n```",
	} {
		drafts, overCap, empty := ParseChatDrafts(reply)
		if len(drafts) != 0 || overCap || !empty {
			t.Errorf("ParseChatDrafts(%q) = %v, over=%v, empty=%v", reply, drafts, overCap, empty)
		}
		if got := WithoutDraftBlocks(reply); got != "" {
			t.Errorf("machine-only reply should leave no raw JSON: %q", got)
		}
	}
}

// TestWithoutDraftBlocks checks the display side: the fence that carried
// drafts goes, the words around it stay, and a fence that is not drafts
// stays fence and all.
func TestWithoutDraftBlocks(t *testing.T) {
	reply := "两条映射如下：\n\n```json\n{\"drafts\":[{\"name\":\"内网 MySQL\",\"protocol\":\"tcp\",\"listenPort\":9000,\"upstreamHost\":\"192.168.1.10\",\"upstreamPort\":3306,\"explanation\":\"x\"}]}\n```\n\n查占用可以跑：\n\n```bash\nlsof -i :9000\n```\n\n导入前可再调整端口。"
	got := WithoutDraftBlocks(reply)
	if strings.Contains(got, "drafts") || strings.Contains(got, "192.168.1.10") {
		t.Errorf("the drafts block must go: %q", got)
	}
	if !strings.Contains(got, "两条映射如下") || !strings.Contains(got, "导入前可再调整端口") {
		t.Errorf("the prose must stay: %q", got)
	}
	if !strings.Contains(got, "lsof -i :9000") || !strings.Contains(got, "```bash") {
		t.Errorf("a fence that is not drafts must stay fence and all: %q", got)
	}
	if !strings.HasPrefix(got, "两条映射如下") || !strings.HasSuffix(got, "导入前可再调整端口。") {
		t.Errorf("the blank runs a dropped block leaves must collapse: %q", got)
	}
}

// TestWithoutDraftBlocksKeepsEverythingWhenNothingIsDrafts checks a plain
// answer round-trips untouched.
func TestWithoutDraftBlocksKeepsEverythingWhenNothingIsDrafts(t *testing.T) {
	reply := "9000 是本机监听端口；想换端口直接告诉我。"
	if got := WithoutDraftBlocks(reply); got != reply {
		t.Errorf("got %q", got)
	}
}
