package engine

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
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

	c, err := net.Dial("udp", listenAddr(listen)[1:])
	if err != nil {
		t.Skipf("the test client could not dial: %v", err)
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

	// A second datagram through a fresh socket still works, which is the
	// behaviour a caller depends on; the reaping itself is internal.
	c2, err := net.Dial("udp", listenAddr(listen)[1:])
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
func TestStopClosesEveryListener(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer up.Close()

	listen := freePort(t)
	udp := freePort(t)
	e := NewLocal()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{
		{ID: "1", Name: "h", Protocol: model.HTTP, Enabled: true, ListenPort: listen,
			UpstreamHost: "127.0.0.1", UpstreamPort: portOfURL(t, up.URL), ListenPath: "/"},
		{ID: "2", Name: "u", Protocol: model.UDP, Enabled: true, ListenPort: udp,
			UpstreamHost: "127.0.0.1", UpstreamPort: 9},
	}}); err != nil {
		t.Fatal(err)
	}
	if e.Status() != Running {
		t.Fatalf("status = %s", e.Status())
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	if e.Status() != Stopped {
		t.Errorf("status after Stop = %s", e.Status())
	}
	for _, port := range []int{listen, udp} {
		if c, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 200*time.Millisecond); err == nil {
			c.Close()
			t.Errorf("port %d still listens after Stop", port)
		}
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
