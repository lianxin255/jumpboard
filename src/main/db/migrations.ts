import type { Database } from 'better-sqlite3'

export function runMigrations(db: Database): void {
  db.pragma('foreign_keys = ON')
  db.pragma('journal_mode = WAL')

  db.exec(`
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
  `)
}
