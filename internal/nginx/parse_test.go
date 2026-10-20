package nginx

import (
	"testing"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// TestParseRoundTrip checks that the generator's output can be read back
// into the mappings it came from: a snapshot rollback depends on it.
func TestParseRoundTrip(t *testing.T) {
	in := []model.Mapping{
		{Name: "MySQL 内网", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
			UpstreamHost: "192.168.1.10", UpstreamPort: 3306},
		{Name: "DNS", Protocol: model.UDP, Enabled: true, ListenPort: 5353,
			UpstreamHost: "8.8.8.8", UpstreamPort: 53},
		{Name: "GitLab", Protocol: model.HTTP, Enabled: true, ListenPort: 8080,
			ServerNames: "git.example.com", ListenPath: "/", UpstreamHost: "192.168.1.20",
			UpstreamPort: 80, ProxyMode: model.ProxyStandard},
		{Name: "TLS 站点", Protocol: model.HTTPS, Enabled: true, ListenPort: 443,
			ServerNames: "app.example.com", ListenPath: "/api",
			UpstreamHost: "10.0.0.7", UpstreamPort: 8080,
			TLSCertPath: "/etc/ssl/app.pem", TLSKeyPath: "/etc/ssl/app.key",
			RedirectHTTP: true, ProxyMode: model.ProxyStandard},
	}
	conf := Generate(in, GenOptions{Now: fixedNow})

	got, err := Parse(conf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("parsed %d rules, want %d:\n%s", len(got), len(in), conf)
	}

	// The generator writes the http rules before the stream ones, so the
	// rules are compared by name rather than by position.
	byName := map[string]model.Mapping{}
	for _, m := range got {
		byName[m.Name] = m
	}
	for _, want := range in {
		have, ok := byName[want.Name]
		if !ok {
			t.Errorf("rule %q is missing from the parse", want.Name)
			continue
		}
		if have.Protocol != want.Protocol ||
			have.ListenPort != want.ListenPort ||
			have.UpstreamHost != want.UpstreamHost ||
			have.UpstreamPort != want.UpstreamPort {
			t.Errorf("%q:\n got %+v\nwant %+v", want.Name, have, want)
		}
		if want.ServerNames != "" && have.ServerNames != want.ServerNames {
			t.Errorf("%q server_name: got %q want %q", want.Name, have.ServerNames, want.ServerNames)
		}
		if want.TLSCertPath != "" && have.TLSCertPath != want.TLSCertPath {
			t.Errorf("%q cert: got %q want %q", want.Name, have.TLSCertPath, want.TLSCertPath)
		}
		if want.RedirectHTTP && !have.RedirectHTTP {
			t.Errorf("%q should keep its HTTP redirect", want.Name)
		}
	}
}

// TestParseRejectsTruncated checks that a half-written configuration is an
// error rather than a partial rollback.
func TestParseRejectsTruncated(t *testing.T) {
	conf := "http {\n  server {\n    listen 8080;\n"
	if _, err := Parse(conf); err == nil {
		t.Error("a truncated configuration should be an error")
	}
}
