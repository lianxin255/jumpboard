// Package nginx generates and parses nginx configurations: the export
// format and the snapshot carrier. The rules themselves run in the Go
// engine, not in nginx.
package nginx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// GenOptions are the extras a configuration is built with.
type GenOptions struct {
	// WhitelistIPs, when not empty, adds allow/deny to every server block.
	WhitelistIPs []string
	// Now is the timestamp written into the header, for stable output in
	// tests. Zero means the current time.
	Now time.Time
}

// sanitize removes the characters that could end the directive a value is
// written into, or escape its block.
func sanitize(v string) string { return model.StripInject(v) }

// safePort returns p when it is a usable port, and 0 otherwise.
func safePort(p int) int {
	if p < 1 || p > 65535 {
		return 0
	}
	return p
}

// renderExtra indents the user's extra directives, dropping blank lines.
func renderExtra(extra, indent string) []string {
	if strings.TrimSpace(extra) == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(extra, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, indent+t)
		}
	}
	return out
}

// standardProxyHeaders are the headers a standard reverse proxy sets.
//
// HTTP/1.1, body size, timeouts and buffering live in the http block, so
// every mode inherits them. The headers cannot move up there:
// proxy_set_header replaces the whole set, so a minimal or custom location
// would inherit Host and be unable to drop it.
func standardProxyHeaders(indent string) []string {
	return []string{
		indent + "proxy_set_header Host $host;",
		indent + "proxy_set_header X-Real-IP $remote_addr;",
		indent + "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
		indent + "proxy_set_header X-Forwarded-Proto $scheme;",
		indent + "proxy_set_header Upgrade $http_upgrade;",
		indent + "proxy_set_header Connection $connection_upgrade;",
	}
}

// proxyDirective matches a line of the custom proxy headers that may be
// written inside a location: a proxy_* directive ending with a semicolon,
// with no brace, quote or newline in it.
var proxyDirective = regexp.MustCompile(`^proxy_[a-z_]+[ \t]+[^;{}#"'\x00-\x1f]+;$`)

// renderProxyDirectives renders what follows proxy_pass in a location.
//
// The custom mode accepts only proxy_* directives, so a line that closes
// the location or starts a server block is dropped rather than trusted:
// the field is for setting headers, not for writing arbitrary nginx.
func renderProxyDirectives(m model.Mapping, indent string) []string {
	switch m.ProxyMode {
	case model.ProxyMinimal:
		return nil
	case model.ProxyCustom:
		if strings.TrimSpace(m.ProxyHeaders) == "" {
			return standardProxyHeaders(indent)
		}
		var out []string
		for _, line := range strings.Split(model.StripProxyHeaders(m.ProxyHeaders), "\n") {
			t := strings.TrimSpace(line)
			if t == "" {
				continue
			}
			if !proxyDirective.MatchString(t) {
				continue // not a header directive: leave it out entirely
			}
			out = append(out, indent+t)
		}
		if len(out) == 0 {
			return standardProxyHeaders(indent)
		}
		return out
	}
	return standardProxyHeaders(indent)
}

// renderWhitelist renders allow lines and a closing deny all. It emits no
// deny all when no address survives sanitizing, which would lock the
// administrator out.
func renderWhitelist(ips []string, indent string) []string {
	if len(ips) == 0 {
		return nil
	}
	var out []string
	for _, ip := range ips {
		if clean := sanitize(ip); clean != "" {
			out = append(out, indent+"allow "+clean+";")
		}
	}
	if len(out) > 0 {
		out = append(out, indent+"deny all;")
	}
	return out
}

// renderProxyPass returns the proxy_pass line for a mapping.
func renderProxyPass(m model.Mapping, path, indent string) string {
	if strings.TrimSpace(m.ProxyPassURL) != "" {
		return indent + "proxy_pass " + sanitize(m.ProxyPassURL) + ";"
	}
	// A trailing slash on the root location keeps path normalization right
	// for backends such as GitLab.
	slash := ""
	if path == "/" {
		slash = "/"
	}
	return fmt.Sprintf("%sproxy_pass http://%s:%d%s;", indent, sanitize(m.UpstreamHost), m.UpstreamPort, slash)
}

// mapHeader starts every mapping's block.
func mapHeader(name string) []string {
	return []string{"  # mapping: " + sanitize(name), "  server {"}
}

// renderTCP renders a stream server forwarding a TCP port.
func renderTCP(m model.Mapping, whitelist []string) []string {
	listen := safePort(m.ListenPort)
	if listen == 0 {
		return nil
	}
	out := mapHeader(m.Name)
	out = append(out,
		fmt.Sprintf("    listen %d;", listen))
	out = append(out, renderWhitelist(whitelist, "    ")...)
	out = append(out, fmt.Sprintf("    proxy_pass %s:%d;", sanitize(m.UpstreamHost), m.UpstreamPort))
	out = append(out, renderExtra(m.ExtraDirectives, "    ")...)
	out = append(out, "  }")
	return out
}

// renderUDP renders a stream server forwarding a UDP port.
func renderUDP(m model.Mapping, whitelist []string) []string {
	listen := safePort(m.ListenPort)
	if listen == 0 {
		return nil
	}
	out := mapHeader(m.Name)
	out = append(out, fmt.Sprintf("    listen %d udp;", listen))
	out = append(out, renderWhitelist(whitelist, "    ")...)
	out = append(out, fmt.Sprintf("    proxy_pass %s:%d;", sanitize(m.UpstreamHost), m.UpstreamPort))
	out = append(out, renderExtra(m.ExtraDirectives, "    ")...)
	out = append(out, "  }")
	return out
}

// renderHTTP renders a plain HTTP reverse proxy.
func renderHTTP(m model.Mapping, whitelist []string) []string {
	port := safePort(m.ListenPort)
	if port == 0 {
		port = 80
	}
	path := sanitize(m.ListenPath)
	if path == "" {
		path = "/"
	}
	out := mapHeader(m.Name)
	out = append(out, fmt.Sprintf("    listen %d;", port))
	if names := sanitize(m.ServerNames); names != "" {
		out = append(out, "    server_name "+names+";")
	}
	out = append(out, "    location "+path+" {")
	out = append(out, renderWhitelist(whitelist, "      ")...)
	out = append(out, renderProxyPass(m, path, "      "))
	out = append(out, renderProxyDirectives(m, "      ")...)
	out = append(out, renderExtra(m.ExtraDirectives, "      ")...)
	out = append(out, "    }", "  }")
	return out
}

// renderHTTPS renders a TLS reverse proxy, with an optional HTTP redirect.
func renderHTTPS(m model.Mapping, whitelist []string) []string {
	path := sanitize(m.ListenPath)
	if path == "" {
		path = "/"
	}
	names := sanitize(m.ServerNames)

	var out []string
	out = append(out, "  # mapping: "+sanitize(m.Name))

	// The redirect block takes the whitelist too: without it port 80 would
	// bypass the access control the TLS block enforces.
	if m.RedirectHTTP && names != "" {
		out = append(out, "  server {", "    listen 80;", "    server_name "+names+";")
		out = append(out, renderWhitelist(whitelist, "    ")...)
		out = append(out, "    return 301 https://$host$request_uri;", "  }", "")
	}

	// The TLS port is the mapping's own: 443 only when the user left it
	// empty, the same default the form and the engine apply.
	port := m.ListenPort
	if port == 0 {
		port = 443
	}
	out = append(out, "  server {", fmt.Sprintf("    listen %d ssl;", port))
	if names != "" {
		out = append(out, "    server_name "+names+";")
	}
	if m.TLSCertPath != "" {
		out = append(out, "    ssl_certificate "+sanitize(m.TLSCertPath)+";")
	}
	if m.TLSKeyPath != "" {
		out = append(out, "    ssl_certificate_key "+sanitize(m.TLSKeyPath)+";")
	}
	out = append(out, "    location "+path+" {")
	out = append(out, renderWhitelist(whitelist, "      ")...)
	out = append(out, renderProxyPass(m, path, "      "))
	out = append(out, renderProxyDirectives(m, "      ")...)
	out = append(out, renderExtra(m.ExtraDirectives, "      ")...)
	out = append(out, "    }", "  }")
	return out
}

// Generate builds a complete nginx.conf for the enabled mappings.
//
// Every path in it is relative to nginx's prefix, which the engine passes
// with -p, so that neither backslashes nor non-ASCII characters in the
// install path reach nginx.
func Generate(mappings []model.Mapping, opts GenOptions) string {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	var tcp, udp, http, https []model.Mapping
	for _, m := range mappings {
		if !m.Enabled {
			continue
		}
		switch m.Protocol {
		case model.TCP:
			tcp = append(tcp, m)
		case model.UDP:
			udp = append(udp, m)
		case model.HTTP:
			http = append(http, m)
		case model.HTTPS:
			https = append(https, m)
		}
	}
	hasStream := len(tcp) > 0 || len(udp) > 0
	hasHTTP := len(http) > 0 || len(https) > 0

	var lines []string
	lines = append(lines,
		"# 由 JumpBoard 自动生成，请勿手动编辑",
		"# 生成时间: "+now.UTC().Format("2006-01-02T15:04:05.000Z"),
		"worker_processes 1;",
		"error_log logs/error.log warn;",
		"pid temp/nginx.pid;",
		"",
		"events {",
		"  worker_connections 1024;",
		"}",
		"",
	)

	if hasHTTP {
		lines = append(lines,
			"http {",
			"  map $http_upgrade $connection_upgrade {",
			"    default upgrade;",
			`    ""      close;`,
			"  }",
			"  proxy_http_version 1.1;",
			"  client_max_body_size 0;",
			"  proxy_read_timeout 300s;",
			"  proxy_buffering off;",
			"  access_log logs/access.log;",
			"  client_body_temp_path temp/client_body;",
			"  proxy_temp_path temp/proxy;",
			"  include mime.types;",
		)
		if len(http) > 0 {
			lines = append(lines, "  # ---------- HTTP server 块 ----------")
			for _, m := range http {
				lines = append(lines, renderHTTP(m, opts.WhitelistIPs)...)
			}
		}
		if len(https) > 0 {
			lines = append(lines, "  # ---------- HTTPS server 块 ----------")
			for _, m := range https {
				lines = append(lines, renderHTTPS(m, opts.WhitelistIPs)...)
			}
		}
		lines = append(lines, "}", "")
	}

	if hasStream {
		lines = append(lines,
			"stream {",
			`  log_format stream '$remote_addr [$time_local] $protocol $status $bytes_sent';`,
			"  access_log logs/stream_access.log stream;",
		)
		for _, m := range tcp {
			lines = append(lines, renderTCP(m, opts.WhitelistIPs)...)
		}
		for _, m := range udp {
			lines = append(lines, renderUDP(m, opts.WhitelistIPs)...)
		}
		lines = append(lines, "}", "")
	}

	return strings.Join(lines, "\n")
}

// HumanizeListen renders a mapping's public side for the UI: ":9000" for
// stream mappings, "git.example.com:80" for HTTP ones.
func HumanizeListen(m model.Mapping) string {
	port := m.ListenPort
	if port == 0 && m.IsHTTP() {
		port = 80
	}
	if m.Protocol == model.HTTPS {
		port = 443
	}
	if m.ServerNames != "" {
		return m.ServerNames + ":" + strconv.Itoa(port)
	}
	return ":" + strconv.Itoa(port)
}
