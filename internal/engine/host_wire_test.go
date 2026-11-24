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

// wireRequest is what an upstream actually received on the wire.
type wireRequest struct {
	host        string // the Host header as the upstream parsed it
	auth        string
	query       string
	escapedPath string
	body        string
	done        chan struct{}
}

// hostWireUpstream starts an upstream that records the Host it was reached
// with, plus the credentials, query and body, so a test can tell a rewritten
// Host from a dropped header.
func hostWireUpstream(t *testing.T) (*httptest.Server, *wireRequest) {
	t.Helper()
	seen := wireRequest{done: make(chan struct{})}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(seen.done)
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("upstream read: %v", err)
			http.Error(w, "body read failed", http.StatusBadRequest)
			return
		}
		// 读取解析后的线上 Host，而非不会参与序列化的 Header["Host"]。
		seen.host = r.Host
		seen.auth = r.Header.Get("Authorization")
		seen.query = r.URL.RawQuery
		seen.escapedPath = r.URL.EscapedPath()
		seen.body = string(b)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(up.Close)
	return up, &seen
}

// 与 Local 的通配地址监听一致，避免 IPv4 专用端口与双栈监听冲突。
func proxyWirePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

// proxyOne applies a single HTTP mapping through the real Local engine and
// returns the recorded upstream request.
func proxyOne(t *testing.T, up *httptest.Server, seen *wireRequest, m model.Mapping, clientHost, path, query string) {
	t.Helper()
	listen := proxyWirePort(t)
	m.Protocol = model.HTTP
	m.Enabled = true
	m.ListenPort = listen
	m.UpstreamHost = "127.0.0.1"
	m.UpstreamPort = portOfURL(t, up.URL)
	if m.ListenPath == "" {
		m.ListenPath = "/"
	}
	if m.ID == "" {
		m.ID = "host-wire"
	}

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{m}}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	body := "pack-data"
	url := "http://127.0.0.1:" + itoa(listen) + path
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	// The client's Host is what nginx calls $host.
	req.Host = clientHost
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("response read: %v", err)
	}
	<-seen.done
	if seen.escapedPath != path {
		t.Errorf("escaped path = %q, want %q", seen.escapedPath, path)
	}
	if seen.auth != "Basic dXNlcjpwYXNz" {
		t.Errorf("Authorization = %q, want it passed through", seen.auth)
	}
	if seen.query != query {
		t.Errorf("RawQuery = %q, want %q", seen.query, query)
	}
	if seen.body != body {
		t.Errorf("body = %q, want %q", seen.body, body)
	}
}

// 编码路径、重复查询参数和转义字符必须在线上原样保留。
func TestStandardModePreservesEncodedPathAndRawQuery(t *testing.T) {
	up, seen := hostWireUpstream(t)
	proxyOne(t, up, seen, model.Mapping{
		Name: "编码路径", ProxyMode: model.ProxyStandard,
	}, "gitlab.example.test", "/group%2Frepo.git/git-receive-pack", "service=git-receive-pack&token=a%2Bb&x=1&x=2")
	if seen.host != "gitlab.example.test" {
		t.Errorf("upstream Host = %q", seen.host)
	}
}

// upstreamHost is the Host an upstream is reached at, as the minimal mode
// must leave it: nginx's精简上游Host, never the client's.
func upstreamHost(t *testing.T, up *httptest.Server) string {
	t.Helper()
	return strings.TrimPrefix(up.URL, "http://")
}

// TestStandardModeSendsTheClientHostOverTheWire checks the standard proxy
// mode the way nginx's default `proxy_set_header Host $host` behaves: the
// upstream must see the client's Host, not its own address.
func TestStandardModeSendsTheClientHostOverTheWire(t *testing.T) {
	up, seen := hostWireUpstream(t)
	proxyOne(t, up, seen, model.Mapping{
		Name:      "标准",
		ProxyMode: model.ProxyStandard,
	}, "gitlab.example.test", "/group/repo.git/info/refs", "service=git-receive-pack")

	if seen.host != "gitlab.example.test" {
		t.Errorf("upstream saw Host %q, want the client's Host %q", seen.host, "gitlab.example.test")
	}
}

// TestMinimalModeSendsTheUpstreamHostOverTheWire checks that the minimal mode
// keeps the upstream's own Host: it adds nothing, so the address the proxy
// dialled is what the upstream sees, and the client's Host is not smuggled in.
func TestMinimalModeSendsTheUpstreamHostOverTheWire(t *testing.T) {
	up, seen := hostWireUpstream(t)
	proxyOne(t, up, seen, model.Mapping{
		Name:      "精简",
		ProxyMode: model.ProxyMinimal,
	}, "gitlab.example.test", "/group/repo.git/info/refs", "service=git-receive-pack")

	want := upstreamHost(t, up)
	if seen.host != want {
		t.Errorf("upstream saw Host %q, want the upstream's own %q", seen.host, want)
	}
}

// TestCustomModeLiteralHostReachesTheWire checks a literal
// `proxy_set_header Host ...` line: the value the administrator wrote is what
// the upstream must see, as nginx would send it.
func TestCustomModeLiteralHostReachesTheWire(t *testing.T) {
	up, seen := hostWireUpstream(t)
	proxyOne(t, up, seen, model.Mapping{
		Name:      "自定义字面量",
		ProxyMode: model.ProxyCustom,
		ProxyHeaders: "proxy_set_header Host gitlab.example.test;\n" +
			"proxy_set_header X-Source internal;",
	}, "another.example.test", "/group/repo.git/git-receive-pack", "")

	if seen.host != "gitlab.example.test" {
		t.Errorf("upstream saw Host %q, want the directive's %q", seen.host, "gitlab.example.test")
	}
}

// TestCustomModeHostVariableReachesTheWire checks the `$host` form: it expands
// to the client's Host, and that is what the upstream must see.
func TestCustomModeHostVariableReachesTheWire(t *testing.T) {
	up, seen := hostWireUpstream(t)
	proxyOne(t, up, seen, model.Mapping{
		Name:         "自定义变量",
		ProxyMode:    model.ProxyCustom,
		ProxyHeaders: "proxy_set_header Host $host;",
	}, "gitlab.example.test", "/group/repo.git/git-receive-pack", "")

	if seen.host != "gitlab.example.test" {
		t.Errorf("upstream saw Host %q, want $host expanded to %q", seen.host, "gitlab.example.test")
	}
}

// 常见 nginx Host 变量必须完整展开；未知变量不能作为字面量发送。
func TestCustomModeAdditionalHostVariablesReachTheWire(t *testing.T) {
	for _, tc := range []struct {
		name, variable string
		clientHost     bool
	}{
		{name: "proxy_host", variable: "$proxy_host"},
		{name: "http_host", variable: "$http_host", clientHost: true},
		{name: "unknown", variable: "$server_name"},
		{name: "host_prefix_is_not_a_variable", variable: "$host_suffix"},
		{name: "proxy_host_prefix_is_not_a_variable", variable: "$proxy_host_suffix"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			up, seen := hostWireUpstream(t)
			const clientHost = "gitlab.example.test:8443"
			proxyOne(t, up, seen, model.Mapping{
				Name: "Host 变量", ProxyMode: model.ProxyCustom,
				ProxyHeaders: "proxy_set_header Host " + tc.variable + ";",
			}, clientHost, "/group/repo.git/git-receive-pack", "service=git-receive-pack")
			want := upstreamHost(t, up)
			if tc.clientHost {
				want = clientHost
			}
			if seen.host != want {
				t.Errorf("Host directive %s: upstream saw %q, want %q", tc.variable, seen.host, want)
			}
		})
	}
}

// TestCustomModeWithoutHostDirectiveKeepsTheUpstreamHost pins nginx's
// default: with no `proxy_set_header Host` line, nginx sends $proxy_host, so
// a custom mode that says nothing about Host must not rewrite it either.
func TestCustomModeWithoutHostDirectiveKeepsTheUpstreamHost(t *testing.T) {
	up, seen := hostWireUpstream(t)
	proxyOne(t, up, seen, model.Mapping{
		Name:         "自定义无Host",
		ProxyMode:    model.ProxyCustom,
		ProxyHeaders: "proxy_set_header X-Source internal;",
	}, "gitlab.example.test", "/group/repo.git/git-receive-pack", "")

	want := upstreamHost(t, up)
	if seen.host != want {
		t.Errorf("upstream saw Host %q, want the upstream's own %q", seen.host, want)
	}
}
