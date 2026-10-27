package engine

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestRuleTracksLastCloseAndError pins what a TCP rule remembers of a
// connection: when the latest one ended, and — when the upstream refused
// the dial — what went wrong and when. A rule whose upstream answers keeps
// no error.
func TestRuleTracksLastCloseAndError(t *testing.T) {
	dead := freePort(t) // nothing listens here
	echo := tcpEchoServer(t)
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{
		{
			ID: "dead", Name: "坏上游", Protocol: model.TCP, Enabled: true,
			ListenPort: dead, UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		},
		{
			ID: "echo", Name: "回显", Protocol: model.TCP, Enabled: true,
			ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: echo,
		},
	}}); err != nil {
		t.Fatal(err)
	}

	before := time.Now()
	// One client to the dead upstream: the dial fails, which is the rule's
	// error to keep.
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", dead), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	// One to the echo upstream: it talks and ends cleanly.
	c2, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listen), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c2.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := c2.Write([]byte("bye")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 3)
	if _, err := io.ReadFull(c2, buf); err != nil {
		t.Fatal(err)
	}
	_ = c2.Close()

	rules := waitRulesSettled(t, e)
	byID := map[string]RuleStatus{}
	for _, r := range rules {
		byID[r.MappingID] = r
	}
	bad, good := byID["dead"], byID["echo"]
	if bad.LastErr == "" || bad.LastErrAt.Before(before) {
		t.Errorf("the refused dial should be the rule's last error: %+v", bad)
	}
	if bad.LastCloseAt.Before(before) {
		t.Errorf("the failed connection still ended: %+v", bad)
	}
	if good.LastErr != "" {
		t.Errorf("a rule whose upstream answers keeps no error: %+v", good)
	}
	if good.LastCloseAt.Before(before) {
		t.Errorf("the clean connection's end should be remembered: %+v", good)
	}
}

// TestHTTPRuleTracksLastError pins the HTTP side: a request the upstream
// refuses becomes the rule's last error, and one it answers is only the
// last close.
func TestHTTPRuleTracksLastError(t *testing.T) {
	deadUp := freePort(t)
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "h1", Name: "站点", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: deadUp,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}}}); err != nil {
		t.Fatal(err)
	}

	before := time.Now()
	// The request goes through the rule and dies at the upstream: the 502
	// is the rule's news to keep.
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", listen))
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusBadGateway {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the proxy never answered 502: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	rules := waitRulesSettled(t, e)
	if len(rules) != 1 {
		t.Fatalf("one rule: %+v", rules)
	}
	r := rules[0]
	if r.LastErr == "" || r.LastErrAt.Before(before) {
		t.Errorf("the refused request should be the rule's last error: %+v", r)
	}
	if r.LastCloseAt.Before(before) {
		t.Errorf("the failed request still ended: %+v", r)
	}
	if r.Connections < 1 {
		t.Errorf("the request should count: %+v", r)
	}
}

// TestEventsFireOnTraffic pins the wake-up: traffic through a rule pokes
// the engine's events, coalesced, so a watcher can reread at once. An idle
// engine must not poke.
func TestEventsFireOnTraffic(t *testing.T) {
	echo := tcpEchoServer(t)
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "回显", Protocol: model.TCP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: echo,
	}}}); err != nil {
		t.Fatal(err)
	}

	// Drain whatever the apply itself left, so the wait below is about the
	// connection only.
	for {
		select {
		case <-e.Events():
			continue
		default:
		}
		break
	}

	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listen), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	select {
	case <-e.Events():
	case <-time.After(3 * time.Second):
		t.Fatal("traffic through the rule should poke the events")
	}
}

// waitRulesSettled rereads the rules until the counters stop moving: a
// pipe's end is recorded on its goroutine, which a single read can beat.
func waitRulesSettled(t *testing.T, e *Local) []RuleStatus {
	t.Helper()
	var last []RuleStatus
	for range 50 {
		rules := e.Rules()
		if sameRules(last, rules) {
			return rules
		}
		last = rules
		time.Sleep(20 * time.Millisecond)
	}
	return last
}

func sameRules(a, b []RuleStatus) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		switch {
		case x.Connections != y.Connections,
			x.Active != y.Active,
			x.BytesIn != y.BytesIn,
			x.BytesOut != y.BytesOut,
			x.LastCloseAt != y.LastCloseAt,
			x.LastErrAt != y.LastErrAt,
			x.LastErr != y.LastErr:
			return false
		}
	}
	return true
}
