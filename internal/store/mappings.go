package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lianxin255/jumpboard/internal/model"
)

// localHostID is the single host a local-only build keeps. The mappings
// table has a NOT NULL host_id, so one row holds it.
const localHostID = "local"

// EnsureLocalHost creates the local host row the mappings belong to.
func (s *Store) EnsureLocalHost() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE id = ?`, localHostID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		// A database from the Electron build may have no default host;
		// make this one the default.
		_, err := s.db.Exec(`UPDATE hosts SET is_default = ? WHERE id = ?`, 1, localHostID)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
		VALUES (?, ?, 'local', 1, ?, NULL, 'unknown', ?)`,
		localHostID, "本机网关", `{"nginxPath":"","confDir":"","logDir":""}`, nowMillis())
	return err
}

// HostName returns the local host's display name.
func (s *Store) HostName() (string, error) {
	var name string
	err := s.db.QueryRow(`SELECT name FROM hosts WHERE id = ?`, localHostID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// SetHostName renames the local host.
func (s *Store) SetHostName(name string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE hosts SET name = ? WHERE id = ?`, name, localHostID)
	return err
}

// mappingColumns is the SELECT list, in scanMapping's order. Inserts name
// host_id as well, which a local-only build always sets to localHostID.
const mappingColumns = `id, name, group_name, enabled, protocol, listen_port, server_names, listen_path,
	upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
	proxy_mode, proxy_headers, proxy_pass_url,
	health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
	health_status, last_checked_at, sort_order, created_at, updated_at`

// insertMapping writes one mapping, with the local host.
const insertMapping = `INSERT INTO mappings (id, host_id, name, group_name, enabled, protocol, listen_port,
	server_names, listen_path, upstream_host, upstream_port, tls_cert_path, tls_key_path,
	redirect_http, extra_directives, proxy_mode, proxy_headers, proxy_pass_url,
	health_check_enabled, health_check_interval_sec, health_check_target_path,
	health_check_expected_status, health_status, last_checked_at, sort_order, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// scanMapping reads one mapping row.
func scanMapping(sc interface{ Scan(...any) error }) (model.Mapping, error) {
	var (
		m                        model.Mapping
		enabled, redirect        int
		listenPort, upstreamPort sql.NullInt64
		serverNames, listenPath  sql.NullString
		cert, key, extra         sql.NullString
		proxyMode, headers, pass sql.NullString
		hcEnabled, hcInterval    int
		hcPath                   sql.NullString
		hcCode                   sql.NullInt64
		health                   string
		lastChecked              sql.NullInt64
	)
	err := sc.Scan(&m.ID, &m.Name, &m.Group, &enabled, &m.Protocol, &listenPort, &serverNames, &listenPath,
		&m.UpstreamHost, &upstreamPort, &cert, &key, &redirect, &extra,
		&proxyMode, &headers, &pass,
		&hcEnabled, &hcInterval, &hcPath, &hcCode,
		&health, &lastChecked, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return model.Mapping{}, err
	}
	m.Enabled = enabled == 1
	m.RedirectHTTP = redirect == 1
	m.ListenPort = int(listenPort.Int64)
	m.UpstreamPort = int(upstreamPort.Int64)
	m.ServerNames = serverNames.String
	m.ListenPath = listenPath.String
	m.TLSCertPath = cert.String
	m.TLSKeyPath = key.String
	m.ExtraDirectives = extra.String
	m.ProxyMode = model.ProxyMode(proxyMode.String)
	if !m.ProxyMode.Valid() {
		m.ProxyMode = model.ProxyStandard
	}
	m.ProxyHeaders = headers.String
	m.ProxyPassURL = pass.String
	m.HealthCheckEnabled = hcEnabled == 1
	m.HealthCheckIntervalSec = hcInterval
	m.HealthCheckTargetPath = hcPath.String
	m.HealthCheckExpectedCode = int(hcCode.Int64)
	m.HealthStatus = model.HealthStatus(health)
	m.LastCheckedAt = lastChecked.Int64
	return m, nil
}

// ListMappings returns every mapping, in display order.
func (s *Store) ListMappings() ([]model.Mapping, error) {
	rows, err := s.db.Query(`SELECT ` + mappingColumns + ` FROM mappings ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Mapping
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListEnabledMappings returns the mappings a configuration is built from.
func (s *Store) ListEnabledMappings() ([]model.Mapping, error) {
	rows, err := s.db.Query(`SELECT ` + mappingColumns + ` FROM mappings WHERE enabled = 1 ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Mapping
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMapping returns a mapping by ID.
func (s *Store) GetMapping(id string) (model.Mapping, error) {
	row := s.db.QueryRow(`SELECT `+mappingColumns+` FROM mappings WHERE id = ?`, id)
	m, err := scanMapping(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Mapping{}, ErrNotFound
	}
	return m, err
}

// CreateMapping inserts a mapping and returns it with its ID and
// timestamps filled in.
func (s *Store) CreateMapping(m model.Mapping) (model.Mapping, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := nowMillis()
	if m.ID == "" {
		m.ID = newID()
	}
	m.CreatedAt, m.UpdatedAt = now, now
	if m.SortOrder == 0 {
		var max int
		if err := s.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM mappings`).Scan(&max); err != nil {
			return model.Mapping{}, err
		}
		m.SortOrder = max + 1
	}
	if m.ProxyMode == "" {
		m.ProxyMode = model.ProxyStandard
	}
	if m.HealthStatus == "" {
		m.HealthStatus = model.HealthUnknown
	}
	_, err := s.db.Exec(insertMapping,
		m.ID, localHostID, m.Name, m.Group, boolInt(m.Enabled), string(m.Protocol), m.ListenPort, m.ServerNames, m.ListenPath,
		m.UpstreamHost, m.UpstreamPort, m.TLSCertPath, m.TLSKeyPath, boolInt(m.RedirectHTTP), m.ExtraDirectives,
		string(m.ProxyMode), m.ProxyHeaders, m.ProxyPassURL,
		boolInt(m.HealthCheckEnabled), m.HealthCheckIntervalSec, m.HealthCheckTargetPath, m.HealthCheckExpectedCode,
		string(m.HealthStatus), nullInt(m.LastCheckedAt), m.SortOrder, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return model.Mapping{}, fmt.Errorf("inserting the mapping: %w", err)
	}
	return m, nil
}

// UpdateMapping replaces a mapping's fields.
func (s *Store) UpdateMapping(m model.Mapping) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	m.UpdatedAt = nowMillis()
	res, err := s.db.Exec(`UPDATE mappings SET
		name=?, group_name=?, enabled=?, protocol=?, listen_port=?, server_names=?, listen_path=?,
		upstream_host=?, upstream_port=?, tls_cert_path=?, tls_key_path=?, redirect_http=?,
		extra_directives=?, proxy_mode=?, proxy_headers=?, proxy_pass_url=?,
		health_check_enabled=?, health_check_interval_sec=?, health_check_target_path=?,
		health_check_expected_status=?, updated_at=?
		WHERE id=?`,
		m.Name, m.Group, boolInt(m.Enabled), string(m.Protocol), m.ListenPort, m.ServerNames, m.ListenPath,
		m.UpstreamHost, m.UpstreamPort, m.TLSCertPath, m.TLSKeyPath, boolInt(m.RedirectHTTP),
		m.ExtraDirectives, string(m.ProxyMode), m.ProxyHeaders, m.ProxyPassURL,
		boolInt(m.HealthCheckEnabled), m.HealthCheckIntervalSec, m.HealthCheckTargetPath,
		m.HealthCheckExpectedCode, m.UpdatedAt, m.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetMappingEnabled turns a mapping on or off.
func (s *Store) SetMappingEnabled(id string, enabled bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE mappings SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolInt(enabled), nowMillis(), id)
	return err
}

// DeleteMapping removes a mapping.
func (s *Store) DeleteMapping(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM mappings WHERE id = ?`, id)
	return err
}

// SetMappingHealth records the outcome of a health check.
func (s *Store) SetMappingHealth(id string, status model.HealthStatus, at int64) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE mappings SET health_status = ?, last_checked_at = ? WHERE id = ?`,
		string(status), at, id)
	return err
}

// ReorderMappings applies a new sort order: ids[i] gets position i.
func (s *Store) ReorderMappings(ids []string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE mappings SET sort_order = ? WHERE id = ?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplaceAllMappings removes every mapping and inserts the given ones,
// which is what an import does.
func (s *Store) ReplaceAllMappings(ms []model.Mapping) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM mappings`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM config_snapshots`); err != nil {
		return err
	}
	for i, m := range ms {
		if m.ID == "" {
			m.ID = newID()
		}
		if _, err := tx.Exec(insertMapping,
			m.ID, localHostID, m.Name, m.Group, boolInt(m.Enabled), string(m.Protocol), m.ListenPort, m.ServerNames, m.ListenPath,
			m.UpstreamHost, m.UpstreamPort, m.TLSCertPath, m.TLSKeyPath, boolInt(m.RedirectHTTP), m.ExtraDirectives,
			string(m.ProxyMode), m.ProxyHeaders, m.ProxyPassURL,
			boolInt(m.HealthCheckEnabled), m.HealthCheckIntervalSec, m.HealthCheckTargetPath, m.HealthCheckExpectedCode,
			string(model.HealthUnknown), nil, i, m.CreatedAt, nowMillis()); err != nil {
			return fmt.Errorf("importing %q: %w", m.Name, err)
		}
	}
	return tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullInt(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
