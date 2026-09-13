import type { SqliteDb } from './connection'

export function runMigrations(db: SqliteDb): void {
  // node:sqlite 无 .pragma() helper，PRAGMA 走 exec
  db.exec('PRAGMA foreign_keys = ON')
  db.exec('PRAGMA journal_mode = WAL')

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

    -- 全局 IP 白名单（开启后只允许列表中的 IP 访问所有代理服务）
    CREATE TABLE IF NOT EXISTS ip_whitelist (
      id TEXT PRIMARY KEY,
      ip TEXT NOT NULL,
      note TEXT,
      enabled INTEGER NOT NULL DEFAULT 1,
      created_at INTEGER NOT NULL
    );

    -- 应用级设置（键值对存储）
    CREATE TABLE IF NOT EXISTS app_settings (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL
    );
  `)

  // 增量迁移：给 mappings 表加 proxy_mode 和 proxy_headers 字段
  // SQLite 的 ALTER TABLE ADD COLUMN 不支持 IF NOT EXISTS，需检查列是否存在
  const cols = db.prepare("PRAGMA table_info(mappings)").all() as unknown as { name: string }[]
  const colNames = cols.map(c => c.name)
  if (!colNames.includes('proxy_mode')) {
    db.exec("ALTER TABLE mappings ADD COLUMN proxy_mode TEXT NOT NULL DEFAULT 'standard'")
  }
  if (!colNames.includes('proxy_headers')) {
    db.exec("ALTER TABLE mappings ADD COLUMN proxy_headers TEXT")
  }
  if (!colNames.includes('proxy_pass_url')) {
    db.exec("ALTER TABLE mappings ADD COLUMN proxy_pass_url TEXT")
  }
}
