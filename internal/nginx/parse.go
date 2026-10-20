package nginx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// Parse reads the mappings out of a generated configuration.
//
// Snapshots hold nginx.conf text, and rolling one back has to restore the
// rules it describes, so the generator's own output is read back. Only the
// shapes the generator writes are understood: a block it did not write is
// reported rather than guessed at.
func Parse(conf string) ([]model.Mapping, error) {
	var (
		out             []model.Mapping
		current         *model.Mapping
		inHTTP          bool
		inTLS           bool
		depth           int
		pendingName     string
		redirect        bool
		pendingRedirect bool
	)

	for _, raw := range strings.Split(conf, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "# mapping: ") {
			pendingName = strings.TrimSpace(strings.TrimPrefix(line, "# mapping: "))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case line == "http {":
			inHTTP = true
			continue
		case line == "stream {":
			inHTTP = false
			continue
		case line == "server {":
			m := model.Mapping{Enabled: true, ListenPath: "/", ProxyMode: model.ProxyStandard}
			if pendingName != "" {
				m.Name = pendingName
				pendingName = ""
			}
			if pendingRedirect {
				// The redirect block named the rule it belongs to.
				m.RedirectHTTP = true
				pendingRedirect = false
			}
			current = &m
			inTLS, depth = false, 0
			continue
		case line == "}":
			if current != nil && depth == 0 {
				// The generated HTTP-to-HTTPS redirect is a server block of
				// its own; it belongs to the TLS rule that follows it.
				if redirect {
					// The redirect block carried the rule's name; the TLS
					// block that follows has no comment of its own, so the
					// name is put back for it.
					pendingName = current.Name
					pendingRedirect = true
					current, redirect = nil, false
					continue
				}
				finish(current, inHTTP)
				if current.ListenPort != 0 {
					out = append(out, *current)
				}
				current = nil
				continue
			}
			if depth > 0 {
				depth--
			}
			continue
		case line == "location /" || strings.HasPrefix(line, "location "):
			if current != nil {
				path := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "location ")), " {")
				if path != "/" {
					current.ListenPath = path
				}
				depth++
			}
			continue
		case strings.HasPrefix(line, "return 301"):
			if current != nil {
				current.RedirectHTTP = true
				redirect = true
			}
			continue
		}
		if current == nil {
			continue
		}
		applyDirective(current, line, inHTTP, &inTLS)
	}
	if current != nil {
		return nil, fmt.Errorf("配置在最后一个 server 块中间结束")
	}
	return out, nil
}

// applyDirective reads one directive into the mapping being built.
//
// Whether the block sits in the http or the stream section decides what a
// bare "listen 8080" means: an HTTP server in one, a TCP forward in the
// other, exactly as nginx reads it.
func applyDirective(m *model.Mapping, line string, inHTTP bool, inTLS *bool) {
	field := strings.TrimSuffix(line, ";")
	parts := strings.Fields(field)
	if len(parts) < 2 {
		if field == "" {
			return
		}
		return
	}
	switch parts[0] {
	case "listen":
		if len(parts) >= 3 && parts[2] == "ssl" {
			m.Protocol = model.HTTPS
			*inTLS = true
		} else if len(parts) >= 3 && parts[2] == "udp" {
			m.Protocol = model.UDP
		}
		m.ListenPort = atoiOr(parts[1], 0)
		if m.Protocol == "" {
			if inHTTP {
				m.Protocol = model.HTTP
			} else {
				m.Protocol = model.TCP
			}
		}
	case "server_name":
		if len(parts) >= 2 {
			m.ServerNames = strings.Join(parts[1:], " ")
		}
	case "ssl_certificate":
		m.TLSCertPath = parts[1]
	case "ssl_certificate_key":
		m.TLSKeyPath = parts[1]
	case "proxy_pass":
		readProxyPass(m, parts[1])
		m.Enabled = true
		if m.Protocol == "" {
			m.Protocol = model.HTTP
		}
	case "proxy_set_header":
		if m.ProxyHeaders != "" {
			m.ProxyHeaders += "\n"
		}
		m.ProxyHeaders += field + ";"
	case "proxy_timeout", "proxy_connect_timeout", "proxy_read_timeout", "proxy_http_version", "proxy_buffering":
		if m.ExtraDirectives != "" {
			m.ExtraDirectives += "\n"
		}
		m.ExtraDirectives += field + ";"
	}
	// A block in the http section is an HTTP server even before its
	// proxy_pass says so.
	if m.Protocol == "" && inHTTP {
		m.Protocol = model.HTTP
	}
}

// readProxyPass reads the upstream out of a proxy_pass value.
func readProxyPass(m *model.Mapping, value string) {
	value = strings.TrimSuffix(value, "/")
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		rest := strings.TrimPrefix(strings.TrimPrefix(value, "https://"), "http://")
		if slash := strings.Index(rest, "/"); slash >= 0 {
			rest = rest[:slash]
		}
		m.UpstreamHost, m.UpstreamPort = splitHostPort(rest, 80)
		if m.Protocol == "" {
			m.Protocol = model.HTTP
		}
		return
	}
	m.UpstreamHost, m.UpstreamPort = splitHostPort(value, 0)
	if m.Protocol == "" || m.Protocol == model.HTTP || m.Protocol == model.HTTPS {
		m.Protocol = model.TCP
	}
}

// splitHostPort splits host:port, with a default for a missing port.
func splitHostPort(s string, def int) (string, int) {
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[:i], atoiOr(s[i+1:], def)
	}
	return s, def
}

// atoiOr parses an int, returning def when it does not.
func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// finish fills what a parsed block leaves open.
func finish(m *model.Mapping, inHTTP bool) {
	if m.ListenPort == 0 && (m.Protocol == model.HTTP || m.Protocol == model.HTTPS) {
		if m.Protocol == model.HTTPS {
			m.ListenPort = 443
		} else {
			m.ListenPort = 80
		}
	}
	if m.Protocol == "" {
		m.Protocol = model.TCP
	}
	if m.Name == "" {
		m.Name = m.ServerNames
		if m.Name == "" {
			m.Name = listenSuffix(m.ListenPort)
		}
	}
	if !m.Protocol.IsStream() && m.ListenPath == "" {
		m.ListenPath = "/"
	}
	if m.ProxyMode == "" {
		m.ProxyMode = model.ProxyStandard
	}
}

// listenSuffix names a rule after its port, for a block with no other name.
func listenSuffix(port int) string { return ":" + strconv.Itoa(port) }
