package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
	"github.com/lianxin255/jumpboard/internal/store"
)

// exportVersion is the format's version, matched by Import. Version 2 is
// what the Electron build wrote, so its backups import unchanged.
const exportVersion = 2

// Payload is an exported backup.
type Payload struct {
	Version    int              `json:"version"`
	ExportedAt int64            `json:"exportedAt"`
	Hosts      []hostOut        `json:"hosts"`
	Mappings   []model.Mapping  `json:"mappings"`
	Snapshots  []model.Snapshot `json:"snapshots,omitempty"`
	Logs       []model.LogEntry `json:"logs,omitempty"`
}

// hostOut mirrors the host record of the Electron build, which keeps the
// export readable by the old version. Remote hosts export without a
// password.
type hostOut struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsDefault    int    `json:"isDefault"`
	LocalConfig  any    `json:"localConfig"`
	RemoteConfig any    `json:"remoteConfig"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
}

// Export returns a backup of one workspace's rules, everything the app
// can restore. Snapshots and logs are the app's history, not a scene's,
// so they ride along whatever scene is exported.
func Export(s *store.Store, ws string) (Payload, error) {
	name, err := s.HostName()
	if err != nil {
		return Payload{}, err
	}
	mappings, err := s.ListMappings(ws)
	if err != nil {
		return Payload{}, err
	}
	snaps, err := s.ListSnapshots(200)
	if err != nil {
		return Payload{}, err
	}
	logs, err := s.ListLogs(2000)
	if err != nil {
		return Payload{}, err
	}
	return Payload{
		Version:    exportVersion,
		ExportedAt: nowMillis(),
		Hosts: []hostOut{{
			ID: "local", Name: name, Type: "local", IsDefault: 1,
			LocalConfig: map[string]string{"nginxPath": "", "confDir": "", "logDir": ""},
			Status:      "unknown",
		}},
		Mappings:  mappings,
		Snapshots: snaps,
		Logs:      logs,
	}, nil
}

// ImportResult reports what an import did.
type ImportResult struct {
	MappingsImported  int `json:"mappingsImported"`
	SnapshotsImported int `json:"snapshotsImported"`
	Skipped           int `json:"skipped"`
}

// Import replaces one workspace's rules with a backup's. A backup has no
// scene of its own — the format predates them — so the scene it lands in
// is the caller's choice: the active one for a plain 导入备份, a fresh one
// for 导入为新场景.
//
// A backup is a file the user may have been sent, so nothing in it is
// trusted: each mapping is validated and sanitized the same way a value
// typed into the form is, and a record that fails is skipped rather than
// aborting the whole import.
func Import(s *store.Store, ws string, data []byte) (ImportResult, error) {
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return ImportResult{}, fmt.Errorf("备份文件不是合法的 JSON: %w", err)
	}
	if p.Version != exportVersion && p.Version != 1 {
		return ImportResult{}, fmt.Errorf("备份版本不匹配：期望 %d，得到 %d", exportVersion, p.Version)
	}

	clean := make([]model.Mapping, 0, len(p.Mappings))
	skipped := 0
	for _, raw := range p.Mappings {
		m, ok := sanitizeImported(raw)
		if !ok {
			skipped++
			continue
		}
		clean = append(clean, m)
	}
	// A backup that carries mappings but none the sanitizer will take
	// promises a replacement and would deliver an empty rule set: the
	// replace is refused whole, before anything is deleted, and the rules
	// it would have destroyed keep standing. A backup with no mappings at
	// all is a deliberate wipe and stays one.
	if len(p.Mappings) > 0 && len(clean) == 0 {
		return ImportResult{}, fmt.Errorf("%w：%d 条映射都缺少可用的转发信息，现有规则已保留", ErrEmptyBackup, len(p.Mappings))
	}
	if err := s.ReplaceAllMappings(ws, clean); err != nil {
		return ImportResult{}, err
	}

	imported := 0
	for _, sn := range p.Snapshots {
		if sn.Content == "" {
			continue
		}
		if err := s.AddSnapshot(sn.Content, firstNonEmpty(sn.Reason, "导入")); err != nil {
			return ImportResult{}, err
		}
		imported++
	}
	_ = s.TrimSnapshots(50)

	return ImportResult{MappingsImported: len(clean), SnapshotsImported: imported, Skipped: skipped}, nil
}

// ImportNginxConf merges the server blocks of an nginx.conf into a
// workspace as new mappings.
//
// This is deliberately not what Import does: a backup replaces the whole
// rule set because it is the rule set, while a hand-written or
// foreign nginx.conf is additions to what is already there. Each parsed
// block goes through the same sanitizer a backup's mapping does, so a
// dropped conf is no more trusted than a dropped file. It returns how
// many mappings were added; blocks the sanitizer refuses are counted as
// skipped rather than failing the import.
func ImportNginxConf(s *store.Store, ws string, data []byte) (added, skipped int, err error) {
	parsed, err := nginx.Parse(string(data))
	if err != nil {
		return 0, 0, fmt.Errorf("解析 nginx 配置失败: %w", err)
	}
	if len(parsed) == 0 {
		return 0, 0, errors.New("配置里没有可识别的 server 块")
	}
	for _, raw := range parsed {
		m, ok := sanitizeImported(raw)
		if !ok {
			skipped++
			continue
		}
		if _, err := s.CreateMapping(ws, m); err != nil {
			return added, skipped, err
		}
		added++
	}
	if added == 0 {
		return 0, skipped, errors.New("配置里的 server 块都缺少可用的转发信息")
	}
	return added, skipped, nil
}

// sanitizeImported validates and cleans one mapping from a backup.
func sanitizeImported(m model.Mapping) (model.Mapping, bool) {
	if !m.Protocol.Valid() {
		return model.Mapping{}, false
	}
	if m.ListenPort != 0 && !model.ValidPort(m.ListenPort) {
		return model.Mapping{}, false
	}
	if m.UpstreamPort != 0 && !model.ValidPort(m.UpstreamPort) {
		return model.Mapping{}, false
	}
	out := model.Mapping{
		ID:           "",
		Name:         truncate(model.StripInject(m.Name), 100),
		Group:        truncate(model.StripInject(m.Group), 50),
		Enabled:      m.Enabled,
		Protocol:     m.Protocol,
		ListenPort:   m.ListenPort,
		ServerNames:  truncate(model.StripInject(m.ServerNames), 500),
		ListenPath:   truncate(model.StripInject(m.ListenPath), 200),
		UpstreamHost: truncate(model.StripInject(m.UpstreamHost), 255),
		UpstreamPort: m.UpstreamPort,
		TLSCertPath:  truncate(model.StripInject(m.TLSCertPath), 500),
		TLSKeyPath:   truncate(model.StripInject(m.TLSKeyPath), 500),
		RedirectHTTP: m.RedirectHTTP,
		// Extra directives are the one field a user may write raw nginx
		// in, by design; they are kept but never interpreted by the app.
		ExtraDirectives:         m.ExtraDirectives,
		ProxyMode:               m.ProxyMode,
		ProxyHeaders:            model.StripProxyHeaders(m.ProxyHeaders),
		ProxyPassURL:            m.ProxyPassURL,
		HealthCheckEnabled:      m.HealthCheckEnabled,
		HealthCheckIntervalSec:  clampInterval(m.HealthCheckIntervalSec),
		HealthCheckTargetPath:   truncate(model.StripInject(m.HealthCheckTargetPath), 500),
		HealthCheckExpectedCode: m.HealthCheckExpectedCode,
		HealthStatus:            model.HealthUnknown,
	}
	if !out.ProxyMode.Valid() {
		out.ProxyMode = model.ProxyStandard
	}
	if out.ProxyPassURL != "" && !model.ValidProxyPassURL(out.ProxyPassURL) {
		out.ProxyPassURL = ""
	}
	if out.Name == "" {
		out.Name = "导入映射"
	}
	if out.ListenPath == "" {
		out.ListenPath = "/"
	}
	if out.UpstreamHost == "" {
		return model.Mapping{}, false
	}
	return out, true
}

// truncate limits a string to n bytes without splitting a rune.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// clampInterval keeps a health-check interval sane.
func clampInterval(sec int) int {
	if sec < 5 {
		return 30
	}
	if sec > 3600 {
		return 3600
	}
	return sec
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// nowMillis is the timestamp the app stores.
func nowMillis() int64 { return timeNowMillis() }

// ErrEmptyBackup is returned for a backup from which nothing can be
// imported: it carries mappings, but none survives the sanitizer.
var ErrEmptyBackup = errors.New("备份文件里没有可导入的映射")
