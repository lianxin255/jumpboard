package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// TestProxyHeadersAndStreaming checks two behaviors a proxy must get right:
// the request headers it adds, and a streamed response that arrives before
// the handler returns (a server-sent event, a git push's progress).
func TestProxyHeadersAndStreaming(t *testing.T) {
	var gotHeaders http.Header
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			fl := w.(http.Flusher)
			for i := 0; i < 3; i++ {
				fmt.Fprintf(w, "data: %d\n\n", i)
				fl.Flush()
			}
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer up.Close()
	upPort := portOf(t, up.URL)
	listen := freePort(t)

	a := testApp(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "头与流", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, ServerNames: "h.example.com",
		UpstreamHost: "127.0.0.1", UpstreamPort: upPort,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	defer a.StopEngine()
	waitPort(t, listen, true)

	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", listen), nil)
	req.Host = "h.example.com"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// A standard proxy names itself to the upstream.
	for _, h := range []string{"X-Real-Ip", "X-Forwarded-For", "X-Forwarded-Proto"} {
		if gotHeaders.Get(h) == "" {
			t.Errorf("the upstream should see %s: %v", h, gotHeaders)
		}
	}
	if got := gotHeaders.Get("X-Forwarded-Proto"); got != "http" {
		t.Errorf("X-Forwarded-Proto = %q", got)
	}

	// A streamed body arrives in parts, not all at once at the end.
	resp, err = client.Get(fmt.Sprintf("http://127.0.0.1:%d/stream", listen))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "data: 0") || !strings.Contains(string(b), "data: 2") {
		t.Errorf("the stream should carry every event: %q", b)
	}
}

// TestMinimalModeAddsNoHeaders checks that the minimal proxy mode leaves
// the request as the client sent it.
func TestMinimalModeAddsNoHeaders(t *testing.T) {
	var got http.Header
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, "ok")
	}))
	defer up.Close()
	listen := freePort(t)

	a := testApp(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "精简", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: portOf(t, up.URL),
		ListenPath: "/", ProxyMode: model.ProxyMinimal,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	defer a.StopEngine()
	waitPort(t, listen, true)

	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", listen), nil)
	req.Header.Set("X-Client", "kept")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got.Get("X-Real-Ip") != "" || got.Get("X-Forwarded-For") != "" {
		t.Errorf("the minimal mode should add no headers: %v", got)
	}
	if got.Get("X-Client") != "kept" {
		t.Errorf("the client's own header should pass through: %v", got)
	}
}

// portOf returns the port of a test server's URL.
func portOf(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(u.Port(), "%d", &port); err != nil {
		t.Fatalf("port of %s: %v", raw, err)
	}
	return port
}
