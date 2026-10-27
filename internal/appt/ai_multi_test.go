package appt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// multiGateway is an in-process AI endpoint for the multi-draft tests: it
// records the request body and answers with reply, or the two-draft
// shape when reply is empty.
type multiGateway struct {
	mu    sync.Mutex
	body  []byte
	reply string
}

func (g *multiGateway) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.body = b
	reply := g.reply
	g.mu.Unlock()
	if reply == "" {
		reply = `{"drafts":[` +
			`{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,` +
			`"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"MySQL 到 9000"},` +
			`{"name":"内网 Redis","protocol":"tcp","listenPort":9001,` +
			`"upstreamHost":"192.168.1.11","upstreamPort":6379,"explanation":"Redis 到 9001"}]}`
	}
	data, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": reply}}},
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// serveMulti wires the gateway in as the app's AI endpoint.
func serveMulti(t *testing.T, a *App, g *multiGateway) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "main-model"); err != nil {
		t.Fatal(err)
	}
}

// model returns the model name the captured request carried.
func (g *multiGateway) model(t *testing.T) string {
	t.Helper()
	g.mu.Lock()
	body := g.body
	g.mu.Unlock()
	if body == nil {
		t.Fatal("the gateway saw no request")
	}
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	return req.Model
}

// TestAIMultiAsksForDrafts pins the happy path: one AIDraft per usable
// item, drafts asked of the main model (the diag model is not for them),
// and nothing stored — adopting is the user's step, not the ask's.
func TestAIMultiAsksForDrafts(t *testing.T) {
	a := droppedApp(t)
	g := &multiGateway{}
	serveMulti(t, a, g)
	if err := a.SetAIDiagModel("diag-model"); err != nil {
		t.Fatal(err)
	}

	note, drafts, err := a.AskAIMulti("把内网 MySQL 和 Redis 分别暴露到 9000/9001")
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 {
		t.Fatalf("drafts = %d, want 2: %+v", len(drafts), drafts)
	}
	if drafts[0].Mapping.Name != "内网 MySQL" || drafts[0].Mapping.ListenPort != 9000 {
		t.Errorf("first draft: %+v", drafts[0])
	}
	if drafts[1].Mapping.Name != "内网 Redis" || drafts[1].Mapping.UpstreamPort != 6379 {
		t.Errorf("second draft: %+v", drafts[1])
	}
	if note != "" {
		t.Errorf("note = %q, want none", note)
	}
	if m := g.model(t); m != "main-model" {
		t.Errorf("drafts must use the main model, got %q", m)
	}
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Errorf("asking must store nothing: %+v", list)
	}
}

// TestAIMultiDropsUnusableDrafts checks that a draft Mapping refuses is
// dropped with a note, while its usable neighbor survives.
func TestAIMultiDropsUnusableDrafts(t *testing.T) {
	a := droppedApp(t)
	g := &multiGateway{}
	g.reply = `{"drafts":[` +
		`{"name":"内网 MySQL","protocol":"tcp","listenPort":9000,` +
		`"upstreamHost":"192.168.1.10","upstreamPort":3306,"explanation":"MySQL 到 9000"},` +
		`{"name":"缺上游","protocol":"tcp","listenPort":9002,` +
		`"upstreamHost":"","upstreamPort":80,"explanation":"没有地址"}]}`
	serveMulti(t, a, g)

	note, drafts, err := a.AskAIMulti("两个服务")
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 || drafts[0].Mapping.Name != "内网 MySQL" {
		t.Fatalf("drafts: %+v", drafts)
	}
	if !strings.Contains(note, "已丢弃 1 条") {
		t.Errorf("note should say what was dropped: %q", note)
	}
}

// TestAIMultiCapsAtFive checks the cap's two halves: the list comes back
// at five, and the note says that is the most there will ever be.
func TestAIMultiCapsAtFive(t *testing.T) {
	a := droppedApp(t)
	g := &multiGateway{}
	items := make([]string, 7)
	for i := range items {
		items[i] = fmt.Sprintf(`{"name":"服务 %d","protocol":"tcp","listenPort":%d,`+
			`"upstreamHost":"10.0.0.1","upstreamPort":3306,"explanation":"x"}`, i, 9100+i)
	}
	g.reply = `{"drafts":[` + strings.Join(items, ",") + `]}`
	serveMulti(t, a, g)

	note, drafts, err := a.AskAIMulti("很多服务")
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 5 {
		t.Fatalf("drafts = %d, want the first 5", len(drafts))
	}
	if !strings.Contains(note, "最多保留 5 条") {
		t.Errorf("note should say the cap: %q", note)
	}
}

// TestAIMultiRefusesUnusableReplies checks the errors the panel shows: no
// AI configured, and a reply with nothing usable in it.
func TestAIMultiRefusesUnusableReplies(t *testing.T) {
	a := droppedApp(t)
	if _, _, err := a.AskAIMulti("x"); err == nil ||
		!strings.Contains(err.Error(), "还没有配置 AI 服务") {
		t.Fatalf("without AI: %v", err)
	}
	g := &multiGateway{}
	g.reply = `{"drafts":[]}`
	serveMulti(t, a, g)
	if _, _, err := a.AskAIMulti("x"); err == nil {
		t.Fatal("an empty reply should be an error")
	}
}

// TestAIDiagModelRoundTrip checks the setting's storage: a name set,
// read back, cleared — and trimmed, the way the other AI settings are.
func TestAIDiagModelRoundTrip(t *testing.T) {
	a := droppedApp(t)
	if a.AIDiagModel() != "" {
		t.Fatalf("out of the box: %q", a.AIDiagModel())
	}
	if err := a.SetAIDiagModel("  diag-model  "); err != nil {
		t.Fatal(err)
	}
	if a.AIDiagModel() != "diag-model" {
		t.Fatalf("read back: %q", a.AIDiagModel())
	}
	if err := a.SetAIDiagModel(""); err != nil {
		t.Fatal(err)
	}
	if a.AIDiagModel() != "" {
		t.Fatalf("cleared: %q", a.AIDiagModel())
	}
}

// TestInterpretUsesDiagModel checks the rename end to end: with the field
// empty the interpretation goes out under the main model, with it filled
// under the diagnosis model.
func TestInterpretUsesDiagModel(t *testing.T) {
	a := droppedApp(t)
	g := &multiGateway{}
	g.reply = `{"summary":"ok"}`
	serveMulti(t, a, g)

	if err := a.InterpretText("access", "access.log", "一行"); err != nil {
		t.Fatal(err)
	}
	if m := g.model(t); m != "main-model" {
		t.Fatalf("empty must mean the main model, got %q", m)
	}
	if err := a.SetAIDiagModel("diag-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.InterpretText("access", "access.log", "一行"); err != nil {
		t.Fatal(err)
	}
	if m := g.model(t); m != "diag-model" {
		t.Fatalf("the diag model must be named, got %q", m)
	}
}
