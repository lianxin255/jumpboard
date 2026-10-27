package engine

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestUDPIdleSessionsAreReaped checks that a UDP rule does not keep a
// client's socket for ever.
func TestUDPIdleSessionsAreReaped(t *testing.T) {
	up, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := up.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = up.WriteTo(buf[:n], addr)
		}
	}()

	listen := freePort(t)
	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{
		Mappings: []model.Mapping{{
			ID: "1", Name: "u", Protocol: model.UDP, Enabled: true,
			ListenPort: listen, UpstreamHost: "127.0.0.1",
			UpstreamPort: up.LocalAddr().(*net.UDPAddr).Port,
			// A short timeout, so the reaper runs during the test.
			ExtraDirectives: "proxy_timeout 1s;",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	c, err := net.Dial("udp", "127.0.0.1:"+itoa(listen))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8)
	if _, err := c.Read(buf); err != nil {
		t.Fatalf("first datagram: %v", err)
	}
	c.Close()

	// The echo came back, so the client address holds a live session: the
	// rule's active count is that session, and the short idle has not
	// expired yet.
	waitActive(t, e, "1", 1, 2*time.Second)

	// A second datagram through a fresh socket still works, which is the
	// behaviour a caller depends on.
	c2, err := net.Dial("udp", "127.0.0.1:"+itoa(listen))
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	_ = c2.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c2.Write([]byte("b")); err != nil {
		t.Fatal(err)
	}
	if _, err := c2.Read(buf); err != nil {
		t.Fatalf("second datagram: %v", err)
	}

	// Both sessions have gone quiet, so after the one-second idle the rule
	// reaps them and holds no active session — which is what "reaped" means
	// where it shows, in the counter the table reads.
	waitActive(t, e, "1", 0, 4*time.Second)
}

// TestProxyPassURLIsUsed checks that a mapping with its own proxy_pass
// target goes there, path and all.
func TestProxyPassURLIsUsed(t *testing.T) {
	var sawPath string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		_, _ = io.WriteString(w, "custom")
	}))
	defer up.Close()

	listen := freePort(t)
	e := NewLocal()
	defer e.Stop()

	// proxy_pass with a path replaces the location prefix, as nginx's
	// trailing-slash form does.
	target := "http://" + strings.TrimPrefix(up.URL, "http://") + "/backend/"
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "自定义", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "ignored", UpstreamPort: 1,
		ListenPath: "/api", ProxyPassURL: target, ProxyMode: model.ProxyStandard,
	}}}); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Get("http://127.0.0.1:" + itoa(listen) + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "custom" {
		t.Errorf("body = %q", b)
	}
	if sawPath != "/backend/api/users" {
		t.Errorf("the upstream saw %q, want /backend/api/users", sawPath)
	}
}

// TestCustomHeadersAreApplied checks the custom proxy mode's headers.
func TestCustomHeadersAreApplied(t *testing.T) {
	var got http.Header
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, "ok")
	}))
	defer up.Close()

	listen := freePort(t)
	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "自定义头", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: portOfURL(t, up.URL),
		ListenPath: "/", ProxyMode: model.ProxyCustom,
		ProxyHeaders: "proxy_set_header X-Source internal;\nproxy_set_header X-Host $host;",
	}}}); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+itoa(listen)+"/", nil)
	req.Host = "custom.example.com"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got.Get("X-Source") != "internal" {
		t.Errorf("X-Source = %q", got.Get("X-Source"))
	}
	// $host is the client's Host, as nginx expands it.
	if got.Get("X-Host") != "custom.example.com" {
		t.Errorf("X-Host = %q", got.Get("X-Host"))
	}
	// The custom mode writes its own headers, so no X-Real-IP is added.
	if got.Get("X-Real-Ip") != "" {
		t.Errorf("the custom mode should not add X-Real-IP: %v", got)
	}
}

// TestWhitelistAcceptsAListedAddress checks the allow side.
func TestWhitelistAcceptsAListedAddress(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer up.Close()

	listen := freePort(t)
	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{
		Mappings: []model.Mapping{{
			ID: "1", Name: "放行", Protocol: model.HTTP, Enabled: true,
			ListenPort: listen, UpstreamHost: "127.0.0.1",
			UpstreamPort: portOfURL(t, up.URL), ListenPath: "/",
		}},
		WhitelistIPs: []string{"127.0.0.0/8", "10.0.0.1"},
	}); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Get("http://127.0.0.1:" + itoa(listen) + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("a listed address should pass: %d", resp.StatusCode)
	}
}

// TestStopClosesEveryListener checks that stopping frees every port.
//
// The checks are on the resources, not on dial success. A UDP rule never
// occupies the TCP namespace of its port, so a TCP dial to it after Stop
// is answered whenever any unrelated process — another package's
// httptest server running in the same `go test ./...`, or a background
// service on a shared runner — happens to hold that TCP port, both
// drawing from the same ephemeral range. Such a dial reported "still
// listens" about a port the engine never touched. The HTTP port has the
// milder form of the same flaw: anyone rebinding it in the window after
// Close answers the dial. So instead the HTTP port must be bindable
// again, and the UDP port must no longer answer an echo through its
// upstream — which only holds when the forwarder's socket is gone.
func TestStopClosesEveryListener(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer up.Close()

	// The UDP rule points at a local echo server, so the forwarder's
	// health can be probed before the Stop.
	ue, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ue.Close()
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := ue.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = ue.WriteTo(buf[:n], addr)
		}
	}()

	listen := freePort(t)
	udp := freePort(t)
	e := NewLocal()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{
		{ID: "1", Name: "h", Protocol: model.HTTP, Enabled: true, ListenPort: listen,
			UpstreamHost: "127.0.0.1", UpstreamPort: portOfURL(t, up.URL), ListenPath: "/"},
		{ID: "2", Name: "u", Protocol: model.UDP, Enabled: true, ListenPort: udp,
			UpstreamHost: "127.0.0.1", UpstreamPort: ue.LocalAddr().(*net.UDPAddr).Port},
	}}); err != nil {
		t.Fatal(err)
	}
	if e.Status() != Running {
		t.Fatalf("status = %s", e.Status())
	}

	// Before the Stop, a datagram through the UDP rule comes back as an
	// echo, proving the test really did have a live forwarder.
	uc, err := net.Dial("udp", "127.0.0.1:"+itoa(udp))
	if err != nil {
		t.Fatalf("the probe could not dial the UDP rule: %v", err)
	}
	_ = uc.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := uc.Write([]byte("stop")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	if _, err := uc.Read(buf); err != nil {
		t.Fatalf("the UDP rule did not echo before Stop: %v", err)
	}
	uc.Close()

	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	if e.Status() != Stopped {
		t.Errorf("status after Stop = %s", e.Status())
	}

	// The HTTP port is free only when it can be bound again, the same
	// address form the engine itself binds.
	ln, err := net.Listen("tcp", listenAddr(listen))
	if err != nil {
		t.Fatalf("HTTP port %d not free after Stop: %v", listen, err)
	}
	ln.Close()

	// The UDP port no longer forwards: a datagram comes back with no
	// echo, only a timeout or an ICMP port-unreachable.
	uc2, err := net.Dial("udp", "127.0.0.1:"+itoa(udp))
	if err != nil {
		t.Fatal(err)
	}
	defer uc2.Close()
	_ = uc2.SetDeadline(time.Now().Add(time.Second))
	if _, err := uc2.Write([]byte("gone")); err != nil {
		t.Fatal(err)
	}
	_ = uc2.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if n, err := uc2.Read(buf); err == nil {
		t.Errorf("UDP port %d still forwards after Stop (got %q)", udp, buf[:n])
	}
}

// portOfURL returns the port of a test server's URL.
func portOfURL(t *testing.T, raw string) int {
	t.Helper()
	u := strings.TrimPrefix(raw, "http://")
	if i := strings.LastIndex(u, ":"); i >= 0 {
		return atoiOr(u[i+1:], 0)
	}
	return 80
}
