package appt

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/engine"
	"github.com/lianxin255/jumpboard/internal/model"
)

// TestRuleSnapshotChanged pins what the watcher counts as news: the fields
// the table draws, and nothing else. A rename or a re-listening of the
// same counters must not wake the window.
func TestRuleSnapshotChanged(t *testing.T) {
	base := engine.RuleStatus{
		MappingID: "m1", Name: "站", Listening: true,
		Connections: 3, Active: 1, BytesIn: 100, BytesOut: 200,
	}
	same := base
	if ruleSnapshotChanged([]engine.RuleStatus{base}, []engine.RuleStatus{same}) {
		t.Fatal("an unchanged snapshot is not news")
	}
	changed := []struct {
		name string
		mod  func(*engine.RuleStatus)
	}{
		{"active", func(r *engine.RuleStatus) { r.Active = 2 }},
		{"connections", func(r *engine.RuleStatus) { r.Connections = 4 }},
		{"bytes in", func(r *engine.RuleStatus) { r.BytesIn = 101 }},
		{"bytes out", func(r *engine.RuleStatus) { r.BytesOut = 201 }},
		{"listening", func(r *engine.RuleStatus) { r.Listening = false }},
		{"last close", func(r *engine.RuleStatus) { r.LastCloseAt = time.Unix(1, 0) }},
		{"last error", func(r *engine.RuleStatus) { r.LastErr, r.LastErrAt = "refused", time.Unix(1, 0) }},
		{"row error", func(r *engine.RuleStatus) { r.Err = "没有监听端口" }},
	}
	for _, tc := range changed {
		next := base
		tc.mod(&next)
		if !ruleSnapshotChanged([]engine.RuleStatus{base}, []engine.RuleStatus{next}) {
			t.Errorf("%s is news the table draws", tc.name)
		}
	}
	// A rule's identity moved — another mapping under the same slot — is
	// news too, whatever the counters say.
	other := base
	other.MappingID = "m2"
	if !ruleSnapshotChanged([]engine.RuleStatus{base}, []engine.RuleStatus{other}) {
		t.Error("a different rule under the slot is news")
	}
	// The name alone is not: it moves only on a re-apply, which asks for a
	// frame of its own.
	renamed := base
	renamed.Name = "改名"
	if ruleSnapshotChanged([]engine.RuleStatus{base}, []engine.RuleStatus{renamed}) {
		t.Error("a rename is not the watcher's news")
	}
	// A rule appearing or leaving is.
	if !ruleSnapshotChanged([]engine.RuleStatus{base}, []engine.RuleStatus{base, base}) {
		t.Error("a second row is news")
	}
}

// TestWatcherLifecycle pins the loop's lifecycle: Stop ends it, waits for
// the goroutine, and is safe to call again.
func TestWatcherLifecycle(t *testing.T) {
	w := newRuleWatcher(engine.NewLocal(), func() bool { return false }, nil)
	w.Start()
	w.Stop()
	w.Stop()
}

// TestRuleWatcherRefreshesWithoutReload walks the live path the table now
// has: a connection through a rule moves the engine's counters, and the
// watcher copies them into the app — no Reload, no apply, nothing the user
// does. The dial refuses upstream on purpose: a refusal is recorded too.
func TestRuleWatcherRefreshesWithoutReload(t *testing.T) {
	a := newTestApp(t)
	listen := freePort(t)
	if _, err := a.Eng.Apply(engine.Spec{Mappings: []model.Mapping{{
		ID: "m1", Name: "探针", Protocol: model.TCP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}}}); err != nil {
		t.Fatal(err)
	}

	// Before anything: the snapshot the frame reads is what the apply left.
	if got := a.RuleInfo("m1").Connections; got != 0 {
		t.Fatalf("a fresh rule has served nobody, got %d", got)
	}

	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listen), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	// The watcher's fallback is a second, so a few of them are plenty.
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		info := a.RuleInfo("m1")
		if info.Connections >= 1 && info.LastErr != "" && !info.LastCloseAt.IsZero() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the watcher never carried the counters into the app: %+v", a.RuleInfo("m1"))
}

// freePort asks the kernel for a port nothing holds.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
