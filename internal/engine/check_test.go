package engine

import (
	"net"
	"strings"
	"testing"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// TestCheckNamesEachProblem checks that the preflight names what would stop
// the rules from running, which is the whole point of the button.
func TestCheckNamesEachProblem(t *testing.T) {
	e := NewLocal()
	defer e.Stop()

	t.Run("one port twice", func(t *testing.T) {
		port := freePort(t)
		err := e.Check(Spec{Mappings: []model.Mapping{
			{ID: "1", Name: "一", Protocol: model.TCP, Enabled: true, ListenPort: port, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
			{ID: "2", Name: "二", Protocol: model.TCP, Enabled: true, ListenPort: port, UpstreamHost: "127.0.0.1", UpstreamPort: 2},
		}})
		if err == nil || !strings.Contains(err.Error(), "都监听") {
			t.Fatalf("want a duplicate-port error, got %v", err)
		}
	})

	t.Run("port taken by another program", func(t *testing.T) {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		err = e.Check(Spec{Mappings: []model.Mapping{{
			ID: "1", Name: "占用的", Protocol: model.TCP, Enabled: true,
			ListenPort:   l.Addr().(*net.TCPAddr).Port,
			UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		}}})
		if err == nil || !strings.Contains(err.Error(), "无法监听") {
			t.Fatalf("want a bind error, got %v", err)
		}
	})

	t.Run("upstream that does not resolve", func(t *testing.T) {
		err := e.Check(Spec{Mappings: []model.Mapping{{
			ID: "1", Name: "坏上游", Protocol: model.TCP, Enabled: true,
			ListenPort: freePort(t),
			// A name no resolver can accept: some networks answer every
			// syntactically valid name, so a made-up domain proves nothing.
			UpstreamHost: "x..y", UpstreamPort: 1,
		}}})
		if err == nil || !strings.Contains(err.Error(), "无法解析") {
			t.Fatalf("want a resolution error, got %v", err)
		}
	})

	t.Run("https without a certificate", func(t *testing.T) {
		err := e.Check(Spec{Mappings: []model.Mapping{{
			ID: "1", Name: "无证书", Protocol: model.HTTPS, Enabled: true,
			ListenPort: freePort(t), ServerNames: "x.example.com",
			UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		}}})
		if err == nil || !strings.Contains(err.Error(), "证书") {
			t.Fatalf("want a certificate error, got %v", err)
		}
	})

	t.Run("a good set passes", func(t *testing.T) {
		if err := e.Check(Spec{Mappings: []model.Mapping{{
			ID: "1", Name: "好", Protocol: model.TCP, Enabled: true,
			ListenPort: freePort(t), UpstreamHost: "127.0.0.1", UpstreamPort: 1,
		}}}); err != nil {
			t.Fatalf("a good rule should pass: %v", err)
		}
	})
}

// TestUnknownHostFallsToTheDefault checks nginx's default-server rule.
func TestUnknownHostFallsToTheDefault(t *testing.T) {
	for _, c := range []struct {
		names []string
		host  string
		want  bool
	}{
		{nil, "anything.example.com", true},
		{[]string{"a.example.com"}, "a.example.com", true},
		{[]string{"a.example.com"}, "b.example.com", false},
		{[]string{"*.example.com"}, "x.example.com", true},
		{[]string{"*.example.com"}, "example.com", false},
	} {
		if got := hostMatches(c.names, c.host); got != c.want {
			t.Errorf("hostMatches(%v, %q) = %v, want %v", c.names, c.host, got, c.want)
		}
	}
}

// TestPathPrefixBoundary checks that a location matches whole segments.
func TestPathPrefixBoundary(t *testing.T) {
	for _, c := range []struct {
		prefix, path string
		want         bool
	}{
		{"/", "/anything", true},
		{"/api", "/api", true},
		{"/api", "/api/users", true},
		{"/api", "/apix", false},
		{"/api/", "/api/users", true},
		{"", "/x", true},
	} {
		if got := pathMatches(c.prefix, c.path); got != c.want {
			t.Errorf("pathMatches(%q, %q) = %v, want %v", c.prefix, c.path, got, c.want)
		}
	}
}
