package engine

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// freePort returns a port nothing listens on right now.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// upstream starts an HTTP server that answers with its own name.
func upstream(t *testing.T, name string) (int, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, name)
	})
	mux.HandleFunc("/path", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, name+" path="+r.URL.Path)
	})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(l)
	return l.Addr().(*net.TCPAddr).Port, func() { _ = srv.Close() }
}

// doGet fetches a URL without the machine's proxy settings.
func doGet(t *testing.T, url string) (int, string) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// echoUpstream answers with the request's body, so a test knows exactly
// which bytes came back.
func echoUpstream(t *testing.T) (int, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the whole body first: a copy that writes as it reads can be
		// cut short when the client stops reading the response.
		body, _ := io.ReadAll(r.Body)
		_, _ = w.Write(body)
	})}
	go srv.Serve(l)
	return l.Addr().(*net.TCPAddr).Port, func() { _ = srv.Close() }
}

// TestHTTPProxy applies one HTTP mapping and requests through it.
func TestHTTPProxy(t *testing.T) {
	up, stop := upstream(t, "upstream-ok")
	defer stop()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	res, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "站点", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: up,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.OK {
		t.Fatalf("apply result: %+v", res)
	}

	status, body := doGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen))
	if status != 200 || body != "upstream-ok" {
		t.Fatalf("proxy returned %d %q", status, body)
	}
	if e.Status() != Running {
		t.Errorf("the engine should be running: %s", e.Status())
	}

	rules := e.Rules()
	if len(rules) != 1 || !rules[0].Listening {
		t.Fatalf("the rule should be listening: %+v", rules)
	}
	if rules[0].Connections == 0 {
		t.Errorf("the rule should have counted a connection: %+v", rules[0])
	}
}

// TestHTTPHostAndPathRouting checks that server_name and location route the
// way the generated configuration did.
func TestHTTPHostAndPathRouting(t *testing.T) {
	a, stopA := upstream(t, "A")
	defer stopA()
	b, stopB := upstream(t, "B")
	defer stopB()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{
		{ID: "1", Name: "A", Protocol: model.HTTP, Enabled: true, ListenPort: listen,
			ServerNames: "a.example.com", ListenPath: "/", UpstreamHost: "127.0.0.1", UpstreamPort: a},
		{ID: "2", Name: "B", Protocol: model.HTTP, Enabled: true, ListenPort: listen,
			ServerNames: "b.example.com", ListenPath: "/api", UpstreamHost: "127.0.0.1", UpstreamPort: b},
	}}); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	get := func(host, path string) (int, string) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", listen, path), nil)
		req.Host = host
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s%s: %v", host, path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	if _, body := get("a.example.com", "/"); body != "A" {
		t.Errorf("a.example.com should reach A: %q", body)
	}
	// proxy_pass without a URI sends the request path through unchanged,
	// as nginx does for a location that is not the root.
	if _, body := get("b.example.com", "/api/path"); body != "B" {
		t.Errorf("b.example.com/api should reach B with its path: %q", body)
	}
	// A host with no rule of its own falls to the port's default server,
	// which is the first rule without a server_name, as nginx's default
	// server does.
	if _, body := get("c.example.com", "/"); body != "A" {
		t.Errorf("an unknown host should fall to the default server: %q", body)
	}
	// A path the second rule does not cover is answered by the default
	// server rather than with a 404: / is the first rule's location.
	if _, body := get("b.example.com", "/other"); body != "A" {
		t.Errorf("an uncovered path should fall to the default: %q", body)
	}
}

// TestTCPForward pipes bytes through a TCP rule.
func TestTCPForward(t *testing.T) {
	// An upstream that echoes what it reads.
	up, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	go func() {
		for {
			c, err := up.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "回显", Protocol: model.TCP, Enabled: true,
		ListenPort:   listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: up.Addr().(*net.TCPAddr).Port,
	}}}); err != nil {
		t.Fatal(err)
	}

	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listen), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("echo = %q", buf)
	}
}

// TestTCPCountsBytes checks the rule's byte counters: what the clients sent
// counts as traffic in, what came back as out, and two connections open at
// once both count.
func TestTCPCountsBytes(t *testing.T) {
	up := tcpEchoServer(t)
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "回显", Protocol: model.TCP, Enabled: true,
		ListenPort:   listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: up,
	}}}); err != nil {
		t.Fatal(err)
	}

	// Both connections stay open while they talk, so their pipes run at
	// the same time and add to the same counters.
	var conns []net.Conn
	for range 2 {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", listen), 2*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, c)
	}
	payload := []byte("0123456789")
	for _, c := range conns {
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := c.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range conns {
		buf := make([]byte, len(payload))
		if _, err := io.ReadFull(c, buf); err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(buf) != string(payload) {
			t.Errorf("echo = %q", buf)
		}
		_ = c.Close()
	}

	rules := e.Rules()
	if len(rules) != 1 {
		t.Fatalf("one rule: %+v", rules)
	}
	want := int64(2 * len(payload))
	if rules[0].BytesIn < want || rules[0].BytesOut < want {
		t.Errorf("bytes = ↓%d ↑%d, want at least ↓%d ↑%d",
			rules[0].BytesIn, rules[0].BytesOut, want, want)
	}
	if rules[0].Connections < 2 {
		t.Errorf("both connections should count: %+v", rules[0])
	}
}

// TestHTTPCountsBytes checks the proxy's byte counters: the request body
// counts as traffic in, the response body as out.
func TestHTTPCountsBytes(t *testing.T) {
	up, stop := echoUpstream(t)
	defer stop()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "回显站", Protocol: model.HTTP, Enabled: true,
		ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: up,
		ListenPath: "/", ProxyMode: model.ProxyStandard,
	}}}); err != nil {
		t.Fatal(err)
	}

	body := strings.Repeat("x", 4096)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/", listen), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST through the proxy: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != body {
		t.Fatalf("echo = %d bytes, want %d", len(b), len(body))
	}

	rules := e.Rules()
	if len(rules) != 1 {
		t.Fatalf("one rule: %+v", rules)
	}
	if rules[0].BytesIn < int64(len(body)) || rules[0].BytesOut < int64(len(body)) {
		t.Errorf("bytes = ↓%d ↑%d, want at least ↓%d ↑%d",
			rules[0].BytesIn, rules[0].BytesOut, len(body), len(body))
	}
}

// TestUDPForward sends a datagram through a UDP rule.
func TestUDPForward(t *testing.T) {
	up, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := up.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = up.WriteTo(append([]byte("echo:"), buf[:n]...), addr)
		}
	}()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "DNS", Protocol: model.UDP, Enabled: true,
		ListenPort:   listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: up.LocalAddr().(*net.UDPAddr).Port,
	}}}); err != nil {
		t.Fatal(err)
	}

	c, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", listen))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1500)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "echo:ping" {
		t.Errorf("reply = %q", got)
	}
}

// TestTLSProxy terminates TLS, picking the certificate by server name.
func TestTLSProxy(t *testing.T) {
	up, stop := upstream(t, "tls-ok")
	defer stop()
	listen := freePort(t)
	cert, key := testCert(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "TLS", Protocol: model.HTTPS, Enabled: true,
		ListenPort: listen, ServerNames: "secure.example.com",
		UpstreamHost: "127.0.0.1", UpstreamPort: up,
		TLSCertPath: cert, TLSKeyPath: key, ListenPath: "/",
	}}}); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: &http.Transport{
		Proxy:             nil,
		TLSClientConfig:   insecureTLS(),
		ForceAttemptHTTP2: false,
	}}
	resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/", listen))
	if err != nil {
		t.Fatalf("https request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "tls-ok" {
		t.Errorf("body = %q", b)
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		t.Error("the response should have been over TLS")
	}
}

// TestWhitelistBlocks checks the address filter.
func TestWhitelistBlocks(t *testing.T) {
	up, stop := upstream(t, "ok")
	defer stop()
	listen := freePort(t)

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{
		Mappings: []model.Mapping{{
			ID: "1", Name: "受限", Protocol: model.HTTP, Enabled: true,
			ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: up,
			ListenPath: "/",
		}},
		WhitelistIPs: []string{"10.0.0.0/8"}, // not the loopback
	}); err != nil {
		t.Fatal(err)
	}
	if status, _ := doGet(t, fmt.Sprintf("http://127.0.0.1:%d/", listen)); status != http.StatusForbidden {
		t.Errorf("a blocked address should get 403, got %d", status)
	}
}

// TestApplyIsTransactional checks that a port that cannot be bound leaves
// the running rules alone.
func TestApplyIsTransactional(t *testing.T) {
	up, stop := upstream(t, "ok")
	defer stop()
	good := freePort(t)

	// A port already taken by someone else.
	taken, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	busy := taken.Addr().(*net.TCPAddr).Port

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "好", Protocol: model.HTTP, Enabled: true, ListenPort: good,
		UpstreamHost: "127.0.0.1", UpstreamPort: up, ListenPath: "/",
	}}}); err != nil {
		t.Fatal(err)
	}
	if status, _ := doGet(t, fmt.Sprintf("http://127.0.0.1:%d/", good)); status != 200 {
		t.Fatalf("the first rule should serve: %d", status)
	}

	// A second mapping on a port that is taken fails the whole apply.
	_, err = e.Apply(Spec{Mappings: []model.Mapping{
		{ID: "1", Name: "好", Protocol: model.HTTP, Enabled: true, ListenPort: good,
			UpstreamHost: "127.0.0.1", UpstreamPort: up, ListenPath: "/"},
		{ID: "2", Name: "坏", Protocol: model.TCP, Enabled: true, ListenPort: busy,
			UpstreamHost: "127.0.0.1", UpstreamPort: 1},
	}})
	if err == nil {
		t.Fatal("applying onto a taken port should fail")
	}
	if !strings.Contains(err.Error(), "无法监听") {
		t.Errorf("the error should name the port: %v", err)
	}
	// The first rule still serves traffic.
	if status, _ := doGet(t, fmt.Sprintf("http://127.0.0.1:%d/", good)); status != 200 {
		t.Errorf("the previous rule should still serve after a failed apply: %d", status)
	}
}

// TestReapplyKeepsUnchangedRules checks that applying the same spec twice
// does not bounce listeners, which would drop live connections.
func TestReapplyKeepsUnchangedRules(t *testing.T) {
	up, stop := upstream(t, "ok")
	defer stop()
	listen := freePort(t)
	spec := Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "稳定", Protocol: model.TCP, Enabled: true, ListenPort: listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: up,
	}}}

	e := NewLocal()
	defer e.Stop()
	if _, err := e.Apply(spec); err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", listen))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	res, err := e.Apply(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Started) != 0 || len(res.Stopped) != 0 {
		t.Errorf("an unchanged rule should not restart: %+v", res)
	}
	// The open connection still works.
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write([]byte("x")); err != nil {
		t.Errorf("the live connection was dropped: %v", err)
	}
}

// TestTimeoutParsing checks the nginx timeout syntax.
func TestTimeoutParsing(t *testing.T) {
	for _, c := range []struct {
		extra string
		want  time.Duration
	}{
		{"proxy_timeout 1h;", time.Hour},
		{"proxy_timeout 30m;", 30 * time.Minute},
		{"proxy_timeout 30;", 30 * time.Second},
		{"proxy_read_timeout 500ms;", 500 * time.Millisecond},
		{"", 0},
		{"proxy_buffering off;", 0},
	} {
		if got := parseTimeout(c.extra); got != c.want {
			t.Errorf("parseTimeout(%q) = %v, want %v", c.extra, got, c.want)
		}
	}
}

// TestUnsupportedDirectivesAreReported checks that a directive the engine
// cannot honor is named rather than dropped silently.
func TestUnsupportedDirectivesAreReported(t *testing.T) {
	e := NewLocal()
	defer e.Stop()
	res, err := e.Apply(Spec{Mappings: []model.Mapping{{
		ID: "1", Name: "x", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		ExtraDirectives: "proxy_timeout 1h;\ngzip on;",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "gzip") {
			found = true
		}
	}
	if !found {
		t.Errorf("gzip should be reported as ignored: %+v", res.Warnings)
	}
}

// testCert writes a self-signed certificate for 127.0.0.1 and returns the
// paths.
func testCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	certPath = dir + "/cert.pem"
	keyPath = dir + "/key.pem"
	if err := writeTestCert(certPath, keyPath); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

// ensure httptest stays imported for the helpers below.
var _ = httptest.NewServer
