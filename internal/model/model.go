// Package model holds the app's plain data types. They are shared by the
// store, the nginx configuration generator and the UI, and carry no
// behavior beyond rendering themselves.
package model

// Protocol is a forwarding protocol.
type Protocol string

// Protocols a mapping can use.
const (
	TCP   Protocol = "tcp"
	UDP   Protocol = "udp"
	HTTP  Protocol = "http"
	HTTPS Protocol = "https"
)

// Valid reports whether p is one of the protocols.
func (p Protocol) Valid() bool {
	switch p {
	case TCP, UDP, HTTP, HTTPS:
		return true
	}
	return false
}

// Label returns the protocol as the UI shows it.
func (p Protocol) Label() string {
	switch p {
	case TCP:
		return "TCP"
	case UDP:
		return "UDP"
	case HTTP:
		return "HTTP"
	case HTTPS:
		return "HTTPS"
	}
	return string(p)
}

// IsStream reports whether p forwards at the stream (TCP/UDP) level.
func (p Protocol) IsStream() bool { return p == TCP || p == UDP }

// ProxyMode is how an HTTP mapping proxies.
type ProxyMode string

// Proxy modes.
const (
	// ProxyStandard adds Host, X-Real-IP, X-Forwarded-*, WebSocket headers.
	ProxyStandard ProxyMode = "standard"
	// ProxyMinimal passes through with no headers of its own.
	ProxyMinimal ProxyMode = "minimal"
	// ProxyCustom uses the user's own header directives.
	ProxyCustom ProxyMode = "custom"
)

// Valid reports whether m is one of the modes.
func (m ProxyMode) Valid() bool {
	return m == ProxyStandard || m == ProxyMinimal || m == ProxyCustom
}

// Label returns the mode as the UI shows it.
func (m ProxyMode) Label() string {
	switch m {
	case ProxyStandard:
		return "标准反代"
	case ProxyMinimal:
		return "精简代理"
	case ProxyCustom:
		return "完全自定义"
	}
	return string(m)
}

// HealthStatus is what the last health check found.
type HealthStatus string

// Health statuses.
const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthChecking  HealthStatus = "checking"
	// HealthOff is a mapping whose checks are turned off.
	HealthOff HealthStatus = "off"
)

// Mapping is one forwarding rule.
type Mapping struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Group is an optional short free-text label, such as "数据库" or
	// "内网"; empty means ungrouped.
	Group    string   `json:"group"`
	Enabled  bool     `json:"enabled"`
	Protocol Protocol `json:"protocol"`

	// ListenPort is the port this machine listens on.
	ListenPort int `json:"listenPort"`
	// ServerNames and ListenPath apply to HTTP and HTTPS.
	ServerNames string `json:"serverNames"`
	ListenPath  string `json:"listenPath"`

	UpstreamHost string `json:"upstreamHost"`
	UpstreamPort int    `json:"upstreamPort"`

	TLSCertPath string `json:"tlsCertPath"`
	TLSKeyPath  string `json:"tlsKeyPath"`

	RedirectHTTP bool `json:"redirectHttp"`

	// ExtraDirectives are appended to the mapping's server or location block.
	ExtraDirectives string `json:"extraDirectives"`

	ProxyMode    ProxyMode `json:"proxyMode"`
	ProxyHeaders string    `json:"proxyHeaders"`
	ProxyPassURL string    `json:"proxyPassUrl"`

	HealthCheckEnabled      bool `json:"healthCheckEnabled"`
	HealthCheckIntervalSec  int  `json:"healthCheckIntervalSec"`
	HealthCheckTargetPath   string
	HealthCheckExpectedCode int

	HealthStatus  HealthStatus `json:"healthStatus"`
	LastCheckedAt int64        `json:"lastCheckedAt"`

	SortOrder int   `json:"sortOrder"`
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// IsHTTP reports whether the mapping is proxied at the HTTP level.
func (m Mapping) IsHTTP() bool { return m.Protocol == HTTP || m.Protocol == HTTPS }

// Workspace is one named scene: a set of mappings of its own, so 公司 and
// 家里 keep their rules apart and a switch is one click instead of an
// export and an import.
type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}

// Snapshot is one saved nginx configuration.
type Snapshot struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"createdAt"`
}

// LogLevel is the severity of an app log entry.
type LogLevel string

// Log levels.
const (
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// LogSource says what wrote a log entry.
type LogSource string

// Log sources.
const (
	SourceApp LogSource = "app"
)

// LogEntry is one line of the operation log.
type LogEntry struct {
	ID        int64     `json:"id"`
	Level     LogLevel  `json:"level"`
	Source    LogSource `json:"source"`
	Message   string    `json:"message"`
	Timestamp int64     `json:"timestamp"`
}

// WhitelistEntry is one allowed IP or CIDR.
type WhitelistEntry struct {
	ID        int64  `json:"id"`
	IP        string `json:"ip"`
	Note      string `json:"note"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
}

// Template is a prefilled mapping draft shipped with the app or saved by
// the user.
type Template struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Category    string `yaml:"category" json:"category"`
	Description string `yaml:"description" json:"description"`
	Builtin     bool   `yaml:"-" json:"builtin"`

	Mapping TemplateMapping `yaml:"mapping" json:"mapping"`
	// ExtraDirectives are appended to the generated block.
	ExtraDirectives string `yaml:"extraDirectives" json:"extraDirectives"`
}

// TemplateMapping is the part of a mapping a template prefills.
type TemplateMapping struct {
	Protocol     Protocol  `yaml:"protocol" json:"protocol"`
	ListenPort   int       `yaml:"listenPort" json:"listenPort"`
	UpstreamHost string    `yaml:"upstreamHost" json:"upstreamHost"`
	UpstreamPort int       `yaml:"upstreamPort" json:"upstreamPort"`
	ServerNames  string    `yaml:"serverNames" json:"serverNames"`
	ListenPath   string    `yaml:"listenPath" json:"listenPath"`
	ProxyMode    ProxyMode `yaml:"proxyMode" json:"proxyMode"`
}
