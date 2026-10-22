package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// Local is the engine that runs the rules itself, in this process.
type Local struct {
	mu      sync.Mutex
	running map[ruleKey]forwarder
	rules   []ruleEntry
	spec    Spec
	access  *accessLog
	status  Status
	started time.Time
	// lastWarning holds the directives that could not be honored.
	lastWarning []string
}

// NewLocal returns an engine with no rules running.
func NewLocal() *Local {
	return &Local{running: map[ruleKey]forwarder{}, status: Stopped}
}

// Mode names the engine.
func (l *Local) Mode() string { return "内置引擎（Go）" }

// Status reports whether any rule is running.
func (l *Local) Status() Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status
}

// ruleEntry pairs a rule's row with the listener that serves it.
type ruleEntry struct {
	status RuleStatus
	key    ruleKey
}

// Rules reports what each rule is doing.
func (l *Local) Rules() []RuleStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]RuleStatus, 0, len(l.rules))
	for _, entry := range l.rules {
		row := entry.status
		// An HTTP rule is served by its port's server, which the key
		// names; a stream rule by its own listener.
		if f, ok := l.running[entry.key]; ok {
			n, active := f.stats()
			row.Connections, row.Active, row.Listening = n, active, true
		} else if entry.key.proto == model.HTTP || entry.key.proto == model.HTTPS {
			// The port's server holds several routes: it is found by the
			// listen address alone.
			for key, f := range l.running {
				if key.listen == entry.key.listen && (key.proto == model.HTTP || key.proto == model.HTTPS) {
					// The counters belong to the port, so the first route
					// of it reports them.
					n, active := f.stats()
					row.Connections, row.Active, row.Listening = n, active, true
					break
				}
			}
		}
		out = append(out, row)
	}
	return out
}

// Stop closes every listener.
func (l *Local) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, f := range l.running {
		_ = f.Close()
		delete(l.running, key)
	}
	l.access.Close()
	l.access = nil
	l.status = Stopped
	for i := range l.rules {
		l.rules[i].status.Listening = false
		l.rules[i].status.Active = 0
	}
	return nil
}

// Apply replaces the rules with spec.
//
// The whole set is prepared before anything is switched over: every new
// listener is opened first, so a port that cannot be bound fails the apply
// while the previous rules keep running. Only then are the listeners that
// are no longer wanted closed.
//
// Rules whose listen address and upstream are unchanged are left alone,
// which keeps an apply from dropping live connections — the app applies
// after every edit.
func (l *Local) Apply(spec Spec) (Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var res Result
	warnings := unsupportedDirectives(spec.Mappings)
	res.Warnings = warnings
	// The access log of a previous apply stays open: it is one file the
	// rules append to, and reopening it per apply would lose lines.
	if err := l.openAccess(spec.LogDir); err != nil {
		res.Warnings = append(res.Warnings, "无法写访问日志: "+err.Error())
	}

	// Group the HTTP and HTTPS mappings by port: one server per port
	// routes among them.
	byPort := map[int][]route{}
	broken := map[string]string{}
	for _, m := range spec.Mappings {
		if m.Protocol.IsStream() {
			continue
		}
		r, err := buildRoute(m, spec)
		if err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("「%s」%v", m.Name, err))
			broken[m.ID] = err.Error()
			continue
		}
		byPort[m.ListenPort] = append(byPort[m.ListenPort], r)
	}

	want := map[ruleKey]bool{}
	next := map[ruleKey]forwarder{}
	var opened []forwarder

	// Stream rules.
	for _, m := range spec.Mappings {
		if !m.Protocol.IsStream() {
			continue
		}
		upstream := net.JoinHostPort(m.UpstreamHost, fmt.Sprint(m.UpstreamPort))
		addr := listenAddr(m.ListenPort)
		key := ruleKey{proto: m.Protocol, listen: addr, upstream: upstream}
		if existing, ok := l.running[key]; ok {
			want[key] = true
			next[key] = existing
			res.Retargeted = append(res.Retargeted, addr)
			continue
		}
		// The same port with a different upstream is a retarget, not a new
		// rule: the old listener still holds the port, so opening another
		// would fail with "address already in use" — and changing a
		// backend is the most common edit there is.
		if old, oldKey, ok := l.streamOn(addr, m.Protocol); ok {
			// The listener moves to the map under its new key; the entry
			// under the old one must survive the "close what is no longer
			// wanted" pass, because both keys name the same forwarder.
			// oldKey is the key as it sits in the map, not old.key(): a
			// stream forwarder reports its bound address, which differs in
			// form from the ":port" these keys are built from.
			old.setUpstream(upstream)
			want[oldKey] = true
			want[key] = true
			next[key] = old
			res.Retargeted = append(res.Retargeted, addr)
			continue
		}
		idle := idleFor(m, spec)
		whitelist := parseWhitelist(spec.WhitelistIPs)
		var (
			f   forwarder
			err error
		)
		if m.Protocol == model.UDP {
			f, err = newUDPForwarder(addr, upstream, idle, whitelist)
		} else {
			f, err = newTCPForwarder(addr, upstream, idle, l.access, whitelist)
		}
		if err != nil {
			closeAll(opened)
			return res, &ErrBind{Port: m.ListenPort, Proto: strings.ToUpper(string(m.Protocol)), Err: err}
		}
		want[key] = true
		next[key] = f
		opened = append(opened, f)
		res.Started = append(res.Started, addr)
	}

	// HTTP and HTTPS ports.
	for port, routes := range byPort {
		proto := routes[0].mapping.Protocol
		// A port serves one protocol. Grouping by port alone would serve an
		// HTTPS rule as plain HTTP whenever an HTTP rule shares the port:
		// the certificate would never load, and the rule would quietly run
		// with less protection than it says.
		for _, r := range routes[1:] {
			if r.mapping.Protocol != proto {
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("「%s」没有生效：%s 端口 %d 已被 %s 规则占用",
						r.mapping.Name, r.mapping.Protocol.Label(), port, proto.Label()))
			}
		}
		key := ruleKey{proto: proto, listen: listenAddr(port)}
		// Load the certificates first: a rule whose certificate cannot be
		// read must not take the running site down, and the old listener
		// has to go before the new one can bind the same port.
		cfg, err := prepareTLS(proto, routes)
		if err != nil {
			closeAll(opened)
			return res, &ErrBind{Port: port, Proto: strings.ToUpper(string(proto)), Err: err}
		}
		for k, old := range l.running {
			if k.listen == key.listen && (k.proto == model.HTTP || k.proto == model.HTTPS) {
				want[k] = true
				_ = old.Close()
				delete(l.running, k)
			}
		}
		f, err := startHTTPForwarder(port, proto, routes, spec, l.access, cfg)
		if err != nil {
			closeAll(opened)
			return res, &ErrBind{Port: port, Proto: strings.ToUpper(string(proto)), Err: err}
		}
		want[key] = true
		next[key] = f
		opened = append(opened, f)
		res.Started = append(res.Started, listenAddr(port))
	}

	// The HTTP→HTTPS redirects. One listener on port 80 serves every
	// mapping that asks for one, the way the generated configuration's
	// shared redirect block did.
	res.Warnings = append(res.Warnings, l.applyRedirects(spec, byPort, want, next, &opened)...)

	// Close what is no longer wanted.
	for key, f := range l.running {
		if want[key] {
			continue
		}
		if _, replaced := next[key]; replaced {
			continue
		}
		_ = f.Close()
		delete(l.running, key)
		res.Stopped = append(res.Stopped, key.listen)
	}

	l.running = next
	l.spec = spec
	l.status = Running
	l.started = time.Now()
	l.lastWarning = warnings
	l.rules = statuses(spec.Mappings, broken)

	sort.Strings(res.Started)
	sort.Strings(res.Stopped)
	res.OK = true
	return res, nil
}

// streamOn finds the running stream listener on an address, whatever its
// upstream: a rule pointed at a new backend keeps its port. It returns the
// key the forwarder sits under, so the caller can keep that entry alive.
func (l *Local) streamOn(addr string, proto model.Protocol) (streamRetargeter, ruleKey, bool) {
	for k, f := range l.running {
		if k.listen != addr || k.proto != proto {
			continue
		}
		if r, ok := f.(streamRetargeter); ok {
			return r, k, true
		}
	}
	return nil, ruleKey{}, false
}

// streamRetargeter is a stream forwarder whose upstream can be swapped
// without closing the listener.
type streamRetargeter interface {
	forwarder
	setUpstream(string)
}

// applyRedirects starts or keeps the port-80 listener that sends plain
// HTTP to the TLS port.
//
// The redirect is a convenience: a port it cannot bind — port 80 wants
// privileges, and an HTTP rule may already hold it — degrades to a warning
// instead of failing the apply, because the TLS rules it belongs to still
// work without it.
func (l *Local) applyRedirects(spec Spec, byPort map[int][]route,
	want map[ruleKey]bool, next map[ruleKey]forwarder, opened *[]forwarder) []string {

	if _, taken := byPort[80]; taken {
		return []string{"HTTP→HTTPS 跳转未生效：80 端口已由 HTTP 规则使用"}
	}
	wanted := false
	for _, m := range spec.Mappings {
		if m.Protocol == model.HTTPS && m.RedirectHTTP &&
			strings.TrimSpace(m.ServerNames) != "" && m.ListenPort != 80 {
			wanted = true
			break
		}
	}
	if !wanted {
		return nil
	}
	// The whitelist is part of the listener's identity: a redirect that
	// keeps answering after the list shrinks leaks the site to a client
	// the user just removed. The upstream slot of the key carries its
	// fingerprint, so changing the list rebuilds the listener.
	key := ruleKey{proto: redirectProto, listen: listenAddr(80),
		upstream: strings.Join(spec.WhitelistIPs, ",")}
	if existing, ok := l.running[key]; ok {
		want[key] = true
		next[key] = existing
		return nil
	}
	for k, old := range l.running {
		if k.proto == redirectProto && k.listen == listenAddr(80) {
			// Port 80 cannot be bound twice: the stale redirect goes first.
			_ = old.Close()
			delete(l.running, k)
		}
	}
	f, err := newRedirectForwarder(80, parseWhitelist(spec.WhitelistIPs), l.access)
	if err != nil {
		return []string{"HTTP→HTTPS 跳转未生效：无法监听 80 端口（需要管理员权限或端口被占用）"}
	}
	want[key] = true
	next[key] = f
	*opened = append(*opened, f)
	return nil
}

// statuses builds the per-rule rows the window shows.
// statuses builds the per-rule rows the window shows. broken maps a
// mapping ID to why its route could not be built: such a rule is not
// listening, whatever its port's server does for the rules that did build.
func statuses(mappings []model.Mapping, broken map[string]string) []ruleEntry {
	out := make([]ruleEntry, 0, len(mappings))
	for _, m := range mappings {
		upstream := net.JoinHostPort(m.UpstreamHost, fmt.Sprint(m.UpstreamPort))
		row := RuleStatus{
			MappingID: m.ID,
			Name:      m.Name,
			Listen:    listenAddr(m.ListenPort),
			Backend:   upstream,
		}
		if m.ListenPort == 0 {
			row.Listen = ""
			row.Err = "没有监听端口"
		}
		if msg, bad := broken[m.ID]; bad {
			// A rule whose route failed to build is not served by its
			// port's server, however well that server is doing for the
			// rules that did build.
			row.Err = msg
			row.Listening = false
		}
		key := ruleKey{proto: m.Protocol, listen: listenAddr(m.ListenPort), upstream: upstream}
		if m.Protocol.IsStream() {
			out = append(out, ruleEntry{status: row, key: key})
			continue
		}
		// An HTTP route is served by its port's server, whose key has no
		// upstream of its own.
		row.Backend = upstream
		out = append(out, ruleEntry{
			status: row,
			key:    ruleKey{proto: m.Protocol, listen: listenAddr(m.ListenPort)},
		})
	}
	return out
}

// buildRoute prepares one HTTP mapping.
func buildRoute(m model.Mapping, spec Spec) (route, error) {
	proxy, u, tr, err := newProxy(m, spec)
	if err != nil {
		return route{}, err
	}
	path := m.ListenPath
	if path == "" {
		path = "/"
	}
	return route{
		mapping:     m,
		upstream:    u,
		proxy:       proxy,
		serverNames: splitNames(m.ServerNames),
		path:        path,
		transport:   tr,
	}, nil
}

// idleFor returns how long a rule keeps a quiet connection.
func idleFor(m model.Mapping, spec Spec) time.Duration {
	if d := parseTimeout(m.ExtraDirectives); d > 0 {
		return d
	}
	if spec.IdleTimeout > 0 {
		return spec.IdleTimeout
	}
	return 5 * time.Minute
}

// closeAll closes what an apply opened before it failed.
func closeAll(fs []forwarder) {
	for _, f := range fs {
		_ = f.Close()
	}
}

// timeoutDirective finds proxy_timeout and the proxy_*_timeout lines a
// mapping may carry.
var timeoutDirective = regexp.MustCompile(`(?m)^\s*proxy_(?:read_)?timeout\s+([0-9]+)(ms|s|m|h)?\s*;`)

// parseTimeout reads a timeout from a mapping's extra directives.
//
// The directives a mapping carries are nginx's, so the value is parsed the
// nginx way, including the unit-less seconds.
func parseTimeout(extra string) time.Duration {
	m := timeoutDirective.FindStringSubmatch(extra)
	if m == nil {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil || n <= 0 {
		return 0
	}
	switch m[2] {
	case "ms":
		return time.Duration(n) * time.Millisecond
	case "m":
		return time.Duration(n) * time.Minute
	case "h":
		return time.Duration(n) * time.Hour
	}
	return time.Duration(n) * time.Second
}

// unsupportedDirectives reports the directives the engine cannot honor, so
// that the user learns what an edit means rather than finding a directive
// silently missing.
func unsupportedDirectives(mappings []model.Mapping) []string {
	known := map[string]bool{
		"proxy_timeout":         true,
		"proxy_read_timeout":    true,
		"proxy_connect_timeout": true,
		"proxy_http_version":    true,
		"proxy_buffering":       true,
		"proxy_set_header":      true,
	}
	directive := regexp.MustCompile(`^\s*([a-z_]+)`)
	var out []string
	for _, m := range mappings {
		for _, key := range []string{m.ExtraDirectives, m.ProxyHeaders} {
			for _, line := range strings.Split(key, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				sub := directive.FindStringSubmatch(line)
				if sub == nil || known[sub[1]] {
					continue
				}
				out = append(out, fmt.Sprintf("「%s」的 %s 由内置引擎忽略", m.Name, sub[1]))
			}
		}
	}
	return out
}

// LogDir is where the engine writes its logs, for the window.
func (l *Local) LogDir() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spec.LogDir
}

// openAccess makes sure the access log is open.
func (l *Local) openAccess(dir string) error {
	if l.access != nil {
		return nil
	}
	if dir == "" {
		return nil
	}
	a, err := openAccessLog(filepath.Join(dir, "access.log"))
	if err != nil {
		return err
	}
	l.access = a
	return nil
}

// Check reports whether the rules could be started, without starting them.
//
// It binds every port the rules need for a moment and lets it go, and
// resolves each upstream name. That is what the old nginx -t did: it told
// the user about the mistakes before the running rules changed.
func (l *Local) Check(spec Spec) error {
	// HTTPS needs its certificates readable.
	for _, m := range spec.Mappings {
		if m.Protocol != model.HTTPS {
			continue
		}
		if m.TLSCertPath == "" || m.TLSKeyPath == "" {
			return fmt.Errorf("「%s」缺少证书或私钥路径", m.Name)
		}
		if _, err := tls.LoadX509KeyPair(m.TLSCertPath, m.TLSKeyPath); err != nil {
			return fmt.Errorf("「%s」的证书无法读取：%v", m.Name, err)
		}
	}

	// An upstream name that does not resolve would fail every request
	// silently, so it is reported here the way nginx reports it.
	for _, m := range spec.Mappings {
		if m.UpstreamHost == "" {
			return fmt.Errorf("「%s」缺少上游地址", m.Name)
		}
		if net.ParseIP(m.UpstreamHost) != nil {
			continue
		}
		// A resolver that answers nothing is no proof the name exists, and
		// a lookup that hangs would freeze the preflight: a short deadline
		// and the error both count as unresolved.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := net.DefaultResolver.LookupHost(ctx, m.UpstreamHost)
		cancel()
		if err != nil {
			return fmt.Errorf("「%s」的上游 %s 无法解析", m.Name, m.UpstreamHost)
		}
	}

	// Each port is bound once, and left to whoever holds it while the
	// check runs.
	seen := map[string]string{}
	for _, m := range spec.Mappings {
		proto := "tcp"
		if m.Protocol == model.UDP {
			proto = "udp"
		}
		addr := listenAddr(m.ListenPort)
		if other, dup := seen[proto+addr]; dup {
			return fmt.Errorf("「%s」和「%s」都监听 %s", other, m.Name, addr)
		}
		seen[proto+addr] = m.Name

		// An HTTP and an HTTPS rule cannot share a port either.
		if m.Protocol.IsStream() {
			if other, dup := seen["webtcp"+addr]; dup {
				return fmt.Errorf("「%s」和「%s」都监听 %s", other, m.Name, addr)
			}
			seen["stream"+addr] = m.Name
		} else {
			if other, dup := seen["stream"+addr]; dup {
				return fmt.Errorf("「%s」和「%s」都监听 %s", other, m.Name, addr)
			}
			if other, dup := seen["webtcp"+addr]; dup {
				return fmt.Errorf("「%s」和「%s」都监听 %s", other, m.Name, addr)
			}
			seen["webtcp"+addr] = m.Name
		}

		if proto == "udp" {
			// A port this engine already holds is not "taken" in the sense
			// the check means: the question is whether applying would
			// work, and reapplying a running rule always does.
			if l.holds(addr, proto) {
				continue
			}
			pc, err := net.ListenPacket("udp", addr)
			if err != nil {
				return &ErrBind{Port: m.ListenPort, Proto: "UDP", Err: err}
			}
			_ = pc.Close()
			continue
		}
		if l.holds(addr, "tcp") {
			continue
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return &ErrBind{Port: m.ListenPort, Proto: "TCP", Err: err}
		}
		_ = ln.Close()
	}
	return nil
}

// holds reports whether this engine already listens on an address, which
// the preflight must not count as someone else's port.
func (l *Local) holds(addr, proto string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.running {
		if k.listen != addr {
			continue
		}
		if proto == "udp" && k.proto == model.UDP {
			return true
		}
		if proto == "tcp" && k.proto != model.UDP {
			return true
		}
	}
	return false
}

// RuleState reports whether the rule of a mapping is listening, and why
// not when it is not.
//
// The table shows it next to each rule, so that "applied but the port was
// taken" is visible without opening the log.
func (l *Local) RuleState(mappingID string) (listening bool, problem string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.rules {
		if entry.status.MappingID != mappingID {
			continue
		}
		if entry.status.Err != "" {
			return false, entry.status.Err
		}
		_, ok := l.running[entry.key]
		if ok {
			return true, ""
		}
		if entry.key.proto == model.HTTP || entry.key.proto == model.HTTPS {
			for key := range l.running {
				if key.listen == entry.key.listen {
					return true, ""
				}
			}
		}
		return false, "未应用"
	}
	return false, "未应用"
}
