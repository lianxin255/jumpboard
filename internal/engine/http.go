package engine

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// HTTP and TLS reverse proxies: one server per listening port, routing by
// host and by path prefix, the way the generated configuration did.

// httpForwarder is a listening HTTP or HTTPS server.
type httpForwarder struct {
	srv     *http.Server
	ln      net.Listener
	port    int
	proto   model.Protocol
	idle    time.Duration
	conns   atomic.Int64
	active  atomic.Int32
	closing atomic.Bool
	started time.Time
}

func (f *httpForwarder) key() ruleKey {
	return ruleKey{proto: f.proto, listen: listenAddr(f.port)}
}

func (f *httpForwarder) stats() (int64, int) {
	return f.conns.Load(), int(f.active.Load())
}

func (f *httpForwarder) Close() error {
	if f.closing.Swap(true) {
		return nil
	}
	// Shutdown waits for in-flight requests, which a reload should not
	// block on: the timeout bounds it.
	return f.srv.Close()
}

// route is one mapping's place in a port's routing table.
type route struct {
	mapping     model.Mapping
	upstream    *url.URL
	proxy       *httputil.ReverseProxy
	serverNames []string
	path        string
}

// newHTTPForwarder starts a server for one port.
func newHTTPForwarder(port int, proto model.Protocol, routes []route, spec Spec, access *accessLog) (*httpForwarder, error) {
	addr := listenAddr(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	if proto == model.HTTPS {
		tlsConfig, err := tlsConfigFor(routes)
		if err != nil {
			ln.Close()
			return nil, err
		}
		ln = tls.NewListener(ln, tlsConfig)
	}

	f := &httpForwarder{port: port, proto: proto, idle: spec.IdleTimeout, ln: ln, started: time.Now()}
	if f.idle == 0 {
		f.idle = 5 * time.Minute
	}

	handler := &router{routes: routes, whitelist: parseWhitelist(spec.WhitelistIPs), log: access, host: f}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       f.idle,
		// No write timeout: a proxied download or a streamed response has
		// no bounded length, and the idle timeout covers a dead peer.
		ConnState: func(c net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				f.conns.Add(1)
				f.active.Add(1)
			case http.StateClosed, http.StateHijacked:
				f.active.Add(-1)
			}
		},
		// A GUI has no stderr to read, and the probe dials a health
		// checker and a port scanner make are noise here; the access log
		// is the record that matters.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	f.srv = srv
	go func() {
		_ = srv.Serve(ln)
	}()
	return f, nil
}

// tlsConfigFor builds a configuration that picks the certificate by the
// server name the client asked for.
func tlsConfigFor(routes []route) (*tls.Config, error) {
	certs := map[string]*tls.Certificate{}
	var first *tls.Certificate
	for _, r := range routes {
		m := r.mapping
		if m.TLSCertPath == "" || m.TLSKeyPath == "" {
			continue
		}
		cert, err := tls.LoadX509KeyPair(m.TLSCertPath, m.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取证书 %s: %w", m.TLSCertPath, err)
		}
		c := cert
		names := splitNames(m.ServerNames)
		if len(names) == 0 {
			// A rule with no server name takes every name on the port: it
			// is the default certificate.
			if first == nil {
				first = &c
			}
			continue
		}
		for _, n := range names {
			if _, taken := certs[n]; !taken {
				certs[n] = &c
			}
		}
		// The first certificate also answers names nothing matched, as
		// nginx's default server does.
		if first == nil {
			first = &c
		}
	}
	if first == nil && len(certs) == 0 {
		return nil, fmt.Errorf("HTTPS 端口需要至少一个可用的证书")
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := strings.ToLower(strings.TrimSuffix(hello.ServerName, "."))
			if c, ok := certs[name]; ok {
				return c, nil
			}
			// A wildcard in the configuration is matched the way a client
			// expects: the longest suffix wins.
			best, rest := "", name
			for configured := range certs {
				if strings.HasPrefix(configured, "*.") && strings.HasSuffix(name, configured[1:]) {
					if len(configured) > len(best) {
						best, rest = configured, configured
					}
				}
			}
			if best != "" {
				return certs[rest], nil
			}
			if first != nil {
				return first, nil
			}
			return nil, fmt.Errorf("没有匹配 %q 的证书", name)
		},
	}, nil
}

// splitNames turns a server_name value into its names.
func splitNames(s string) []string {
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSuffix(f, "."))
		if f != "" && f != "_" {
			out = append(out, f)
		}
	}
	return out
}

// router picks the route for a request and proxies it.
type router struct {
	routes    []route
	whitelist []*net.IPNet
	log       *accessLog
	host      *httpForwarder
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowed(rt.whitelist, r.RemoteAddr) {
		http.Error(w, "403 forbidden", http.StatusForbidden)
		rt.log.write(rt.host.port, r, 403, 0)
		return
	}

	host := strings.ToLower(strings.TrimSuffix(stripPort(r.Host), "."))
	best := -1
	for i := range rt.routes {
		rte := &rt.routes[i]
		if !hostMatches(rte.serverNames, host) {
			continue
		}
		if !pathMatches(rte.path, r.URL.Path) {
			continue
		}
		// The longest path wins, as nginx's prefix locations do.
		if best < 0 || len(rte.path) > len(rt.routes[best].path) {
			best = i
		}
	}
	if best < 0 {
		// A host no rule names falls to the port's default server: the
		// first rule with no server_name, or the first rule at all, as
		// nginx's default server does.
		for i := range rt.routes {
			if len(rt.routes[i].serverNames) == 0 && pathMatches(rt.routes[i].path, r.URL.Path) {
				best = i
				break
			}
		}
	}
	if best < 0 {
		for i := range rt.routes {
			if pathMatches(rt.routes[i].path, r.URL.Path) {
				best = i
				break
			}
		}
	}
	if best < 0 {
		http.NotFound(w, r)
		rt.log.write(rt.host.port, r, 404, 0)
		return
	}

	rte := &rt.routes[best]
	rec := &statusRecorder{ResponseWriter: w, status: 200}
	rte.proxy.ServeHTTP(rec, r)
	rt.log.write(rt.host.port, r, rec.status, rec.bytes)
}

// hostMatches reports whether a route takes this host.
func hostMatches(names []string, host string) bool {
	if len(names) == 0 {
		return true // the default server of the port
	}
	for _, n := range names {
		if n == host {
			return true
		}
		if strings.HasPrefix(n, "*.") && strings.HasSuffix(host, n[1:]) {
			return true
		}
	}
	return false
}

// pathMatches reports whether a request path falls under a location.
func pathMatches(prefix, path string) bool {
	if prefix == "" || prefix == "/" {
		return true
	}
	// A prefix location matches whole segments: /api takes /api and
	// /api/x, not /apix.
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return len(path) == len(prefix) || prefix[len(prefix)-1] == '/' || path[len(prefix)] == '/'
}

// stripPort removes the port from a Host header.
func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host[i:], "]") {
		return host[:i]
	}
	return host
}

// statusRecorder remembers what a handler answered, for the log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// Hijack lets a WebSocket or other Upgrade take the connection over.
func (s *statusRecorder) Hijack() (net.Conn, *bufio_ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("the response writer cannot be hijacked")
	}
	return h.Hijack()
}

// Flush passes a flush through, so that a streamed response arrives.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// newProxy builds the reverse proxy of a mapping.
func newProxy(m model.Mapping, spec Spec) (*httputil.ReverseProxy, *url.URL, error) {
	target := m.ProxyPassURL
	if target == "" {
		scheme := "http"
		if m.Protocol == model.HTTPS {
			// The upstream is usually plain HTTP behind a TLS front;
			// an explicit proxy_pass URL picks https.
			scheme = "http"
		}
		target = fmt.Sprintf("%s://%s:%d", scheme, m.UpstreamHost, m.UpstreamPort)
		if m.ListenPath == "/" || m.ListenPath == "" {
			target += "/"
		}
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, nil, fmt.Errorf("proxy_pass %q: %w", target, err)
	}

	transport := &http.Transport{
		Proxy:                 nil, // an internal upstream, never a proxy's
		DialContext:           (&net.Dialer{Timeout: dialTimeout(spec), KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		// A private upstream usually has a self-signed certificate, and
		// verifying it here would reject every internal HTTPS service.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		// Stream the response: nginx's proxy_buffering off, which is what
		// git push, server-sent events and large downloads need.
		DisableCompression: false,
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// The client's Host is what nginx's $host means, so it is read
			// before the request is retargeted at the upstream.
			clientHost := req.Host
			// proxy_pass without a path sends the request path unchanged,
			// which is what nginx does when the URL has no URI part.
			// A path in proxy_pass replaces the location prefix, as nginx's
			// trailing-slash rule does.
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			if u.Path != "" && u.Path != "/" {
				req.URL.Path = singleJoin(u.Path, req.URL.Path)
			}
			req.Host = req.URL.Host
			applyHeaders(req, m, clientHost)
		},
		Transport:     transport,
		FlushInterval: -1, // write through, as proxy_buffering off
		ErrorLog:      nil,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("502 bad gateway: " + err.Error()))
		},
	}
	return proxy, u, nil
}

// applyHeaders sets the headers a mapping asks for.
//
// clientHost is the Host the client sent, which is what nginx's $host
// expands to: by this point the request itself carries the upstream's.
func applyHeaders(req *http.Request, m model.Mapping, clientHost string) {
	switch m.ProxyMode {
	case model.ProxyMinimal:
		// Nothing added: the upstream sees what the client sent, with the
		// Host rewritten to the upstream above. The reverse proxy appends
		// X-Forwarded-For on its own, and the minimal mode is what a
		// backend that rejects extra headers (GitLab signing a webhook)
		// asks for, so that one is removed too.
		req.Header["X-Forwarded-For"] = nil
	case model.ProxyCustom:
		// The custom lines are nginx directives; the ones that map onto a
		// request header are honored, the rest is reported when applied.
		for name, value := range customHeaders(m.ProxyHeaders, req, clientHost) {
			req.Header.Set(name, value)
		}
	default:
		req.Header.Set("Host", clientHost)
		req.Header.Set("X-Real-IP", clientIP(req.RemoteAddr))
		req.Header.Set("X-Forwarded-For", appendForwarded(req))
		proto := "http"
		if req.TLS != nil {
			proto = "https"
		}
		req.Header.Set("X-Forwarded-Proto", proto)
		// WebSocket and other upgrades pass through as they are: the
		// client's Upgrade and Connection headers are left alone.
	}
}

// customHeaders reads the proxy_set_header lines of a custom mode.
func customHeaders(raw string, req *http.Request, clientHost string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "proxy_set_header ") {
			continue
		}
		rest := strings.TrimSuffix(strings.TrimPrefix(line, "proxy_set_header "), ";")
		fields := strings.SplitN(strings.TrimSpace(rest), " ", 2)
		if len(fields) != 2 {
			continue
		}
		name, value := fields[0], strings.TrimSpace(fields[1])
		value = strings.Trim(value, `"`)
		// The variables nginx would expand are the ones a mapping uses.
		value = strings.ReplaceAll(value, "$host", clientHost)
		value = strings.ReplaceAll(value, "$remote_addr", clientIP(req.RemoteAddr))
		value = strings.ReplaceAll(value, "$http_upgrade", req.Header.Get("Upgrade"))
		value = strings.ReplaceAll(value, "$connection_upgrade", connectionUpgrade(req))
		out[name] = value
	}
	return out
}

// connectionUpgrade is what nginx's map produces: "upgrade" for a request
// that upgrades, "close" otherwise.
func connectionUpgrade(req *http.Request) string {
	if req.Header.Get("Upgrade") != "" {
		return "upgrade"
	}
	return "close"
}

// appendForwarded extends X-Forwarded-For.
func appendForwarded(req *http.Request) string {
	prior := req.Header.Get("X-Forwarded-For")
	ip := clientIP(req.RemoteAddr)
	if prior == "" {
		return ip
	}
	return prior + ", " + ip
}

// clientIP returns the address of a RemoteAddr.
func clientIP(remote string) string {
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return host
	}
	return remote
}

// singleJoin joins a base path and a request path with one slash.
func singleJoin(base, path string) string {
	switch {
	case base == "":
		return path
	case path == "":
		return base
	case strings.HasSuffix(base, "/") && strings.HasPrefix(path, "/"):
		return base + path[1:]
	case !strings.HasSuffix(base, "/") && !strings.HasPrefix(path, "/"):
		return base + "/" + path
	}
	return base + path
}

// dialTimeout is the connection timeout an upstream gets.
func dialTimeout(spec Spec) time.Duration {
	if spec.DialTimeout > 0 {
		return spec.DialTimeout
	}
	return 10 * time.Second
}

// accessLog writes one line per request, in a shape close to nginx's.
type accessLog struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func openAccessLog(path string) (*accessLog, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &accessLog{file: f, path: path}, nil
}

// writeStream logs a forwarded connection, in the shape nginx's stream log
// has: the client, when it came in, and how long it lasted.
func (l *accessLog) writeStream(client, upstream string, d time.Duration) {
	if l == nil || l.file == nil {
		return
	}
	line := fmt.Sprintf("%s [%s] -> %s %s\n", client,
		time.Now().Format("02/Jan/2006:15:04:05 -0700"), upstream, d.Round(time.Millisecond))
	l.mu.Lock()
	_, _ = l.file.WriteString(line)
	l.mu.Unlock()
}

func (l *accessLog) write(port int, r *http.Request, status int, bytes int64) {
	if l == nil || l.file == nil {
		return
	}
	line := fmt.Sprintf("%s - [%s] \"%s %s %s\" %d %d \"%s\"\n",
		clientIP(r.RemoteAddr), time.Now().Format("02/Jan/2006:15:04:05 -0700"),
		r.Method, r.URL.RequestURI(), r.Proto, status, bytes, r.Referer())
	l.mu.Lock()
	_, _ = l.file.WriteString(line)
	l.mu.Unlock()
}

func (l *accessLog) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.file.Close()
	l.file = nil
	return err
}
