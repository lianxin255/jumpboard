package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The tests in this file drive the window the way the old Playwright suite
// did, through ui.Tester instead of a debug port: the packaged mygo binary
// exposes none, so a browser driver cannot attach to it. Each one walks the
// real view and checks the effect on real traffic or the real store.

// httpRule is an HTTP mapping that proxies to a live upstream.
func httpRule(t *testing.T, name string, listen, upstream int) model.Mapping {
	t.Helper()
	return model.Mapping{
		Name: name, Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upstream,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}
}

// taggedUpstream serves one fixed body, so a test can tell two apart.
func taggedUpstream(t *testing.T, body string) (port int, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	return ln.Addr().(*net.TCPAddr).Port, func() { _ = srv.Close() }
}

// statusOf fetches a URL and returns its status code.
func statusOf(t *testing.T, url string) (int, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// confirm clicks the confirmation dialog's own button.
//
// The dialog's button is labeled 确认 plus its text, so the click lands on
// it and not on the button in the page that opened the dialog.
func confirm(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	tt.Frame()
	if err := tt.Click("确认" + label); err != nil {
		t.Fatalf("confirming %q: %v\nshown: %v", label, err, tt.Texts())
	}
}

// waitBody polls until the proxy answers with a body containing want.
func waitBody(t *testing.T, port int, want string) string {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	var last string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, err := httpGet(t, url)
		if err == nil {
			last = body
			if strings.Contains(body, want) {
				return body
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("proxy on %d never answered %q, last %q", port, want, last)
	return last
}

// waitStatus polls until the proxy answers with the status want.
func waitStatus(t *testing.T, port, want int) int {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	var last int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		code, err := statusOf(t, url)
		if err == nil {
			last = code
			if code == want {
				return code
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("proxy on %d never answered %d, last %d", port, want, last)
	return last
}

// TestEngineSwitch drives the forwarding service's master switch in the
// rail's foot: on while the engine runs, and turning it off asks the
// confirmation before the forwarding really stops; turning it on again
// applies the rules back, which is what a start is. The confirmation's
// own words are the ones that stop it.
func TestEngineSwitch(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), httpRule(t, "开关验证", listen, upPort)); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the engine running", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)
	if !tt.HasText("运行中") {
		t.Errorf("the window should say the engine is running: %v", tt.Texts())
	}
	if !tt.HasText("转发服务") {
		t.Errorf("the rail's foot should name the service: %v", tt.Texts())
	}
	// The switch is held while the window is busy, so the click waits for
	// the calm the apply leaves behind.
	waitFor(t, tt, "the window calm", func() bool {
		_, _, _, _, _, _, busy, _ := a.Snapshot()
		return !busy
	})
	tt.Frame()

	// Turning the service off asks first; nothing has stopped yet.
	if err := tt.Click("转发服务开关"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ConfirmState() == nil || !tt.HasText("停止引擎") {
		t.Fatalf("stopping should ask first: %v", tt.Texts())
	}
	confirm(t, tt, "停止")
	waitFor(t, tt, "the engine stopped", func() bool { return a.Eng.Status() == "stopped" })
	waitPort(t, listen, false)

	// Turning it back on applies the rules again, without asking.
	waitFor(t, tt, "the window calm", func() bool {
		_, _, _, _, _, _, busy, _ := a.Snapshot()
		return !busy
	})
	tt.Frame()
	if err := tt.Click("转发服务开关"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the engine running again", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)
}

// TestMappingTrafficLifecycle creates a mapping in the form, applies it,
// switches its upstream, and deletes it, checking the traffic each time.
func TestMappingTrafficLifecycle(t *testing.T) {
	a := testApp(t)
	upA, stopA := taggedUpstream(t, "upstream-A")
	defer stopA()
	upB, stopB := taggedUpstream(t, "upstream-B")
	defer stopB()
	listen := freePort(t)

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "名称", "流量验证")
	if err := tt.Click("HTTP 反代"); err != nil {
		t.Fatalf("choosing HTTP: %v\nshown: %v", err, tt.Texts())
	}
	tt.Frame()
	fillField(t, tt, "监听端口", fmt.Sprint(listen))
	fillField(t, tt, "上游主机", "127.0.0.1")
	fillField(t, tt, "上游端口", fmt.Sprint(upA))
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping saved", func() bool { return tt.HasText("流量验证") })

	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)
	if got := waitBody(t, listen, "upstream-A"); got != "upstream-A" {
		t.Fatalf("traffic should hit upstream A, got %q", got)
	}

	// Point the rule at the second upstream and apply again.
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "上游端口", fmt.Sprint(upB))
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the edit saved", func() bool { return a.Dirty() })
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	if got := waitBody(t, listen, "upstream-B"); got != "upstream-B" {
		t.Fatalf("traffic should switch to upstream B, got %q", got)
	}

	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	confirm(t, tt, "确认删除")
	waitFor(t, tt, "the mapping gone", func() bool {
		list, _ := a.Store.ListMappings(a.ActiveWorkspace())
		return len(list) == 0
	})
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the apply to finish", func() bool { return !a.Dirty() })
	waitPort(t, listen, false)
}

// TestAppLogClear checks that clearing the operation log from the log page
// leaves only the line that records the clearing.
func TestAppLogClear(t *testing.T) {
	a := testApp(t)
	a.Log.Info("配置应用成功")
	a.Log.Info("新建映射-日志验证")
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("配置应用成功") || !tt.HasText("新建映射-日志验证") {
		t.Fatalf("the operations should be listed: %v", tt.Texts())
	}

	if err := tt.Click("清空操作日志"); err != nil {
		t.Fatal(err)
	}
	confirm(t, tt, "清空")
	waitFor(t, tt, "the log cleared", func() bool { return tt.HasText("操作日志已清空") })
	if tt.HasText("配置应用成功") || tt.HasText("新建映射-日志验证") {
		t.Errorf("clearing should drop the old lines: %v", tt.Texts())
	}
}

// TestAccessLogClear checks that a real request shows in the access log and
// that clearing it from the log page removes the line.
func TestAccessLogClear(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), httpRule(t, "访问日志验证", listen, upPort)); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)
	if _, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("访问日志"); err != nil {
		t.Fatal(err)
	}
	// The page reads the log file every two seconds of its own clock, which
	// stands still until Advance moves it.
	tt.Advance(3 * time.Second)
	tt.Frame()
	if !tt.HasText("GET /") {
		t.Fatalf("the request should show in the access log: %v", tt.Texts())
	}

	if err := tt.Click("清空访问日志"); err != nil {
		t.Fatal(err)
	}
	confirm(t, tt, "清空")
	waitFor(t, tt, "the access log cleared", func() bool {
		tt.Advance(3 * time.Second)
		tt.Frame()
		return tt.HasText("还没有访问记录")
	})
	if tt.HasText("GET /") {
		t.Errorf("the cleared log should not keep the request: %v", tt.Texts())
	}
}

// TestWhitelistBlocksTraffic adds an address through the settings page, turns
// the whitelist on, and checks that a request from outside it is refused
// while one is served again once the whitelist is off.
func TestWhitelistBlocksTraffic(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := taggedUpstream(t, "whitelist-ok")
	defer stopUp()
	listen := freePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), httpRule(t, "白名单验证", listen, upPort)); err != nil {
		t.Fatal(err)
	}

	tt := tester(t, a, 1200, 900)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// A malformed address is refused and never stored.
	fillField(t, tt, "白名单地址", "not-an-ip")
	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the format error", func() bool { return tt.HasText("格式不正确") })
	if list, _ := a.Store.ListWhitelist(); len(list) != 0 {
		t.Fatalf("a bad address was stored: %+v", list)
	}

	fillField(t, tt, "白名单地址", "10.0.0.0/24")
	// The format error's toast holds the bottom of the window, over the
	// add button, and its countdown runs on the tester's clock and only
	// while the pointer is off it — the malformed add's click left the
	// pointer resting on the card, so the toast never left on its own.
	// Filling the field moves the pointer away; the advance waits the
	// toast out, as a user would, before the add's click crosses the band
	// the card held.
	tt.Advance(5 * time.Second)
	tt.Frame()
	if err := tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	// The store, not the screen, says the address listed: the field alone
	// shows the text either way, and a click the toast ate would pass a
	// text check while adding nothing.
	waitFor(t, tt, "the address listed", func() bool {
		list, _ := a.Store.ListWhitelist()
		return len(list) == 1 && list[0].IP == "10.0.0.0/24"
	})

	if err := tt.Click("启用白名单"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the whitelist on", a.WhitelistOn)

	if err := tt.Click("映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)
	if got := waitStatus(t, listen, http.StatusForbidden); got != http.StatusForbidden {
		t.Fatalf("an address outside the list should get 403, got %d", got)
	}

	// Removing the address and switching the whitelist off serves traffic.
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The failed add left an error toast, and a toast holds the bottom of
	// the window until its time is up. The caption size the redesign
	// raised slid the row's 移除 into that band, so the test waits the
	// toast out first, as a user would.
	tt.Advance(5 * time.Second)
	tt.Frame()
	reveal(t, tt, "移除")
	if err := tt.Click("移除"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address removed", func() bool {
		list, _ := a.Store.ListWhitelist()
		return len(list) == 0
	})
	if err := tt.Click("启用白名单"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the whitelist off", func() bool { return !a.WhitelistOn() })

	if err := tt.Click("映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	if got := waitBody(t, listen, "whitelist-ok"); got != "whitelist-ok" {
		t.Fatalf("traffic should pass once the whitelist is off, got %q", got)
	}
}

// TestHistoryRollbackTraffic applies twice, rolls back to the older snapshot
// from the history page, and checks the traffic follows.
func TestHistoryRollbackTraffic(t *testing.T) {
	a := testApp(t)
	upA, stopA := taggedUpstream(t, "snap-A")
	defer stopA()
	upB, stopB := taggedUpstream(t, "snap-B")
	defer stopB()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), httpRule(t, "回滚验证", listen, upA)); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	waitPort(t, listen, true)

	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	list[0].UpstreamPort = upB
	if err := a.Store.UpdateMapping(list[0]); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := waitBody(t, listen, "snap-B"); got != "snap-B" {
		t.Fatalf("the second apply should serve B, got %q", got)
	}

	snaps, err := a.Store.ListSnapshots(5)
	if err != nil || len(snaps) != 2 {
		t.Fatalf("two applies should store two snapshots: %d %v", len(snaps), err)
	}
	// The list is newest first, so the last entry is the older snapshot,
	// the one that still points at upstream A.
	older := snaps[len(snaps)-1]

	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("历史"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("回滚 " + older.ID); err != nil {
		t.Fatalf("opening the rollback: %v\nshown: %v", err, tt.Texts())
	}
	confirm(t, tt, "回滚")
	waitFor(t, tt, "the rollback announced", func() bool { return tt.HasText("已回滚") })
	if got := waitBody(t, listen, "snap-A"); got != "snap-A" {
		t.Fatalf("rolling back should serve A again, got %q", got)
	}
	if a.Dirty() {
		t.Error("a rollback applies what it restores, so nothing is left unapplied")
	}
}

// TestTemplatePickerRoundTrip lists the built-in templates, saves a custom
// one through the service the picker reads, uses it, and deletes it.
func TestTemplatePickerRoundTrip(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("从模板新建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("MySQL 数据库") {
		t.Fatalf("the built-in templates should be listed: %v", tt.Texts())
	}
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	tpl := templateFrom("e2e-svc", "模板源映射", "10.0.0.9", 8080)
	if err := a.Tpl.Save(tpl); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("从模板新建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("模板源映射") {
		t.Fatalf("the custom template should be listed: %v", tt.Texts())
	}
	// The picker scrolls: the built-in templates fill its box, so a custom
	// one added after them sits below the fold.
	reveal(t, tt, "使用 模板源映射")
	if err := tt.Click("使用 模板源映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_, _, f, _ := a.FormState()
	if f.UpstreamPort != "8080" {
		t.Errorf("using the template should prefill its port: %+v", f)
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if err := tt.Click("从模板新建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	reveal(t, tt, "删除模板 模板源映射")
	if err := tt.Click("删除模板 模板源映射"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the template gone", func() bool {
		for _, got := range a.Tpl.List() {
			if got.Name == "模板源映射" {
				return false
			}
		}
		return true
	})
}

// TestDataRoundTripUI imports a backup through the settings button and then
// wipes everything, checking the app still applies afterwards.
func TestDataRoundTripUI(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "要被替换的站")); err != nil {
		t.Fatal(err)
	}
	payload, err := a.Export()
	if err != nil {
		t.Fatal(err)
	}
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	list[0].Name = "备份里的站"
	payload.Mappings = list
	file := filepath.Join(t.TempDir(), "backup.json")
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	a.OpenDialog = func(mygo.OpenDialogOptions) ([]string, error) { return []string{file}, nil }

	tt := tester(t, a, 1200, 900)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	reveal(t, tt, "导入备份")
	if err := tt.Click("导入备份"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the import to land", func() bool {
		got, err := a.Store.ListMappings(a.ActiveWorkspace())
		return err == nil && len(got) == 1 && got[0].Name == "备份里的站"
	})

	reveal(t, tt, "清空所有数据")
	if err := tt.Click("清空所有数据"); err != nil {
		t.Fatal(err)
	}
	confirm(t, tt, "全部删除")
	waitFor(t, tt, "the data wiped", func() bool {
		got, err := a.Store.ListMappings(a.ActiveWorkspace())
		return err == nil && len(got) == 0
	})

	// The app still works afterwards.
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), streamRule(t, "清空之后")); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatalf("applying after a wipe: %v", err)
	}
}
