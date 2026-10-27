package e2e

import (
	"fmt"
	"testing"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
)

// applyOneRule stands up one HTTP mapping end to end — created, applied,
// and answering — so a test can get on with what it actually measures.
func applyOneRule(t *testing.T, a *appt.App, name string) (listen int) {
	t.Helper()
	upPort, stopUp := upstream(t)
	t.Cleanup(stopUp)
	listen = freePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: name, Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}); err != nil {
		t.Fatal(err)
	}
	// A direct store write does not refresh the app's snapshot, so the page
	// would not see the row until the next reload.
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	return listen
}

// TestTrafficUpdatesWithoutReload pushes one request through an applied
// rule and expects the table to report its counters with no Reload, no
// click, nothing but the watcher's own refresh — the whole point of live
// traffic visibility.
func TestTrafficUpdatesWithoutReload(t *testing.T) {
	a := testApp(t)
	listen := applyOneRule(t, a, "流量直播")

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the mapping", func() bool { return tt.HasText("流量直播") })
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the engine running", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)

	if _, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil {
		t.Fatal(err)
	}
	// No Reload here: the watcher's event has to land the counts, and the
	// frame waitFor asks for has to draw them.
	waitFor(t, tt, "the connection count live", func() bool { return tt.HasText("· 1 次") })
	waitFor(t, tt, "the byte count live", func() bool { return tt.HasText("流量 ↓0 B ↑11 B") })
}

// TestShowTrafficSetting walks the setting as a user does: the switch on
// the settings page quiets the table's counters, and back on the mappings
// page the counts are gone — then the switch returns them.
func TestShowTrafficSetting(t *testing.T) {
	a := testApp(t)
	listen := applyOneRule(t, a, "流量开关")

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the mapping", func() bool { return tt.HasText("流量开关") })
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the engine running", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)
	if _, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the counters to land", func() bool { return tt.HasText("流量 ↓0 B ↑11 B") })

	// Off: the switch sits on the settings page, which scrolls.
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickScrolled(t, tt, "显示流量")
	if a.ShowTraffic() {
		t.Fatal("the switch should have turned the display off")
	}

	// The quiet holds on the mappings page, where the counters lived.
	if err := tt.Click("映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("流量 ↓0 B ↑11 B") || tt.HasText("· 1 次") {
		t.Fatalf("the counters should be gone with the display off: %v", tt.Texts())
	}
	if !tt.HasText("流量开关") {
		t.Fatalf("the rule itself should still be listed: %v", tt.Texts())
	}

	// And back on, the counts return without another request.
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickScrolled(t, tt, "显示流量")
	if err := tt.Click("映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("流量 ↓0 B ↑11 B") {
		t.Fatalf("the traffic line should be back: %v", tt.Texts())
	}
}
