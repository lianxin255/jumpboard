package nginx

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// update rewrites the golden files instead of comparing them.
var update = flag.Bool("update", false, "rewrite the golden configuration files")

var fixedNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

type genCase struct {
	mappings []model.Mapping
	opts     GenOptions
}

// cases are the configurations whose exact output is locked down. The
// generator must keep producing these, or a user's existing mappings would
// change behavior on upgrade.
func cases() map[string]genCase {
	tcp := model.Mapping{
		Name: "MySQL 内网", Protocol: model.TCP, Enabled: true,
		ListenPort: 9000, UpstreamHost: "192.168.1.10", UpstreamPort: 3306,
	}
	udp := model.Mapping{
		Name: "DNS", Protocol: model.UDP, Enabled: true,
		ListenPort: 5353, UpstreamHost: "8.8.8.8", UpstreamPort: 53,
	}
	httpStd := model.Mapping{
		Name: "GitLab Web", Protocol: model.HTTP, Enabled: true,
		ListenPort: 8080, ServerNames: "git.example.com", ListenPath: "/",
		UpstreamHost: "192.168.1.20", UpstreamPort: 80, ProxyMode: model.ProxyStandard,
	}
	https := model.Mapping{
		Name: "TLS 站点", Protocol: model.HTTPS, Enabled: true,
		ListenPort: 443, ServerNames: "app.example.com", ListenPath: "/api",
		UpstreamHost: "10.0.0.7", UpstreamPort: 8080, ProxyMode: model.ProxyStandard,
		TLSCertPath: "/etc/ssl/app.pem", TLSKeyPath: "/etc/ssl/app.key", RedirectHTTP: true,
	}

	return map[string]genCase{
		"tcp":            {[]model.Mapping{tcp}, GenOptions{}},
		"udp":            {[]model.Mapping{udp}, GenOptions{}},
		"http-standard":  {[]model.Mapping{httpStd}, GenOptions{}},
		"https-redirect": {[]model.Mapping{https}, GenOptions{}},
		"whitelist":      {[]model.Mapping{tcp, httpStd}, GenOptions{WhitelistIPs: []string{"192.168.1.0/24", "10.0.0.5"}}},
		"extra-directives": {[]model.Mapping{{
			Name: "Redis", Protocol: model.TCP, Enabled: true, ListenPort: 9100,
			UpstreamHost: "192.168.1.30", UpstreamPort: 6379,
			ExtraDirectives: "proxy_timeout 1h;\nproxy_connect_timeout 5s;",
		}}, GenOptions{}},
		"http-minimal": {[]model.Mapping{{
			Name: "精简", Protocol: model.HTTP, Enabled: true, ListenPort: 8081,
			UpstreamHost: "10.0.0.5", UpstreamPort: 80, ProxyMode: model.ProxyMinimal,
		}}, GenOptions{}},
		"http-custom": {[]model.Mapping{{
			Name: "自定义头", Protocol: model.HTTP, Enabled: true, ListenPort: 8082,
			UpstreamHost: "10.0.0.6", UpstreamPort: 80, ProxyMode: model.ProxyCustom,
			ProxyHeaders: "proxy_set_header Host $host;\nproxy_set_header X-Real-IP $remote_addr;",
		}}, GenOptions{}},
		"http-proxypass-url": {[]model.Mapping{{
			Name: "自定义 proxy_pass", Protocol: model.HTTP, Enabled: true, ListenPort: 8083,
			UpstreamHost: "ignored", UpstreamPort: 80, ProxyPassURL: "http://1.2.3.4/",
		}}, GenOptions{}},
		"disabled-skipped": {[]model.Mapping{
			tcp,
			{Name: "关掉的", Protocol: model.TCP, Enabled: false, ListenPort: 9200,
				UpstreamHost: "10.0.0.9", UpstreamPort: 6379},
		}, GenOptions{}},
		"empty": {nil, GenOptions{}},
	}
}

// TestGolden locks the exact output of the generator.
func TestGolden(t *testing.T) {
	dir := "testdata"
	for name, c := range cases() {
		t.Run(name, func(t *testing.T) {
			opts := c.opts
			opts.Now = fixedNow
			got := Generate(c.mappings, opts)

			path := filepath.Join(dir, name+".conf")
			if *update {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the golden file (run -update to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("output changed:\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
			}
		})
	}
}

// lineServer and lineListen match a directive at the start of a line, so
// that text a user typed inside a value — "a.com  server  listen 6666" —
// is not mistaken for a directive of its own.
var (
	lineServer = regexp.MustCompile(`(?m)^\s*server\s*\{`)
	lineListen = regexp.MustCompile(`(?m)^\s*listen\s+\d`)
)

// TestInjectionNeverEscapes checks that the values a user types cannot
// start a directive or close the block they are written into.
func TestInjectionNeverEscapes(t *testing.T) {
	clean := model.Mapping{
		Name: "clean", Protocol: model.HTTP, Enabled: true, ListenPort: 8080,
		ServerNames: "ok.example.com", ListenPath: "/",
		UpstreamHost: "1.2.3.4", UpstreamPort: 80,
	}
	evil := clean
	evil.Name = "x\nevil"
	evil.ServerNames = "a.com; } server { listen 6666;"
	evil.ListenPath = "/; } server {"
	evil.UpstreamHost = "1.2.3.4; }"

	base := Generate([]model.Mapping{clean}, GenOptions{})
	got := Generate([]model.Mapping{evil}, GenOptions{})

	if n, want := len(lineServer.FindAllString(got, -1)), len(lineServer.FindAllString(base, -1)); n != want {
		t.Errorf("%d server blocks, want %d:\n%s", n, want, got)
	}
	if n, want := len(lineListen.FindAllString(got, -1)), len(lineListen.FindAllString(base, -1)); n != want {
		t.Errorf("%d listen directives, want %d:\n%s", n, want, got)
	}
	// The text may survive as part of a value ("server_name a.com server
	// listen 6666"), which nginx reads as a hostname: harmless. What must
	// not survive is a directive of its own, checked above, or a
	// character that could close a block.
	if lineListen.MatchString(strings.ReplaceAll(got, "listen 8080", "")) {
		t.Errorf("an extra listen directive appeared:\n%s", got)
	}
	for _, bad := range []string{"a.com;", "1.2.3.4;", "/;"} {
		if strings.Contains(got, bad) {
			t.Errorf("injected character survived in %q:\n%s", bad, got)
		}
	}
}

// TestWhitelistNeverLocksOut checks that a whitelist whose entries all
// sanitize away does not leave a bare deny all behind.
func TestWhitelistNeverLocksOut(t *testing.T) {
	conf := Generate([]model.Mapping{{
		Name: "t", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "1.2.3.4", UpstreamPort: 3306,
	}}, GenOptions{WhitelistIPs: []string{";;", "##"}})

	if strings.Contains(conf, "deny all") {
		t.Errorf("deny all without an allow would lock the admin out:\n%s", conf)
	}
}

// TestInvalidPortSkipped checks that a mapping with no usable listen port
// produces no server block, rather than a broken one.
func TestInvalidPortSkipped(t *testing.T) {
	for _, port := range []int{0, -1, 70000} {
		conf := Generate([]model.Mapping{{
			Name: "bad", Protocol: model.TCP, Enabled: true, ListenPort: port,
			UpstreamHost: "1.2.3.4", UpstreamPort: 3306,
		}}, GenOptions{})
		if strings.Contains(conf, "server {") {
			t.Errorf("port %d produced a server block:\n%s", port, conf)
		}
	}
}

// TestProxyHeadersCannotEscape checks that the custom proxy headers drop
// the characters that would close the location block, keeping the
// semicolons a directive needs.
func TestProxyHeadersCannotEscape(t *testing.T) {
	conf := Generate([]model.Mapping{{
		Name: "h", Protocol: model.HTTP, Enabled: true, ListenPort: 8080,
		UpstreamHost: "1.2.3.4", UpstreamPort: 80, ProxyMode: model.ProxyCustom,
		ProxyHeaders: "proxy_set_header X-A 1;\n} server { listen 6666;",
	}}, GenOptions{})

	if strings.Contains(conf, "listen 6666") {
		t.Errorf("proxy headers escaped their block:\n%s", conf)
	}
	if !strings.Contains(conf, "proxy_set_header X-A 1;") {
		t.Errorf("the semicolon a directive needs was stripped:\n%s", conf)
	}
}
