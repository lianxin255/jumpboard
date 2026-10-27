package e2e

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// holdPort opens a wildcard listener on a free port and holds it for the
// test, so a rule on that port cannot be bound.
func holdPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":"+strconv.Itoa(freePort(t)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// TestAIDiagnoseAdviceCard walks the whole path: a port that cannot be
// bound fails the apply, the diagnose button appears, the advice card
// shows, its command copies — and nothing about the configuration moved.
func TestAIDiagnoseAdviceCard(t *testing.T) {
	a := testApp(t)
	port := holdPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "诊断演示", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}

	stepTitle := "查出占用端口的进程"
	command := "lsof -i :" + strconv.Itoa(port)
	advice := `{"summary":"监听端口被占用","causes":["已有进程监听该端口"],` +
		`"steps":[{"title":"` + stepTitle + `","detail":"看是哪个进程占着端口",` +
		`"command":"` + command + `","risk":"结束进程前先确认它是什么"}]}`
	srv := fakeOpenAI(t, advice)
	a.SetAI(srv.URL, "test-model")

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the failure bar", func() bool { return tt.HasText("无法监听") })

	if err := tt.Click("AI 诊断"); err != nil {
		t.Fatal(err)
	}
	// The card's header is there from the busy frame on; only the advice
	// itself proves the request landed.
	waitFor(t, tt, "the advice card", func() bool { return tt.HasText("监听端口被占用") })
	if !tt.HasText("仅供参考") {
		t.Errorf("the card should say what it is: %v", tt.Texts())
	}
	for _, want := range []string{
		"监听端口被占用", "可能原因", "已有进程监听该端口", stepTitle, command, "风险：",
	} {
		if !tt.HasText(want) {
			t.Errorf("the card should show %q: %v", want, tt.Texts())
		}
	}

	if err := tt.Click("复制命令-" + stepTitle); err != nil {
		t.Fatalf("the step's copy button: %v", err)
	}
	waitFor(t, tt, "the command copied", func() bool { return a.Copied() == command })

	// The diagnosis is display only: the rule set is exactly what was
	// saved, field for field.
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 {
		t.Fatalf("the diagnosis must not touch the rules: %v %v", list, err)
	}
	got := list[0]
	if got.Name != "诊断演示" || got.ListenPort != port || !got.Enabled ||
		got.UpstreamHost != "10.0.0.1" || got.UpstreamPort != 3306 {
		t.Errorf("the mapping changed under the diagnosis: %+v", got)
	}
}

// TestAIDiagnoseHiddenWithoutAI checks that the diagnose entry stays
// hidden until AI is configured, while the failure itself still shows.
func TestAIDiagnoseHiddenWithoutAI(t *testing.T) {
	a := testApp(t)
	port := holdPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "未配置演示", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the failure bar", func() bool { return tt.HasText("无法监听") })
	if tt.HasText("AI 诊断") {
		t.Errorf("the diagnose entry should stay hidden without AI: %v", tt.Texts())
	}
	if !tt.HasText("无法监听") {
		t.Errorf("the failure itself should still show: %v", tt.Texts())
	}
}

// TestAIDiagnoseServerError checks that a broken diagnosis leaves the
// error visible and the app running: the ordinary flow still executes
// afterwards.
func TestAIDiagnoseServerError(t *testing.T) {
	a := testApp(t)
	port := holdPort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "错误演示", Protocol: model.TCP, Enabled: true, ListenPort: port,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
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
	a.SetAI(srv.URL, "test-model")

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the failure bar", func() bool { return tt.HasText("无法监听") })
	if err := tt.Click("AI 诊断"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the diagnosis error", func() bool { return tt.HasText("AI 服务返回 500") })
	if busy, _, _ := a.Diagnosis(); busy {
		t.Error("busy should clear after a failed diagnosis")
	}

	// The flow is not wedged: the preflight still runs and still reports
	// the taken port.
	if err := a.TestConfig(); err == nil || !strings.Contains(err.Error(), "预检失败") {
		t.Errorf("the check should still fail on the taken port: %v", err)
	}
}
