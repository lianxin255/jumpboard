// Package store keeps the app's data in SQLite. The schema is the one the
// Electron build wrote, so an existing data/data.db opens as it is.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // the pure-Go driver: no cgo, cross-compiles everywhere
)

// Store is the app's database.
type Store struct {
	db *sql.DB
	// writeMu serializes writes: the driver is used with one connection,
	// and SQLite allows one writer anyway.
	writeMu sync.Mutex
}

// Open opens or creates the database at path and runs the migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	// One connection keeps the pragmas above on every statement and
	// avoids SQLITE_BUSY between the pool's connections.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for the tests and the few queries that live
// outside the repositories.
func (s *Store) DB() *sql.DB { return s.db }

// migrate creates the schema, adding the columns later versions introduced
// the same way the Electron build did: ALTER TABLE has no IF NOT EXISTS,
// so each column is checked first.
func (s *Store) migrate() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("creating the schema: %w", err)
	}
	cols, err := s.tableColumns("mappings")
	if err != nil {
		return err
	}
	for _, c := range []struct{ name, ddl string }{
		{"proxy_mode", `ALTER TABLE mappings ADD COLUMN proxy_mode TEXT NOT NULL DEFAULT 'standard'`},
		{"proxy_headers", `ALTER TABLE mappings ADD COLUMN proxy_headers TEXT`},
		{"proxy_pass_url", `ALTER TABLE mappings ADD COLUMN proxy_pass_url TEXT`},
		{"health_check_target_path", `ALTER TABLE mappings ADD COLUMN health_check_target_path TEXT`},
		{"health_check_expected_status", `ALTER TABLE mappings ADD COLUMN health_check_expected_status INTEGER`},
		{"group_name", `ALTER TABLE mappings ADD COLUMN group_name TEXT NOT NULL DEFAULT ''`},
	} {
		if cols[c.name] {
			continue
		}
		if _, err := s.db.Exec(c.ddl); err != nil {
			return fmt.Errorf("adding mappings.%s: %w", c.name, err)
		}
	}
	// app_settings gained no schema of its own; the rows are defaults.
	return nil
}

// tableColumns returns the columns of a table, by name.
func (s *Store) tableColumns(table string) (map[string]bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name, typ  string
			notNull    int
			dflt       sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &primaryKey); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

// schema matches the Electron build's migrations.ts, so that an existing
// database needs no conversion.
const schema = `
CREATE TABLE IF NOT EXISTS hosts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK(type IN ('local','remote')),
  is_default INTEGER NOT NULL DEFAULT 0,
  local_config TEXT,
  remote_config TEXT,
  status TEXT NOT NULL DEFAULT 'unknown',
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mappings (
  id TEXT PRIMARY KEY,
  host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  protocol TEXT NOT NULL CHECK(protocol IN ('tcp','udp','http','https')),
  listen_port INTEGER,
  server_names TEXT,
  listen_path TEXT,
  upstream_host TEXT NOT NULL,
  upstream_port INTEGER NOT NULL,
  tls_cert_path TEXT,
  tls_key_path TEXT,
  redirect_http INTEGER NOT NULL DEFAULT 0,
  extra_directives TEXT,
  health_check_enabled INTEGER NOT NULL DEFAULT 0,
  health_check_interval_sec INTEGER NOT NULL DEFAULT 30,
  health_check_target_path TEXT,
  health_check_expected_status INTEGER,
  health_status TEXT NOT NULL DEFAULT 'unknown',
  last_checked_at INTEGER,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_mappings_host ON mappings(host_id);

CREATE TABLE IF NOT EXISTS config_snapshots (
  id TEXT PRIMARY KEY,
  host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_snapshots_host_time ON config_snapshots(host_id, created_at);

CREATE TABLE IF NOT EXISTS app_logs (
  id TEXT PRIMARY KEY,
  host_id TEXT,
  level TEXT NOT NULL,
  source TEXT NOT NULL,
  message TEXT NOT NULL,
  timestamp INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_logs_host_time ON app_logs(host_id, timestamp);

CREATE TABLE IF NOT EXISTS ip_whitelist (
  id TEXT PRIMARY KEY,
  ip TEXT NOT NULL,
  note TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// nowMillis is the timestamp the app stores everywhere. Tests replace it to
// force several rows into the same millisecond.
var nowMillis = func() int64 { return time.Now().UnixMilli() }
