package appt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
)

// interpretGateway is an in-process AI endpoint for the interpretation
// tests: it counts and records the requests and answers with a small
// usable interpretation.
type interpretGateway struct {
	mu   sync.Mutex
	body []byte
	hits int
}

func (g *interpretGateway) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.body, g.hits = b, g.hits+1
	g.mu.Unlock()
	content := `{"summary":"ok","listeners":["a"],"forwards":["b"],"risks":["c"],"notes":["d"]}`
	reply, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(reply)
}

// content returns the user message of the captured request.
func (g *interpretGateway) content(t *testing.T) string {
	t.Helper()
	g.mu.Lock()
	body := g.body
	g.mu.Unlock()
	if body == nil {
		t.Fatal("the gateway saw no request")
	}
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	for _, m := range req.Messages {
		if m.Role == "user" {
			return m.Content
		}
	}
	t.Fatal("no user message in the request")
	return ""
}

// hitCount returns how many requests reached the gateway.
func (g *interpretGateway) hitCount(t *testing.T) int {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.hits
}

// serveInterpret wires the gateway in as the app's AI endpoint.
func serveInterpret(t *testing.T, a *App, g *interpretGateway) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}
}

// TestInterpretConfSendsLocalParse pins what a conf's interpretation is
// built from: the file itself and the local parse's summary. Who listens
// and where it forwards is decided on this machine, before anything
// travels — the model restates the summary, never counts the file itself.
func TestInterpretConfSendsLocalParse(t *testing.T) {
	a := droppedApp(t)
	conf := nginx.Generate([]model.Mapping{
		{Name: "解读甲", Protocol: model.TCP, Enabled: true, ListenPort: 17011,
			UpstreamHost: "10.8.0.1", UpstreamPort: 3306},
		{Name: "解读乙", Protocol: model.TCP, Enabled: true, ListenPort: 17012,
			UpstreamHost: "10.8.0.2", UpstreamPort: 6379},
	}, nginx.GenOptions{})
	path := filepath.Join(t.TempDir(), "site.conf")
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &interpretGateway{}
	serveInterpret(t, a, g)
	if err := a.InterpretFile("conf", path); err != nil {
		t.Fatalf("InterpretFile: %v", err)
	}
	busy, errText, interp, target, source := a.Interpretation()
	if busy || errText != "" || interp == nil || target != "site.conf" || source != "logs" {
		t.Fatalf("interpretation = %v %q %v %q %q", busy, errText, interp, target, source)
	}

	content := g.content(t)
	for _, want := range []string{
		"文件名：site.conf",
		"监听端口：17011",
		"监听端口：17012",
		"上游：10.8.0.1:3306",
		"上游：10.8.0.2:6379",
		"解读甲",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the interpretation context misses %q:\n%s", want, content)
		}
	}
}

// TestInterpretConfParseFailureDegrades checks that a conf the local
// parser cannot read still travels — with a note that says the parse
// failed, so the model knows the summary is absent rather than empty.
func TestInterpretConfParseFailureDegrades(t *testing.T) {
	a := droppedApp(t)
	path := filepath.Join(t.TempDir(), "truncated.conf")
	if err := os.WriteFile(path, []byte("http {\n server {\n listen 8080;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &interpretGateway{}
	serveInterpret(t, a, g)
	if err := a.InterpretFile("conf", path); err != nil {
		t.Fatalf("a conf the parser cannot read should still interpret: %v", err)
	}
	if content := g.content(t); !strings.Contains(content, "本地解析失败") ||
		!strings.Contains(content, "按原文解读") {
		t.Errorf("the degradation should be said:\n%s", content)
	}
	if _, errText, interp, _, _ := a.Interpretation(); errText != "" || interp == nil {
		t.Fatalf("the interpretation should have landed: %q %v", errText, interp)
	}
}

// TestInterpretFileTooLarge keeps the size cap a refusal, not a
// truncation: no request may travel for a file that was never read whole.
func TestInterpretFileTooLarge(t *testing.T) {
	a := droppedApp(t)
	path := filepath.Join(t.TempDir(), "big.conf")
	if err := os.WriteFile(path, bytes.Repeat([]byte("listen 127.0.0.1:8080;\n"), 20_000), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &interpretGateway{}
	serveInterpret(t, a, g)
	err := a.InterpretFile("conf", path)
	if err == nil || !strings.Contains(err.Error(), "超过 256 KB") ||
		!strings.Contains(err.Error(), "big.conf") {
		t.Fatalf("an oversized file should be refused by limit and name: %v", err)
	}
	if n := g.hitCount(t); n != 0 {
		t.Errorf("no request should travel for a refused file, saw %d", n)
	}
}

// TestInterpretFileMissing checks that a path that points nowhere is an
// error naming the file, never a crash.
func TestInterpretFileMissing(t *testing.T) {
	a := droppedApp(t)
	g := &interpretGateway{}
	serveInterpret(t, a, g)
	err := a.InterpretFile("", filepath.Join(t.TempDir(), "gone", "missing.conf"))
	if err == nil || !strings.Contains(err.Error(), "读取") ||
		!strings.Contains(err.Error(), "missing.conf") {
		t.Fatalf("a missing file is an error, not a crash: %v", err)
	}
	if n := g.hitCount(t); n != 0 {
		t.Errorf("no request should travel: %d", n)
	}
}

// TestInterpretKindForPath pins what a picked file is taken for by its
// name alone: a .conf, an error log, and everything else an access log.
func TestInterpretKindForPath(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"site.conf", "conf"},
		{"dropped.Conf", "conf"},
		{"error.log", "error"},
		{"ERROR.LOG", "error"},
		{"/var/log/nginx/my-error.log.txt", "error"},
		{"access.log", "access"},
		{"notes.txt", "access"},
	} {
		if got := interpretKindForPath(tc.path); got != tc.want {
			t.Errorf("interpretKindForPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestInterpretAccessLogFromLogsPage walks the logs page's entry: the
// engine's own access log travels as content, and a log read that is
// turned off is refused before anything is sent.
func TestInterpretAccessLogFromLogsPage(t *testing.T) {
	a := droppedApp(t)
	if err := os.WriteFile(filepath.Join(a.Paths.LogsDir, "access.log"),
		[]byte("127.0.0.1 - - [26/Oct/2026:10:00:00] \"GET / HTTP/1.1\" 200\nINTERP-LOG-MARK\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if !a.NginxLogRead() {
		t.Fatal("the access log read should be on by default")
	}

	g := &interpretGateway{}
	serveInterpret(t, a, g)
	if err := a.InterpretAccessLog(); err != nil {
		t.Fatalf("InterpretAccessLog: %v", err)
	}
	if _, errText, interp, _, source := a.Interpretation(); errText != "" || interp == nil || source != "logs" {
		t.Fatalf("the interpretation should land as the logs page's: %q %v %q", errText, interp, source)
	}
	if content := g.content(t); !strings.Contains(content, "INTERP-LOG-MARK") {
		t.Errorf("the log tail should travel:\n%s", content)
	}

	a.SetNginxLogRead(false)
	if err := a.InterpretAccessLog(); err == nil ||
		!strings.Contains(err.Error(), "访问日志读取已关闭") {
		t.Fatalf("a closed log should be refused: %v", err)
	}
}

// TestInterpretNeedsAI checks the refusals before any request travels:
// every entry answers the unconfigured AI first, and the dropped-conf
// entry says when there is nothing imported to interpret.
func TestInterpretNeedsAI(t *testing.T) {
	a := droppedApp(t)
	path := filepath.Join(t.TempDir(), "site.conf")
	if err := os.WriteFile(path, []byte("http {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func() error{
		"file":   func() error { return a.InterpretFile("conf", path) },
		"conf":   a.InterpretDroppedConf,
		"access": a.InterpretAccessLog,
	} {
		if err := fn(); err == nil || !strings.Contains(err.Error(), "还没有配置 AI 服务") {
			t.Errorf("%s without AI configured: %v", name, err)
		}
	}

	g := &interpretGateway{}
	serveInterpret(t, a, g)
	if err := a.InterpretDroppedConf(); err == nil ||
		!strings.Contains(err.Error(), "还没有刚导入的配置可解读") {
		t.Fatalf("without an import: %v", err)
	}
}

// TestInterpretDroppedConfTravels walks the import's follow-up: a dropped
// .conf is remembered, its interpretation lands as the import's, and the
// two dismissals fold their own pieces away.
func TestInterpretDroppedConfTravels(t *testing.T) {
	a := droppedApp(t)
	conf := nginx.Generate([]model.Mapping{
		{Name: "拖入的库", Protocol: model.TCP, Enabled: true, ListenPort: 17015,
			UpstreamHost: "10.8.0.2", UpstreamPort: 6379},
	}, nginx.GenOptions{})
	path := filepath.Join(t.TempDir(), "dropped.conf")
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.ImportDroppedFiles([]string{path}); err != nil {
		t.Fatal(err)
	}
	imp, ok := a.LastConfImport()
	if !ok || imp.Path != path || imp.Name != "dropped.conf" || imp.Count != 1 {
		t.Fatalf("the import should be remembered: %+v ok=%v", imp, ok)
	}

	g := &interpretGateway{}
	serveInterpret(t, a, g)
	if err := a.InterpretDroppedConf(); err != nil {
		t.Fatalf("InterpretDroppedConf: %v", err)
	}
	if _, errText, interp, target, source := a.Interpretation(); errText != "" ||
		interp == nil || target != "dropped.conf" || source != "import" {
		t.Fatalf("the interpretation should land on the import: %q %v %q %q",
			errText, interp, target, source)
	}
	if content := g.content(t); !strings.Contains(content, "拖入的库") {
		t.Errorf("the dropped conf's local summary should travel:\n%s", content)
	}

	a.DismissConfImport()
	if _, ok := a.LastConfImport(); ok {
		t.Error("the import bar should close")
	}
	a.DismissInterpretation()
	if busy, errText, interp, target, source := a.Interpretation(); busy ||
		errText != "" || interp != nil || target != "" || source != "" {
		t.Errorf("the card should fold away: %v %q %v %q %q", busy, errText, interp, target, source)
	}
}
