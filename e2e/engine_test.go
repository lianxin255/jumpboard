package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

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

// httpGet fetches a URL, with a short timeout so a hang fails the test.
func httpGet(t *testing.T, url string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	// The proxy variables of the machine would send loopback traffic
	// through a VPN; the test talks to nginx directly.
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// waitPort waits until a port answers, or fails.
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
	t.Fatalf("port %d never became %v", port, map[bool]string{true: "open", false: "closed"}[open])
}

// TestApplyStartsNginxAndForwards is the whole point of the app: a mapping
// applied in the window takes real traffic, and stopping releases the port.
func TestApplyStartsNginxAndForwards(t *testing.T) {
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

	// The mapping went straight into the store, so the app's cache is
	// stale until it reloads, which the UI does after every action.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the mapping", func() bool { return tt.HasText("测试站点") })

	// Apply, which starts the engine.
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "nginx running", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)

	body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen))
	if err != nil {
		t.Fatalf("request through nginx: %v", err)
	}
	if body != "upstream-ok" {
		t.Fatalf("body = %q, want upstream-ok", body)
	}
	if !tt.HasText("运行中") {
		t.Errorf("the window should show nginx running: %v", tt.Texts())
	}
	// Applying clears the dirty mark and stores a snapshot.
	if a.Dirty() {
		t.Error("applying should clear the dirty mark")
	}
	snaps, _ := a.Store.ListSnapshots(5)
	if len(snaps) == 0 {
		t.Error("applying should store a snapshot")
	}

	// Stop, which must release the port.
	if err := a.StopEngine(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, false)
	if a.Eng.Status() != "stopped" {
		t.Errorf("engine should be stopped: %s", a.Eng.Status())
	}
	tt.Frame()
	if !tt.HasText("已停止") && !tt.HasText("nginx") {
		t.Errorf("the window should show the engine stopped: %v", tt.Texts())
	}
}

// TestBadConfigRollsBack checks that a configuration nginx rejects leaves
// the previous one running.
func TestBadConfigRollsBack(t *testing.T) {
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
		t.Fatalf("the first apply should work: %v", err)
	}
	defer a.Eng.Stop()
	waitPort(t, listen, true)
	good, err := a.Eng.ReadConf()
	if err != nil {
		t.Fatal(err)
	}

	// A mapping whose extra directives break the configuration.
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "坏的", Protocol: model.HTTP, Enabled: true,
		ListenPort: freePort(t), UpstreamHost: "127.0.0.1", UpstreamPort: 80,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
		ExtraDirectives: "this_is_not_a_directive",
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err == nil {
		t.Fatal("the broken configuration should fail")
	}
	after, err := a.Eng.ReadConf()
	if err != nil {
		t.Fatal(err)
	}
	if after != good {
		t.Errorf("the file should have been rolled back:\n--- after ---\n%s\n--- good ---\n%s", after, good)
	}
	// The working proxy still works.
	if body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil || body != "upstream-ok" {
		t.Errorf("the old configuration should still serve: %q %v", body, err)
	}
	if a.ApplyError() == "" {
		t.Error("the failure should be shown to the user")
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
	defer a.Eng.Stop()
	waitPort(t, listen, true)

	snaps, err := a.Store.ListSnapshots(5)
	if err != nil || len(snaps) == 0 {
		t.Fatalf("no snapshot to roll back to: %v", err)
	}
	sn := snaps[0]

	// Change the mapping so the running configuration and the file differ.
	m, _ := a.Store.ListMappings()
	m[0].Name = "改了"
	if err := a.Store.UpdateMapping(m[0]); err != nil {
		t.Fatal(err)
	}
	if err := a.RollbackSnapshot(sn); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil || body != "upstream-ok" {
		t.Errorf("the rolled back configuration should serve: %q %v", body, err)
	}
	if !a.Dirty() {
		t.Error("after a rollback the database and the file differ, which is dirty")
	}
}

// TestLogsAndHistoryPages open the two read-only pages.
func TestLogsAndHistoryPages(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "留着", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	defer a.Eng.Stop()
	a.Log.Info("这是一条测试日志")
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the log line", func() bool { return tt.HasText("这是一条测试日志") })

	if err := tt.Click("nginx 日志"); err != nil {
		t.Fatalf("the nginx log tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText("access 日志") {
		t.Errorf("the nginx log page should show: %v", tt.Texts())
	}

	if err := tt.Click("历史"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("配置历史") {
		t.Errorf("the history page should show: %v", tt.Texts())
	}
	if !tt.HasText("查看") {
		t.Errorf("the snapshot row should be there: %v", tt.Texts())
	}
	if err := tt.Click("查看"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("配置快照") {
		t.Errorf("the snapshot reader should open: %v", tt.Texts())
	}
}

// TestQuitStopsNginx checks that closing the app leaves nothing running.
func TestQuitStopsNginx(t *testing.T) {
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
	if a.Eng.Status() != "stopped" {
		t.Errorf("closing should stop nginx: %s", a.Eng.Status())
	}
}

// ensure the tester import stays used if the file gains no UI test.
var _ = ui.NewTester
