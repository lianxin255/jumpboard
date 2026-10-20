package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// The end-to-end tests below apply rules in the app and push real traffic
// through them: the whole point of the app, with no nginx in between.

// freePort returns a port nothing listens on.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// upstream starts an HTTP server that answers "upstream-ok".
func upstream(t *testing.T) (port int, stop func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream-ok")
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(l)
	return l.Addr().(*net.TCPAddr).Port, func() { _ = srv.Close() }
}

// httpGet fetches a URL with no proxy and a short timeout.
func httpGet(t *testing.T, url string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// waitPort waits until a port is open or closed.
func waitPort(t *testing.T, port int, open bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		up := err == nil
		if up {
			c.Close()
		}
		if up == open {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	state := "open"
	if !open {
		state = "closed"
	}
	t.Fatalf("port %d never became %s", port, state)
}

// TestApplyForwardsTraffic is the whole point: a mapping applied in the
// window takes real traffic, and stopping releases the port.
func TestApplyForwardsTraffic(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "测试站点", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the mapping", func() bool { return tt.HasText("测试站点") })

	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the engine running", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)

	body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen))
	if err != nil {
		t.Fatalf("request through the proxy: %v", err)
	}
	if body != "upstream-ok" {
		t.Fatalf("body = %q, want upstream-ok", body)
	}
	if !tt.HasText("运行中") {
		t.Errorf("the window should show the engine running: %v", tt.Texts())
	}
	if a.Dirty() {
		t.Error("applying should clear the dirty mark")
	}
	snaps, _ := a.Store.ListSnapshots(5)
	if len(snaps) == 0 {
		t.Error("applying should store a snapshot")
	}
	if n, _ := a.Store.CountLogs(); n == 0 {
		t.Error("applying should log it")
	}

	// The counter the window shows counts the request.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	rules := a.RuleStatus()
	if len(rules) != 1 || !rules[0].Listening {
		t.Fatalf("the rule should be listening: %+v", rules)
	}
	if rules[0].Connections == 0 {
		t.Errorf("the rule should have counted the request: %+v", rules[0])
	}

	if err := a.StopEngine(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, false)
	if a.Eng.Status() != "stopped" {
		t.Errorf("the engine should be stopped: %s", a.Eng.Status())
	}
}

// TestPortConflictLeavesRulesRunning checks the transaction: a rule that
// cannot bind fails the apply, and what was running keeps running.
func TestPortConflictLeavesRulesRunning(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "好的", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatalf("the first apply: %v", err)
	}
	waitPort(t, listen, true)

	// A rule whose port somebody else holds.
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "冲突", Protocol: model.TCP, Enabled: true,
		ListenPort:   busy.Addr().(*net.TCPAddr).Port,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("applying onto a taken port should fail")
	}
	if a.ApplyError() == "" {
		t.Error("the failure should be shown to the user")
	}

	// The first rule still forwards.
	if body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil || body != "upstream-ok" {
		t.Errorf("the running rule should be untouched: %q %v", body, err)
	}
	// The engine is still running, with the rule it had.
	if a.Eng.Status() != "running" {
		t.Errorf("the engine should keep running: %s", a.Eng.Status())
	}
	rules := a.RuleStatus()
	listening := 0
	for _, r := range rules {
		if r.Listening {
			listening++
		}
	}
	if listening != 1 {
		t.Errorf("exactly the old rule should be listening: %+v", rules)
	}
}

// TestCheckConfigReportsProblems checks the validate button: it names what
// would stop the rules from starting, without changing them.
func TestCheckConfigReportsProblems(t *testing.T) {
	a := testApp(t)
	listen := freePort(t)

	// Two rules on one port.
	for _, name := range []string{"一", "二"} {
		if _, err := a.Store.CreateMapping(model.Mapping{
			Name: name, Protocol: model.TCP, Enabled: true, ListenPort: listen,
			UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.TestConfig(); err == nil {
		t.Fatal("two rules on one port should fail the check")
	}

	// A resolvable single rule passes.
	if err := a.Store.ReplaceAllMappings([]model.Mapping{{
		ID: "1", Name: "好", Protocol: model.TCP, Enabled: true, ListenPort: listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.TestConfig(); err != nil {
		t.Errorf("one reachable rule should pass: %v", err)
	}
}

// TestSnapshotRollback restores an earlier configuration.
func TestSnapshotRollback(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "站点", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)

	snaps, err := a.Store.ListSnapshots(5)
	if err != nil || len(snaps) == 0 {
		t.Fatalf("no snapshot: %v", err)
	}

	// Add a second rule, so the snapshot and the database differ.
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "后来加的", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := a.RollbackSnapshot(snaps[0]); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	list, err := a.Store.ListMappings()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "站点" {
		t.Errorf("the rollback should restore one rule: %+v", list)
	}
	if body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil || body != "upstream-ok" {
		t.Errorf("the restored rule should serve: %q %v", body, err)
	}
}

// TestQuitStopsEverything checks that closing the app closes its listeners.
func TestQuitStopsEverything(t *testing.T) {
	a := testApp(t)
	listen := freePort(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "x", Protocol: model.TCP, Enabled: true, ListenPort: listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)

	a.Close()
	waitPort(t, listen, false)
}
