package store

import (
	"database/sql"
	"errors"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// Setting keys of app_settings.
const (
	SettingTheme        = "theme"             // "system" | "light" | "dark"
	SettingWhitelistOn  = "whitelist_enabled" // "1" | "0"
	SettingNginxLogRead = "nginx_log_read_enabled"
	SettingAIBaseURL    = "ai_base_url"
	SettingAIModel      = "ai_model"
)

// Setting returns a value, or def when it is unset.
func (s *Store) Setting(key, def string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return def
	}
	return v
}

// SettingBool returns a boolean setting, where only "1" is true.
func (s *Store) SettingBool(key string, def bool) bool {
	v := s.Setting(key, "")
	if v == "" {
		return def
	}
	return v == "1"
}

// SetSetting stores a value.
func (s *Store) SetSetting(key, value string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// --- logs ---

// AddLog appends an entry to the operation log.
func (s *Store) AddLog(level model.LogLevel, source model.LogSource, message string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO app_logs (id, host_id, level, source, message, timestamp)
		VALUES (?, NULL, ?, ?, ?, ?)`,
		newID(), string(level), string(source), message, nowMillis())
	return err
}

// ListLogs returns the most recent entries, newest first.
func (s *Store) ListLogs(limit int) ([]model.LogEntry, error) {
	rows, err := s.db.Query(`SELECT rowid, level, source, message, timestamp
		FROM app_logs ORDER BY timestamp DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.LogEntry
	for rows.Next() {
		var e model.LogEntry
		if err := rows.Scan(&e.ID, &e.Level, &e.Source, &e.Message, &e.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountLogs returns how many entries the log holds.
func (s *Store) CountLogs() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM app_logs`).Scan(&n)
	return n, err
}

// ClearLogs removes every entry.
func (s *Store) ClearLogs() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM app_logs`)
	return err
}

// TrimLogs keeps only the newest keep entries.
func (s *Store) TrimLogs(keep int) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM app_logs WHERE rowid NOT IN (
		SELECT rowid FROM app_logs ORDER BY timestamp DESC, rowid DESC LIMIT ?)`, keep)
	return err
}

// --- snapshots ---

// AddSnapshot stores a configuration.
func (s *Store) AddSnapshot(content, reason string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT INTO config_snapshots (id, host_id, content, reason, created_at)
		VALUES (?, ?, ?, ?, ?)`, newID(), localHostID, content, reason, nowMillis())
	return err
}

// ListSnapshots returns the newest snapshots.
func (s *Store) ListSnapshots(limit int) ([]model.Snapshot, error) {
	rows, err := s.db.Query(`SELECT id, content, reason, created_at FROM config_snapshots
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Snapshot
	for rows.Next() {
		var sn model.Snapshot
		if err := rows.Scan(&sn.ID, &sn.Content, &sn.Reason, &sn.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// GetSnapshot returns one snapshot.
func (s *Store) GetSnapshot(id string) (model.Snapshot, error) {
	var sn model.Snapshot
	err := s.db.QueryRow(`SELECT id, content, reason, created_at FROM config_snapshots WHERE id = ?`, id).
		Scan(&sn.ID, &sn.Content, &sn.Reason, &sn.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Snapshot{}, ErrNotFound
	}
	return sn, err
}

// TrimSnapshots keeps the newest keep snapshots.
func (s *Store) TrimSnapshots(keep int) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM config_snapshots WHERE id NOT IN (
		SELECT id FROM config_snapshots ORDER BY created_at DESC LIMIT ?)`, keep)
	return err
}

// --- whitelist ---

// ListWhitelist returns the allowed addresses.
func (s *Store) ListWhitelist() ([]model.WhitelistEntry, error) {
	rows, err := s.db.Query(`SELECT rowid, ip, note, enabled, created_at FROM ip_whitelist ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WhitelistEntry
	for rows.Next() {
		var e model.WhitelistEntry
		var enabled int
		if err := rows.Scan(&e.ID, &e.IP, &e.Note, &enabled, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Enabled = enabled == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// EnabledWhitelistIPs returns the addresses a configuration should allow.
func (s *Store) EnabledWhitelistIPs() ([]string, error) {
	rows, err := s.db.Query(`SELECT ip FROM ip_whitelist WHERE enabled = 1 ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		out = append(out, ip)
	}
	return out, rows.Err()
}

// AddWhitelist allows an address.
func (s *Store) AddWhitelist(ip, note string) (model.WhitelistEntry, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	e := model.WhitelistEntry{IP: ip, Note: note, Enabled: true, CreatedAt: nowMillis()}
	res, err := s.db.Exec(`INSERT INTO ip_whitelist (id, ip, note, enabled, created_at) VALUES (?, ?, ?, 1, ?)`,
		newID(), ip, note, e.CreatedAt)
	if err != nil {
		return model.WhitelistEntry{}, err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return e, nil
}

// DeleteWhitelist removes an entry.
func (s *Store) DeleteWhitelist(id int64) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`DELETE FROM ip_whitelist WHERE rowid = ?`, id)
	return err
}

// SetWhitelistEnabled turns one entry on or off.
func (s *Store) SetWhitelistEnabled(id int64, enabled bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE ip_whitelist SET enabled = ? WHERE rowid = ?`, boolInt(enabled), id)
	return err
}

// WhitelistOn reports whether the whitelist limits access at all.
func (s *Store) WhitelistOn() bool { return s.SettingBool(SettingWhitelistOn, false) }

// SetWhitelistOn turns the whitelist on or off.
func (s *Store) SetWhitelistOn(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return s.SetSetting(SettingWhitelistOn, v)
}

// Wipe removes every row of every table, keeping the schema.
func (s *Store) Wipe() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"app_logs", "config_snapshots", "mappings", "ip_whitelist", "app_settings"} {
		if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Keep sqlite imported for the tests' convenience.
var _ = sql.ErrNoRows
