package appt

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// multiGateway is an in-process AI endpoint for the model-choice tests:
// it records the request body and answers with reply, or a usable
// default when reply is empty.
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
