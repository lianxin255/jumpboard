package appt

import (
	"fmt"
	"strings"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// FormValues is what the mapping form holds while it is open.
//
// Everything is a string: a text input edits a string, and the form can
// then show "not a number" where the user left a letter in a port, instead
// of silently clearing it.
type FormValues struct {
	Name         string
	Protocol     string // TCP / UDP / HTTP / HTTPS
	ListenPort   string
	ServerNames  string
	ListenPath   string
	UpstreamHost string
	UpstreamPort string

	TLSCertPath  string
	TLSKeyPath   string
	RedirectHTTP bool

	ExtraDirectives string
	ProxyMode       string // 标准反代 / 精简代理 / 完全自定义
	ProxyHeaders    string
	ProxyPassURL    string

	HealthCheckEnabled  bool
	HealthCheckPath     string
	HealthCheckCode     string
	HealthCheckInterval string
}

// protocolFromLabel maps the dropdown's text back to a protocol.
func protocolFromLabel(label string) model.Protocol {
	switch label {
	case "UDP":
		return model.UDP
	case "HTTP":
		return model.HTTP
	case "HTTPS":
		return model.HTTPS
	}
	return model.TCP
}

// proxyModeFromLabel maps the dropdown's text back to a mode.
func proxyModeFromLabel(label string) model.ProxyMode {
	switch label {
	case "精简代理":
		return model.ProxyMinimal
	case "完全自定义":
		return model.ProxyCustom
	}
	return model.ProxyStandard
}

// NewForm returns the values of an empty form.
func NewForm() FormValues {
	return FormValues{
		Protocol:            "TCP",
		ListenPath:          "/",
		ProxyMode:           "标准反代",
		HealthCheckInterval: "30",
		HealthCheckCode:     "200",
	}
}

// FormFromMapping fills the form from an existing mapping.
func FormFromMapping(m model.Mapping) FormValues {
	f := NewForm()
	f.Name = m.Name
	f.Protocol = m.Protocol.Label()
	f.ListenPort = itoa(m.ListenPort)
	f.ServerNames = m.ServerNames
	f.ListenPath = m.ListenPath
	f.UpstreamHost = m.UpstreamHost
	f.UpstreamPort = itoa(m.UpstreamPort)
	f.TLSCertPath = m.TLSCertPath
	f.TLSKeyPath = m.TLSKeyPath
	f.RedirectHTTP = m.RedirectHTTP
	f.ExtraDirectives = m.ExtraDirectives
	f.ProxyMode = m.ProxyMode.Label()
	f.ProxyHeaders = m.ProxyHeaders
	f.ProxyPassURL = m.ProxyPassURL
	f.HealthCheckEnabled = m.HealthCheckEnabled
	f.HealthCheckPath = m.HealthCheckTargetPath
	f.HealthCheckCode = itoa(m.HealthCheckExpectedCode)
	if m.HealthCheckIntervalSec > 0 {
		f.HealthCheckInterval = itoa(m.HealthCheckIntervalSec)
	}
	if f.ListenPath == "" {
		f.ListenPath = "/"
	}
	return f
}

// FormFromTemplate fills the form from a template.
func FormFromTemplate(t model.Template) FormValues {
	f := NewForm()
	f.Protocol = t.Mapping.Protocol.Label()
	f.ListenPort = itoa(t.Mapping.ListenPort)
	f.UpstreamHost = t.Mapping.UpstreamHost
	f.UpstreamPort = itoa(t.Mapping.UpstreamPort)
	f.ServerNames = t.Mapping.ServerNames
	f.ListenPath = t.Mapping.ListenPath
	if t.Mapping.ProxyMode.Valid() && t.Mapping.ProxyMode != "" {
		f.ProxyMode = t.Mapping.ProxyMode.Label()
	}
	f.ExtraDirectives = t.ExtraDirectives
	if f.ListenPath == "" {
		f.ListenPath = "/"
	}
	return f
}

// Validate checks the form and returns the mapping it describes, plus the
// fields that are wrong.
//
// The errors are returned rather than stored on the form: the view writes
// its own values back at the end of every frame, which would erase them
// before the user could read them.
//
// The rules mirror the ones the store and the generator apply, so a value
// that passes here cannot produce a configuration nginx rejects for reasons
// the user could have seen in advance.
func (f *FormValues) Validate() (model.Mapping, map[string]string, bool) {
	errs := map[string]string{}
	proto := protocolFromLabel(f.Protocol)
	m := model.Mapping{
		Name:                  strings.TrimSpace(f.Name),
		Protocol:              proto,
		Enabled:               true,
		ServerNames:           strings.TrimSpace(f.ServerNames),
		ListenPath:            strings.TrimSpace(f.ListenPath),
		UpstreamHost:          strings.TrimSpace(f.UpstreamHost),
		TLSCertPath:           strings.TrimSpace(f.TLSCertPath),
		TLSKeyPath:            strings.TrimSpace(f.TLSKeyPath),
		RedirectHTTP:          f.RedirectHTTP,
		ExtraDirectives:       f.ExtraDirectives,
		ProxyMode:             proxyModeFromLabel(f.ProxyMode),
		ProxyHeaders:          f.ProxyHeaders,
		ProxyPassURL:          strings.TrimSpace(f.ProxyPassURL),
		HealthCheckEnabled:    f.HealthCheckEnabled,
		HealthCheckTargetPath: strings.TrimSpace(f.HealthCheckPath),
	}
	if m.ListenPath == "" {
		m.ListenPath = "/"
	}

	if m.Name == "" {
		errs["Name"] = "请填写名称"
	}
	if m.UpstreamHost == "" {
		errs["UpstreamHost"] = "请填写上游主机"
	}

	if n, err := atoi(f.UpstreamPort); err != nil || !model.ValidPort(n) {
		errs["UpstreamPort"] = "端口应为 1-65535"
	} else {
		m.UpstreamPort = n
	}

	// HTTP and HTTPS default to 80 and 443, so an empty listen port is
	// fine for them and an error for stream mappings, which have none.
	if strings.TrimSpace(f.ListenPort) == "" {
		if proto.IsStream() {
			errs["ListenPort"] = "请填写监听端口"
		}
	} else if n, err := atoi(f.ListenPort); err != nil || !model.ValidPort(n) {
		errs["ListenPort"] = "端口应为 1-65535"
	} else {
		m.ListenPort = n
	}

	if proto == model.HTTPS {
		if m.TLSCertPath == "" || m.TLSKeyPath == "" {
			errs["TLSCertPath"] = "HTTPS 需要证书和私钥路径"
		}
		if m.ServerNames == "" {
			errs["ServerNames"] = "HTTPS 需要 server_name"
		}
	}

	if m.ProxyPassURL != "" && !model.ValidProxyPassURL(m.ProxyPassURL) {
		errs["ProxyPassURL"] = "必须以 http:// 或 https:// 开头，且不含 ; { } # 换行"
	}

	if m.HealthCheckEnabled {
		if n, err := atoi(f.HealthCheckInterval); err != nil || n < 5 || n > 3600 {
			errs["HealthCheckInterval"] = "间隔应为 5-3600 秒"
		} else {
			m.HealthCheckIntervalSec = n
		}
		if strings.TrimSpace(f.HealthCheckCode) != "" {
			if n, err := atoi(f.HealthCheckCode); err != nil || n < 100 || n > 599 {
				errs["HealthCheckCode"] = "状态码应为 100-599"
			} else {
				m.HealthCheckExpectedCode = n
			}
		}
	} else {
		m.HealthCheckIntervalSec = 30
	}

	return m, errs, len(errs) == 0
}

// ListenPortSuggest returns a free port the form can suggest, starting
// from the protocol's usual one.
func (f FormValues) ListenPortSuggest() int {
	proto := protocolFromLabel(f.Protocol)
	switch proto {
	case model.HTTP:
		return 8080
	case model.HTTPS:
		return 8443
	}
	return 9000
}

func itoa(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func atoi(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			return 0, fmt.Errorf("too large")
		}
	}
	return n, nil
}
