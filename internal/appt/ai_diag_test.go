package appt

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// diagGateway is an in-process AI endpoint for the diagnosis tests: it
// records the request body and answers with a small usable advice.
type diagGateway struct {
	mu   sync.Mutex
	body []byte
}

func (g *diagGateway) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.body = b
	g.mu.Unlock()
	content := `{"summary":"端口被占用","causes":["已有进程监听"],` +
		`"steps":[{"title":"查占用","detail":"用 lsof 查","command":"lsof -i :9000"}]}`
	reply, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(reply)
}

// content returns the user message of the captured request.
func (g *diagGateway) content(t *testing.T) string {
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

// heldPort returns a port this test holds open until cleanup, so the
// engine cannot bind it.
func heldPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// sparePort returns a port the OS currently has free.
func sparePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestDiagnoseFailureAssemblesContext pins what the diagnosis is built
// from: the error itself, the one mapping on the failed port, both logs'
// tails — and never the rest of the table.
func TestDiagnoseFailureAssemblesContext(t *testing.T) {
	a := droppedApp(t)
	port := heldPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "诊断目标库", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.8.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "无关映射", Protocol: model.TCP, Enabled: true, ListenPort: sparePort(t),
		UpstreamHost: "10.8.0.2", UpstreamPort: 80,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}
	if err := os.WriteFile(filepath.Join(a.Paths.LogsDir, "access.log"),
		[]byte("安静的一天\nDIAG-ACCESS-MARK\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &diagGateway{}
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.DiagnoseFailure("apply"); err != nil {
		t.Fatalf("DiagnoseFailure: %v", err)
	}
	if busy, errText, advice := a.Diagnosis(); busy || errText != "" || advice == nil {
		t.Fatalf("diagnosis = %v %q %v", busy, errText, advice)
	}

	content := g.content(t)
	for _, want := range []string{
		"操作：应用配置",
		strconv.Itoa(port), // the error names the port
		"诊断目标库",            // the involved mapping alone
		"应用规则失败",           // the operation log line Apply wrote
		"DIAG-ACCESS-MARK", // the access log tail
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the diagnosis context misses %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "无关映射") {
		t.Error("an unrelated mapping must not travel; only the involved one is sent")
	}
}

// TestDiagnoseFailureCheckKind checks that a diagnosis of a failed
// preflight names 预检 as the operation, not 应用配置.
func TestDiagnoseFailureCheckKind(t *testing.T) {
	a := droppedApp(t)
	port := heldPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "预检目标", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.8.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.TestConfig(); err == nil {
		t.Fatal("checking onto a taken port should fail")
	}
	if kind := a.FailKind(); kind != "check" {
		t.Fatalf("FailKind = %q, want check", kind)
	}

	g := &diagGateway{}
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.DiagnoseFailure("check"); err != nil {
		t.Fatal(err)
	}
	if content := g.content(t); !strings.Contains(content, "操作：预检") {
		t.Errorf("a check failure should name 预检:\n%s", content)
	}
}

// TestDiagnoseFailureNeedsFailure checks the two refusals before any
// request goes out: no AI configured, and nothing failed yet.
func TestDiagnoseFailureNeedsFailure(t *testing.T) {
	a := droppedApp(t)
	if err := a.DiagnoseFailure("apply"); err == nil ||
		!strings.Contains(err.Error(), "还没有配置 AI 服务") {
		t.Fatalf("without AI: %v", err)
	}
	g := &diagGateway{}
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.DiagnoseFailure("apply"); err == nil ||
		!strings.Contains(err.Error(), "还没有可诊断的失败") {
		t.Fatalf("without a failure: %v", err)
	}
}

// TestDiagnoseFailureUnreadableAccessLog checks that a broken access
// log degrades to a note in the context instead of failing the
// diagnosis.
func TestDiagnoseFailureUnreadableAccessLog(t *testing.T) {
	a := droppedApp(t)
	port := heldPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "目标", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.8.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}
	// The failed apply leaves the engine holding access.log open, and
	// Windows will not remove an open file, so the handle is released
	// before the log is replaced. Remove then reports a real problem
	// instead of one swallowed by "_ =", and MkdirAll tolerates the
	// directory already being there.
	if err := a.Eng.Stop(); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(a.Paths.LogsDir, "access.log")
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(logPath) })

	g := &diagGateway{}
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.DiagnoseFailure("apply"); err != nil {
		t.Fatalf("an unreadable log should not fail the diagnosis: %v", err)
	}
	if content := g.content(t); !strings.Contains(content, "读取失败") {
		t.Errorf("the degradation should be said:\n%s", content)
	}
}

// TestDiagnoseFailureNewFailureClearsAdvice pins that a diagnosis belongs
// to the failure that asked for it: a new failed apply or check retires
// the advice, instead of leaving it under the next red bar.
func TestDiagnoseFailureNewFailureClearsAdvice(t *testing.T) {
	a := droppedApp(t)
	port := heldPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "目标", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.8.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}

	g := &diagGateway{}
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}
	if err := a.DiagnoseFailure("apply"); err != nil {
		t.Fatal(err)
	}
	if _, _, advice := a.Diagnosis(); advice == nil {
		t.Fatal("the diagnosis should have landed first")
	}

	// The port is still held, so a second apply fails again — and the old
	// advice must go with the old failure.
	if err := a.Apply(); err == nil {
		t.Fatal("applying again onto the taken port should fail")
	}
	busy, errText, advice := a.Diagnosis()
	if busy || errText != "" || advice != nil {
		t.Fatalf("a new failure should retire the advice: %v %q %v", busy, errText, advice)
	}

	// The check's failure retires one too.
	if err := a.DiagnoseFailure("apply"); err != nil {
		t.Fatal(err)
	}
	if _, _, advice = a.Diagnosis(); advice == nil {
		t.Fatal("the diagnosis should have landed again")
	}
	if err := a.TestConfig(); err == nil {
		t.Fatal("checking onto the taken port should fail")
	}
	_, _, advice = a.Diagnosis()
	if advice != nil {
		t.Fatalf("a new check failure should retire the advice: %v", advice)
	}
}

// TestDiagnoseFailureServerError checks that a gateway that answers 500
// leaves the failure visible and the app running: the ordinary paths
// still work after a broken diagnosis.
func TestDiagnoseFailureServerError(t *testing.T) {
	a := droppedApp(t)
	port := heldPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "目标", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.8.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "diagnosis down"},
		})
	}))
	t.Cleanup(srv.Close)
	if err := a.SetAI(srv.URL, "test-model"); err != nil {
		t.Fatal(err)
	}

	err := a.DiagnoseFailure("apply")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("a 500 endpoint should fail the diagnosis: %v", err)
	}
	busy, errText, advice := a.Diagnosis()
	if busy {
		t.Error("busy must clear after a failure")
	}
	if errText == "" || advice != nil {
		t.Fatalf("the failure should be kept for the card: %q %v", errText, advice)
	}

	// The main flow is unaffected: the check still runs and still
	// reports the taken port.
	if err := a.TestConfig(); err == nil || !strings.Contains(err.Error(), "预检失败") {
		t.Errorf("the check should still fail on the taken port: %v", err)
	}
}
