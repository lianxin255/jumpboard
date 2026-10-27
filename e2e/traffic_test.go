package e2e

import (
	"fmt"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestTrafficCounterShowsInTheTable applies a rule, pushes bytes through
// it, and checks the mappings table reports them humanized.
func TestTrafficCounterShowsInTheTable(t *testing.T) {
	a := testApp(t)
	upPort, stopUp := upstream(t)
	defer stopUp()
	listen := freePort(t)

	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "流量站", Protocol: model.HTTP, Enabled: true,
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
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the mapping", func() bool { return tt.HasText("流量站") })

	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the engine running", func() bool { return a.Eng.Status() == "running" })
	waitPort(t, listen, true)

	// A GET has no request body, so the rule counts the response's eleven
	// bytes as traffic out and none in — a value the table must show as it
	// stands, not as a raw count.
	if body, err := httpGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); err != nil || body != "upstream-ok" {
		t.Fatalf("request through the proxy: %q %v", body, err)
	}

	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the traffic line", func() bool { return tt.HasText("流量 ↓0 B ↑11 B") })
}
