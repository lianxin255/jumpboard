package model

import "testing"

func TestValidPort(t *testing.T) {
	for _, c := range []struct {
		port int
		want bool
	}{
		{1, true}, {80, true}, {65535, true},
		{0, false}, {-1, false}, {65536, false},
	} {
		if got := ValidPort(c.port); got != c.want {
			t.Errorf("ValidPort(%d) = %v, want %v", c.port, got, c.want)
		}
	}
}

func TestValidIPOrCIDR(t *testing.T) {
	valid := []string{
		"192.168.1.1", "10.0.0.5", "0.0.0.0", "255.255.255.255",
		"192.168.1.0/24", "10.0.0.0/8", "172.16.0.0/12", "1.2.3.4/32",
	}
	for _, s := range valid {
		if !ValidIPOrCIDR(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	invalid := []string{
		"", "192.168.1", "192.168.1.1.1", "256.1.1.1", "1.2.3.-1",
		"01.2.3.4", "1.2.3.004", // leading zeros would read differently
		"0.0.0.0/0",  // would match everything and disable the whitelist
		"1.2.3.4/33", "1.2.3.4/-1", "1.2.3.4/", "abc", "1.2.3.4 5",
	}
	for _, s := range invalid {
		if ValidIPOrCIDR(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestValidProxyPassURL(t *testing.T) {
	valid := []string{
		"http://1.2.3.4/", "https://gitlab.com", "http://10.0.0.1:8080/path/",
	}
	for _, s := range valid {
		if !ValidProxyPassURL(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	invalid := []string{
		"", "ftp://x", "1.2.3.4", "http://x/;", "http://x/}", "http://x/{",
		"http://x/#", "http://x/\n", "http://" + string(make([]byte, 600)),
	}
	for _, s := range invalid {
		if ValidProxyPassURL(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestStripInject(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"normal", "normal"},
		{"a.com; } server {", "a.com  server"},
		{"line\nbreak", "linebreak"},
		{"# comment", "comment"},
		{"  padded  ", "padded"},
	} {
		if got := StripInject(c.in); got != c.want {
			t.Errorf("StripInject(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripProxyHeaders(t *testing.T) {
	got := StripProxyHeaders("proxy_set_header X-A 1;\n} server { listen 1;")
	if want := "proxy_set_header X-A 1;\n server  listen 1;"; got != want {
		t.Errorf("StripProxyHeaders = %q, want %q", got, want)
	}
}

func TestProtocolLabels(t *testing.T) {
	for _, p := range []Protocol{TCP, UDP, HTTP, HTTPS} {
		if !p.Valid() {
			t.Errorf("%q should be valid", p)
		}
		if p.Label() == "" {
			t.Errorf("%q has no label", p)
		}
	}
	if Protocol("icmp").Valid() {
		t.Error("icmp should be invalid")
	}
	if !TCP.IsStream() || !UDP.IsStream() || HTTP.IsStream() || HTTPS.IsStream() {
		t.Error("IsStream is wrong")
	}
}
