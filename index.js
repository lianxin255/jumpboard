"use strict";
const node_path = require("node:path");
const Database = require("better-sqlite3");
const fsExtra = require("fs-extra");
const node_crypto = require("node:crypto");
const node_child_process = require("node:child_process");
const ssh2 = require("ssh2");
const http = require("node:http");
const https = require("node:https");
const node_net = require("node:net");
const yaml = require("yaml");
const node_fs = require("node:fs");
const electron = require("electron");
function getElectronApp() {
  try {
    const electron2 = require("electron");
    return electron2?.app ?? null;
  } catch {
    return null;
  }
}
function getDataRoot() {
  const app = getElectronApp();
  if (app?.isPackaged) {
    if (process.platform === "win32" && process.env.PORTABLE_EXECUTABLE_DIR)
      return process.env.PORTABLE_EXECUTABLE_DIR;
    const exePath = app.getPath("exe");
    if (process.platform === "darwin")
      return node_path.dirname(node_path.dirname(node_path.dirname(exePath)));
    return node_path.dirname(exePath);
  }
  return process.cwd();
}
function getResourcesDir() {
  const app = getElectronApp();
  if (app?.isPackaged && process.resourcesPath)
    return process.resourcesPath;
  return node_path.join(getDataRoot(), "resources");
}
function resolvePaths() {
  const dataRoot = getDataRoot();
  const dataDir = node_path.join(dataRoot, "data");
  const resourcesDir = getResourcesDir();
  return {
    dataRoot,
    dataDir,
    dbPath: node_path.join(dataDir, "data.db"),
    nginxConfDir: node_path.join(dataDir, "nginx-conf"),
    nginxLogDir: node_path.join(dataDir, "logs"),
    snapshotDir: node_path.join(dataDir, "snapshots"),
    tempDir: node_path.join(dataDir, "temp"),
    resourcesDir,
    bundledNginxDir: node_path.join(resourcesDir, "bundled", "nginx"),
    ensureDataDirs: () => {
      for (const d of [
        dataDir,
        node_path.join(dataDir, "nginx-conf"),
        node_path.join(dataDir, "nginx-conf", "http"),
        node_path.join(dataDir, "nginx-conf", "stream"),
        node_path.join(dataDir, "logs"),
        node_path.join(dataDir, "snapshots"),
        node_path.join(dataDir, "temp")
      ]) {
        fsExtra.ensureDirSync(d);
      }
    }
  };
}
function runMigrations(db) {
  db.pragma("foreign_keys = ON");
  db.pragma("journal_mode = WAL");
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
  `);
  const cols = db.prepare("PRAGMA table_info(mappings)").all();
  const colNames = cols.map((c) => c.name);
  if (!colNames.includes("proxy_mode")) {
    db.exec("ALTER TABLE mappings ADD COLUMN proxy_mode TEXT NOT NULL DEFAULT 'standard'");
  }
  if (!colNames.includes("proxy_headers")) {
    db.exec("ALTER TABLE mappings ADD COLUMN proxy_headers TEXT");
  }
  if (!colNames.includes("proxy_pass_url")) {
    db.exec("ALTER TABLE mappings ADD COLUMN proxy_pass_url TEXT");
  }
}
let dbInstance = null;
function resolveDbPath() {
  const paths = resolvePaths();
  paths.ensureDataDirs();
  return paths.dbPath;
}
function openDb() {
  if (dbInstance)
    return dbInstance;
  const path = resolveDbPath();
  const db = new Database(path);
  try {
    runMigrations(db);
  } catch (e) {
    db.close();
    throw e;
  }
  dbInstance = db;
  return db;
}
function closeDb() {
  if (dbInstance) {
    dbInstance.close();
    dbInstance = null;
  }
}
function getDb() {
  if (!dbInstance)
    throw new Error("DB not opened. Call openDb() first.");
  return dbInstance;
}
function rowToHost$1(r) {
  return {
    id: r.id,
    name: r.name,
    type: r.type,
    isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status,
    createdAt: r.created_at
  };
}
function createHost(input) {
  const db = getDb();
  const id = node_crypto.randomUUID();
  const now = Date.now();
  db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
              VALUES (?, ?, ?, ?, ?, ?, 'unknown', ?)`).run(id, input.name, input.type, input.isDefault, input.localConfig ? JSON.stringify(input.localConfig) : null, input.remoteConfig ? JSON.stringify(input.remoteConfig) : null, now);
  return getHost(id);
}
function listHosts() {
  const rows = getDb().prepare("SELECT * FROM hosts ORDER BY created_at").all();
  return rows.map(rowToHost$1);
}
function getHost(id) {
  const row = getDb().prepare("SELECT * FROM hosts WHERE id = ?").get(id);
  return row ? rowToHost$1(row) : void 0;
}
function updateHost(id, patch) {
  const cur = getHost(id);
  if (!cur)
    throw new Error(`Host not found: ${id}`);
  const merged = { ...cur, ...patch };
  getDb().prepare(`UPDATE hosts SET name=?, type=?, is_default=?, local_config=?, remote_config=? WHERE id=?`).run(merged.name, merged.type, merged.isDefault, merged.localConfig ? JSON.stringify(merged.localConfig) : null, merged.remoteConfig ? JSON.stringify(merged.remoteConfig) : null, id);
}
function deleteHost(id) {
  getDb().prepare("DELETE FROM hosts WHERE id = ?").run(id);
}
function setDefaultHost(id) {
  const db = getDb();
  db.transaction(() => {
    db.prepare("UPDATE hosts SET is_default = 0").run();
    db.prepare("UPDATE hosts SET is_default = 1 WHERE id = ?").run(id);
  })();
}
function getDefaultHost() {
  const row = getDb().prepare("SELECT * FROM hosts WHERE is_default = 1 ORDER BY created_at LIMIT 1").get();
  return row ? rowToHost$1(row) : void 0;
}
const API = {
  // Host
  HOST_LIST: "/api/host/list",
  HOST_CREATE: "/api/host/create",
  HOST_UPDATE: "/api/host/update",
  HOST_DELETE: "/api/host/delete",
  HOST_SET_DEFAULT: "/api/host/set-default",
  HOST_SWITCH: "/api/host/switch",
  HOST_TEST_CONNECTION: "/api/host/test-connection",
  // Mapping
  MAPPING_LIST: "/api/mapping/list",
  MAPPING_CREATE: "/api/mapping/create",
  MAPPING_UPDATE: "/api/mapping/update",
  MAPPING_DELETE: "/api/mapping/delete",
  // Engine
  ENGINE_START: "/api/engine/start",
  ENGINE_STOP: "/api/engine/stop",
  ENGINE_RELOAD: "/api/engine/reload",
  ENGINE_TEST: "/api/engine/test",
  ENGINE_STATUS: "/api/engine/status",
  ENGINE_DIRTY_GET: "/api/engine/dirty-get",
  ENGINE_PID_GET: "/api/engine/pid-get",
  ENGINE_KIND_GET: "/api/engine/kind-get",
  ENGINE_REFRESH_STATUS: "/api/engine/refresh-status",
  // Template
  TEMPLATE_LIST: "/api/template/list",
  TEMPLATE_LIST_BUILTIN: "/api/template/list-builtin",
  TEMPLATE_SAVE_CUSTOM: "/api/template/save-custom",
  TEMPLATE_DELETE_CUSTOM: "/api/template/delete-custom",
  // Log
  LOG_LIST: "/api/log/list",
  LOG_CLEAR: "/api/log/clear",
  NGINX_LOG_TAIL: "/api/nginx-log/tail",
  NGINX_LOG_CLEAR: "/api/nginx-log/clear",
  NGINX_LOG_READ_ENABLED_GET: "/api/nginx-log/read-enabled-get",
  NGINX_LOG_READ_ENABLED_SET: "/api/nginx-log/read-enabled-set",
  // Snapshot
  SNAPSHOT_LIST: "/api/snapshot/list",
  SNAPSHOT_GET: "/api/snapshot/get",
  SNAPSHOT_ROLLBACK: "/api/snapshot/rollback",
  // Health check
  HEALTH_CHECK_NOW: "/api/health/check-now",
  // Port check
  PORT_CHECK: "/api/port/check",
  // Network
  GET_LOCAL_IPS: "/api/network/local-ips",
  // 数据管理
  DATA_WIPE: "/api/data/wipe",
  DATA_EXPORT: "/api/data/export",
  DATA_IMPORT: "/api/data/import",
  // IP 白名单（全局访问控制）
  WHITELIST_LIST: "/api/whitelist/list",
  WHITELIST_ADD: "/api/whitelist/add",
  WHITELIST_REMOVE: "/api/whitelist/remove",
  WHITELIST_TOGGLE_IP: "/api/whitelist/toggle-ip",
  WHITELIST_GET_ENABLED: "/api/whitelist/get-enabled",
  WHITELIST_SET_ENABLED: "/api/whitelist/set-enabled"
};
function isValidIpOrCidr(s) {
  if (typeof s !== "string")
    return false;
  const [ipPart, cidr] = s.split("/");
  const segs = ipPart.split(".");
  if (segs.length !== 4)
    return false;
  if (!segs.every((n) => /^(0|[1-9]\d{0,2})$/.test(n) && +n >= 0 && +n <= 255))
    return false;
  if (cidr !== void 0 && (!/^\d+$/.test(cidr) || +cidr < 0 || +cidr > 32))
    return false;
  if (cidr !== void 0 && +cidr === 0)
    return false;
  return true;
}
function isValidPort(p) {
  const n = Number(p);
  return Number.isInteger(n) && n >= 1 && n <= 65535;
}
function clampInt(n, min, max) {
  const v = Number(n);
  if (!Number.isFinite(v))
    return min;
  return Math.max(min, Math.min(max, Math.trunc(v)));
}
function isValidProtocol(v) {
  return v === "tcp" || v === "udp" || v === "http" || v === "https";
}
function isValidProxyMode(v) {
  return v === "standard" || v === "minimal" || v === "custom";
}
function isValidProxyPassUrl(v) {
  if (typeof v !== "string" || v.length === 0 || v.length > 500)
    return false;
  if (!/^https?:\/\//.test(v))
    return false;
  if (/[;{}#\r\n]/.test(v))
    return false;
  return true;
}
const FALLBACK_KEY = node_crypto.randomBytes(32);
function trySafeStorage() {
  try {
    const electron2 = require("electron");
    if (electron2?.safeStorage && electron2.safeStorage.isEncryptionAvailable())
      return electron2.safeStorage;
  } catch {
  }
  return null;
}
function encryptPassword(plain) {
  const safe = trySafeStorage();
  if (safe) {
    const buf = safe.encryptString(plain);
    return `ss1:${buf.toString("base64")}`;
  }
  const iv = node_crypto.randomBytes(12);
  const cipher = node_crypto.createCipheriv("aes-256-gcm", FALLBACK_KEY, iv);
  const enc = Buffer.concat([cipher.update(plain, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return `fb1:${Buffer.concat([iv, tag, enc]).toString("base64")}`;
}
function decryptPassword(stored) {
  const safe = trySafeStorage();
  if (stored.startsWith("ss1:")) {
    if (!safe)
      throw new Error("密码用 safeStorage 加密，但当前环境不可用");
    const buf = Buffer.from(stored.slice(4), "base64");
    return safe.decryptString(buf);
  }
  if (stored.startsWith("fb1:")) {
    const raw = Buffer.from(stored.slice(4), "base64");
    const iv = raw.subarray(0, 12);
    const tag = raw.subarray(12, 28);
    const enc = raw.subarray(28);
    const decipher = node_crypto.createDecipheriv("aes-256-gcm", FALLBACK_KEY, iv);
    decipher.setAuthTag(tag);
    return Buffer.concat([decipher.update(enc), decipher.final()]).toString("utf8");
  }
  throw new Error("无法识别的密码加密格式");
}
function rowToLog$1(r) {
  return {
    id: r.id,
    hostId: r.host_id,
    level: r.level,
    source: r.source,
    message: r.message,
    timestamp: r.timestamp
  };
}
function addLog(input) {
  const id = node_crypto.randomUUID();
  const timestamp = Date.now();
  getDb().prepare(`INSERT INTO app_logs (id, host_id, level, source, message, timestamp)
    VALUES (?, ?, ?, ?, ?, ?)`).run(id, input.hostId, input.level, input.source, input.message, timestamp);
  return { id, ...input, timestamp };
}
function listLogs(hostId, limit) {
  const sql = hostId ? `SELECT * FROM app_logs WHERE host_id = ? ORDER BY timestamp DESC, rowid DESC LIMIT ?` : `SELECT * FROM app_logs ORDER BY timestamp DESC, rowid DESC LIMIT ?`;
  const rows = hostId ? getDb().prepare(sql).all(hostId, limit) : getDb().prepare(sql).all(limit);
  return rows.map(rowToLog$1);
}
function trimLogs(keepCount) {
  const db = getDb();
  const result = db.prepare(`DELETE FROM app_logs WHERE rowid NOT IN (
    SELECT rowid FROM app_logs ORDER BY timestamp DESC, rowid DESC LIMIT ?
  )`).run(keepCount);
  return result.changes;
}
function clearLogs() {
  getDb().prepare("DELETE FROM app_logs").run();
}
function rowToMapping$1(r) {
  return {
    id: r.id,
    hostId: r.host_id,
    name: r.name,
    enabled: r.enabled,
    protocol: r.protocol,
    listenPort: r.listen_port,
    serverNames: r.server_names,
    listenPath: r.listen_path,
    upstreamHost: r.upstream_host,
    upstreamPort: r.upstream_port,
    tlsCertPath: r.tls_cert_path,
    tlsKeyPath: r.tls_key_path,
    redirectHttp: r.redirect_http,
    extraDirectives: r.extra_directives,
    proxyMode: r.proxy_mode || "standard",
    proxyHeaders: r.proxy_headers,
    proxyPassUrl: r.proxy_pass_url,
    healthCheckEnabled: r.health_check_enabled,
    healthCheckIntervalSec: r.health_check_interval_sec,
    healthCheckTargetPath: r.health_check_target_path,
    healthCheckExpectedStatus: r.health_check_expected_status,
    healthStatus: r.health_status,
    lastCheckedAt: r.last_checked_at,
    sortOrder: r.sort_order,
    createdAt: r.created_at,
    updatedAt: r.updated_at
  };
}
const COLUMNS = `id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
  upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
  proxy_mode, proxy_headers, proxy_pass_url,
  health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
  health_status, last_checked_at, sort_order, created_at, updated_at`;
function createMapping(input) {
  const db = getDb();
  const id = node_crypto.randomUUID();
  const now = Date.now();
  const maxOrder = db.prepare("SELECT COALESCE(MAX(sort_order), -1) AS m FROM mappings WHERE host_id = ?").get(input.hostId).m;
  db.prepare(`INSERT INTO mappings (${COLUMNS})
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`).run(id, input.hostId, input.name, input.enabled, input.protocol, input.listenPort, input.serverNames, input.listenPath, input.upstreamHost, input.upstreamPort, input.tlsCertPath, input.tlsKeyPath, input.redirectHttp, input.extraDirectives, input.proxyMode || "standard", input.proxyHeaders ?? null, input.proxyPassUrl ?? null, input.healthCheckEnabled, input.healthCheckIntervalSec, input.healthCheckTargetPath, input.healthCheckExpectedStatus, "unknown", null, maxOrder + 1, now, now);
  return getMapping(id);
}
function listMappings(hostId) {
  const rows = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE host_id = ? ORDER BY sort_order`).all(hostId);
  return rows.map(rowToMapping$1);
}
function listEnabledMappings(hostId) {
  const rows = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE host_id = ? AND enabled = 1 ORDER BY sort_order`).all(hostId);
  return rows.map(rowToMapping$1);
}
function getMapping(id) {
  const row = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE id = ?`).get(id);
  return row ? rowToMapping$1(row) : void 0;
}
function updateMapping(id, patch) {
  const cur = getMapping(id);
  if (!cur)
    throw new Error(`Mapping not found: ${id}`);
  const merged = { ...cur, ...patch };
  getDb().prepare(`UPDATE mappings SET
    name=?, enabled=?, protocol=?, listen_port=?, server_names=?, listen_path=?,
    upstream_host=?, upstream_port=?, tls_cert_path=?, tls_key_path=?, redirect_http=?,
    extra_directives=?, proxy_mode=?, proxy_headers=?, proxy_pass_url=?,
    health_check_enabled=?, health_check_interval_sec=?,
    health_check_target_path=?, health_check_expected_status=?, updated_at=? WHERE id=?`).run(merged.name, merged.enabled, merged.protocol, merged.listenPort, merged.serverNames, merged.listenPath, merged.upstreamHost, merged.upstreamPort, merged.tlsCertPath, merged.tlsKeyPath, merged.redirectHttp, merged.extraDirectives, merged.proxyMode || "standard", merged.proxyHeaders ?? null, merged.proxyPassUrl ?? null, merged.healthCheckEnabled, merged.healthCheckIntervalSec, merged.healthCheckTargetPath, merged.healthCheckExpectedStatus, Date.now(), id);
}
function deleteMapping(id) {
  getDb().prepare("DELETE FROM mappings WHERE id = ?").run(id);
}
function updateHealthStatus(id, healthStatus, lastCheckedAt) {
  getDb().prepare(`UPDATE mappings SET health_status = ?, last_checked_at = ? WHERE id = ?`).run(healthStatus, lastCheckedAt, id);
}
function rowToSnapshot$1(r) {
  return {
    id: r.id,
    hostId: r.host_id,
    content: r.content,
    reason: r.reason,
    createdAt: r.created_at
  };
}
function createSnapshot(input) {
  const id = node_crypto.randomUUID();
  const createdAt = Date.now();
  getDb().prepare(`INSERT INTO config_snapshots (id, host_id, content, reason, created_at)
    VALUES (?, ?, ?, ?, ?)`).run(id, input.hostId, input.content, input.reason, createdAt);
  return { id, ...input, createdAt };
}
function listSnapshots(hostId, limit) {
  const rows = getDb().prepare(`SELECT * FROM config_snapshots WHERE host_id = ? ORDER BY created_at DESC LIMIT ?`).all(hostId, limit);
  return rows.map(rowToSnapshot$1);
}
function getSnapshot(id) {
  const row = getDb().prepare(`SELECT * FROM config_snapshots WHERE id = ?`).get(id);
  return row ? rowToSnapshot$1(row) : void 0;
}
function trimSnapshots(keepPerHost) {
  const db = getDb();
  const result = db.prepare(`DELETE FROM config_snapshots WHERE id NOT IN (
    SELECT id FROM (
      SELECT id, host_id,
             ROW_NUMBER() OVER (PARTITION BY host_id ORDER BY created_at DESC) AS rn
      FROM config_snapshots
    ) WHERE rn <= ?
  )`).run(keepPerHost);
  return result.changes;
}
function rowToEntry(r) {
  return {
    id: r.id,
    ip: r.ip,
    note: r.note,
    enabled: r.enabled,
    createdAt: r.created_at
  };
}
function listWhitelist() {
  const rows = getDb().prepare("SELECT * FROM ip_whitelist ORDER BY created_at").all();
  return rows.map(rowToEntry);
}
function getEnabledWhitelistIps() {
  const rows = getDb().prepare("SELECT ip FROM ip_whitelist WHERE enabled = 1").all();
  return rows.map((r) => r.ip);
}
function addWhitelist(ip, note) {
  const entry = {
    id: node_crypto.randomUUID(),
    ip,
    note: note ?? null,
    enabled: 1,
    createdAt: Date.now()
  };
  getDb().prepare("INSERT INTO ip_whitelist (id, ip, note, enabled, created_at) VALUES (?, ?, ?, ?, ?)").run(entry.id, entry.ip, entry.note, entry.enabled, entry.createdAt);
  return entry;
}
function removeWhitelist(id) {
  getDb().prepare("DELETE FROM ip_whitelist WHERE id = ?").run(id);
}
function toggleWhitelist(id, enabled) {
  getDb().prepare("UPDATE ip_whitelist SET enabled = ? WHERE id = ?").run(enabled, id);
}
function isWhitelistEnabled() {
  const row = getDb().prepare("SELECT value FROM app_settings WHERE key = ?").get("whitelist_enabled");
  return row?.value === "1";
}
function setWhitelistEnabled(enabled) {
  const db = getDb();
  db.prepare("INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?").run("whitelist_enabled", enabled ? "1" : "0", enabled ? "1" : "0");
}
const MAX_LOGS = 1e4;
function log(hostId, level, source, message) {
  try {
    addLog({ hostId, level, source, message });
    if (Math.random() < 0.01)
      trimLogs(MAX_LOGS);
  } catch {
  }
}
const logger = {
  info: (hostId, source, msg) => log(hostId, "info", source, msg),
  warn: (hostId, source, msg) => log(hostId, "warn", source, msg),
  error: (hostId, source, msg) => log(hostId, "error", source, msg)
};
function sanitize(v) {
  if (!v)
    return "";
  return v.replace(/[;{}#\n\r]/g, "").trim();
}
function safeName(name) {
  return sanitize(name);
}
function safePort(p) {
  if (p == null)
    return null;
  const n = Number(p);
  if (!Number.isInteger(n) || n < 1 || n > 65535)
    return null;
  return n;
}
function renderExtra(extra, indent = "    ") {
  if (!extra || !extra.trim())
    return [];
  const out = [];
  for (const line of extra.split("\n")) {
    if (line.trim())
      out.push(indent + line.trim());
  }
  return out;
}
function standardProxySetHeaders(indent = "      ") {
  return [
    `${indent}proxy_set_header Host $host;`,
    `${indent}proxy_set_header X-Real-IP $remote_addr;`,
    `${indent}proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
    `${indent}proxy_set_header X-Forwarded-Proto $scheme;`,
    `${indent}proxy_set_header Upgrade $http_upgrade;`,
    `${indent}proxy_set_header Connection $connection_upgrade;`
  ];
}
function renderProxyDirectives(m, indent = "      ") {
  const mode = m.proxyMode || "standard";
  if (mode === "minimal") {
    return [];
  }
  if (mode === "custom" && m.proxyHeaders && m.proxyHeaders.trim()) {
    return m.proxyHeaders.split("\n").map((l) => l.replace(/[{}#\r]/g, "").trim()).filter((l) => l).map((l) => indent + l);
  }
  return standardProxySetHeaders(indent);
}
function renderWhitelist(wl, indent = "    ") {
  if (!wl || wl.length === 0)
    return [];
  const lines = [];
  for (const ip of wl) {
    const clean = sanitize(ip);
    if (clean)
      lines.push(`${indent}allow ${clean};`);
  }
  if (lines.length > 0)
    lines.push(`${indent}deny all;`);
  return lines;
}
function renderTcpServer(m, wl) {
  const listen = safePort(m.listenPort);
  if (listen == null)
    return [];
  const upHost = sanitize(m.upstreamHost);
  return [
    `  # mapping: ${safeName(m.name)}`,
    "  server {",
    `    listen ${listen};`,
    ...renderWhitelist(wl),
    `    proxy_pass ${upHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    "  }"
  ];
}
function renderUdpServer(m, wl) {
  const listen = safePort(m.listenPort);
  if (listen == null)
    return [];
  const upHost = sanitize(m.upstreamHost);
  return [
    `  # mapping: ${safeName(m.name)}`,
    "  server {",
    `    listen ${listen} udp;`,
    ...renderWhitelist(wl),
    `    proxy_pass ${upHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    "  }"
  ];
}
function renderProxyPass(m, path, indent = "      ") {
  const upHost = sanitize(m.upstreamHost);
  if (m.proxyPassUrl && m.proxyPassUrl.trim()) {
    return `${indent}proxy_pass ${sanitize(m.proxyPassUrl)};`;
  }
  const trailingSlash = path === "/" ? "/" : "";
  return `${indent}proxy_pass http://${upHost}:${m.upstreamPort}${trailingSlash};`;
}
function renderHttpServer(m, wl) {
  const port = safePort(m.listenPort) ?? 80;
  const path = sanitize(m.listenPath) || "/";
  const serverNames = sanitize(m.serverNames);
  const lines = [];
  lines.push(`  # mapping: ${safeName(m.name)}`);
  lines.push("  server {");
  lines.push(`    listen ${port};`);
  if (serverNames)
    lines.push(`    server_name ${serverNames};`);
  lines.push(`    location ${path} {`);
  lines.push(...renderWhitelist(wl, "      "));
  lines.push(renderProxyPass(m, path));
  lines.push(...renderProxyDirectives(m));
  lines.push(...renderExtra(m.extraDirectives, "      "));
  lines.push("    }");
  lines.push("  }");
  return lines;
}
function renderHttpsServer(m, wl) {
  const path = sanitize(m.listenPath) || "/";
  const serverNames = sanitize(m.serverNames);
  const lines = [];
  lines.push(`  # mapping: ${safeName(m.name)}`);
  if (m.redirectHttp === 1 && serverNames) {
    lines.push("  server {");
    lines.push("    listen 80;");
    lines.push(`    server_name ${serverNames};`);
    lines.push(...renderWhitelist(wl));
    lines.push("    return 301 https://$host$request_uri;");
    lines.push("  }");
    lines.push("");
  }
  lines.push("  server {");
  lines.push("    listen 443 ssl;");
  if (serverNames)
    lines.push(`    server_name ${serverNames};`);
  if (m.tlsCertPath)
    lines.push(`    ssl_certificate ${sanitize(m.tlsCertPath)};`);
  if (m.tlsKeyPath)
    lines.push(`    ssl_certificate_key ${sanitize(m.tlsKeyPath)};`);
  lines.push(`    location ${path} {`);
  lines.push(...renderWhitelist(wl, "      "));
  lines.push(renderProxyPass(m, path));
  lines.push(...renderProxyDirectives(m));
  lines.push(...renderExtra(m.extraDirectives, "      "));
  lines.push("    }");
  lines.push("  }");
  return lines;
}
function generateFullConf(mappings, opts) {
  const enabled = mappings.filter((m) => m.enabled);
  const tcpMappings = enabled.filter((m) => m.protocol === "tcp");
  const udpMappings = enabled.filter((m) => m.protocol === "udp");
  const httpMappings = enabled.filter((m) => m.protocol === "http");
  const httpsMappings = enabled.filter((m) => m.protocol === "https");
  const hasStream = tcpMappings.length > 0 || udpMappings.length > 0;
  const hasHttp = httpMappings.length > 0 || httpsMappings.length > 0;
  const wl = opts.whitelistIps && opts.whitelistIps.length > 0 ? opts.whitelistIps : void 0;
  const lines = [];
  lines.push("# 由 nginx-jump-gui 自动生成，请勿手动编辑");
  lines.push(`# 生成时间: ${(/* @__PURE__ */ new Date()).toISOString()}`);
  lines.push("worker_processes 1;");
  lines.push("error_log logs/error.log warn;");
  lines.push("pid temp/nginx.pid;");
  lines.push("");
  lines.push("events {");
  lines.push("  worker_connections 1024;");
  lines.push("}");
  lines.push("");
  if (hasHttp) {
    lines.push("http {");
    lines.push("  map $http_upgrade $connection_upgrade {");
    lines.push("    default upgrade;");
    lines.push('    ""      close;');
    lines.push("  }");
    lines.push("  proxy_http_version 1.1;");
    lines.push("  client_max_body_size 0;");
    lines.push("  proxy_read_timeout 300s;");
    lines.push("  proxy_buffering off;");
    lines.push("  access_log logs/access.log;");
    lines.push("  client_body_temp_path temp/client_body;");
    lines.push("  proxy_temp_path temp/proxy;");
    lines.push("  include mime.types;");
    if (httpMappings.length > 0) {
      lines.push("  # ---------- HTTP server 块 ----------");
      for (const m of httpMappings) lines.push(...renderHttpServer(m, wl));
    }
    if (httpsMappings.length > 0) {
      lines.push("  # ---------- HTTPS server 块 ----------");
      for (const m of httpsMappings) lines.push(...renderHttpsServer(m, wl));
    }
    lines.push("}");
    lines.push("");
  }
  if (hasStream) {
    lines.push("stream {");
    lines.push("  log_format stream '$remote_addr [$time_local] $protocol $status $bytes_sent';");
    lines.push("  access_log logs/stream_access.log stream;");
    for (const m of tcpMappings) lines.push(...renderTcpServer(m, wl));
    for (const m of udpMappings) lines.push(...renderUdpServer(m, wl));
    lines.push("}");
    lines.push("");
  }
  return lines.join("\n");
}
let confChain = Promise.resolve();
async function safeApplyConf(engine, newConf) {
  let release;
  const myTurn = new Promise((r) => {
    release = r;
  });
  const previous = confChain;
  confChain = myTurn;
  await previous;
  try {
    const oldConf = await engine.readConf();
    try {
      await engine.writeConf(newConf);
    } catch (e) {
      return { ok: false, output: e?.message ?? String(e), failStage: "io" };
    }
    const testResult = await engine.test();
    if (!testResult.ok) {
      try {
        await engine.writeConf(oldConf);
      } catch {
      }
      return { ok: false, output: testResult.output, failStage: "test" };
    }
    try {
      await engine.reload();
      return { ok: true, output: testResult.output };
    } catch (e) {
      try {
        await engine.writeConf(oldConf);
      } catch {
      }
      return { ok: false, output: e?.message ?? String(e), failStage: "reload" };
    }
  } catch (e) {
    return { ok: false, output: e?.message ?? String(e), failStage: "io" };
  } finally {
    release();
  }
}
const SNAPSHOT_KEEP = 50;
async function applyMappings(engine, mappings, opts) {
  const conf = generateFullConf(mappings, opts);
  const result = await safeApplyConf(engine, conf);
  if (result.ok) {
    if (opts.hostId) {
      createSnapshot({ hostId: opts.hostId, content: conf, reason: "reload" });
      trimSnapshots(SNAPSHOT_KEEP);
    }
    logger.info(opts.hostId ?? null, "app", `配置应用成功，${mappings.length} 条映射`);
  } else {
    if (result.failStage === "test") {
      logger.warn(opts.hostId ?? null, "app", `配置校验失败: ${result.output}`);
    } else {
      logger.error(opts.hostId ?? null, "app", `配置应用失败(${result.failStage ?? "unknown"}): ${result.output}`);
    }
  }
  return { ok: result.ok, conf, output: result.output };
}
let current = null;
let currentHostId = null;
let dirty = false;
function setActiveEngine(eng, hostId) {
  if (current?.close) {
    try {
      current.close();
    } catch {
    }
  }
  current = eng;
  currentHostId = eng ? hostId ?? null : null;
  dirty = false;
}
function getActiveEngine() {
  return current;
}
function getActiveHostId() {
  return currentHostId;
}
function isEngineDirty() {
  return dirty;
}
function setEngineDirty(v) {
  dirty = v;
}
async function refreshActiveStatus() {
  const eng = current;
  if (!eng)
    return;
  await eng.refreshStatus();
}
function toFwdSlash(p) {
  if (process.platform === "win32")
    return p.replace(/\\/g, "/");
  return p;
}
class LocalNginxEngine {
  engineKind = "local";
  supportsProcessControl = true;
  child = null;
  cfg;
  constructor(cfg) {
    this.cfg = cfg;
  }
  status() {
    return this.child && !this.child.killed ? "running" : "stopped";
  }
  // local 的 status 实时来自 child，无需异步刷新。refreshStatus 为接口一致性而存在。
  async refreshStatus() {
    return this.status();
  }
  getPid() {
    return this.child?.pid ?? null;
  }
  async start() {
    if (this.child && !this.child.killed) {
      throw new Error("nginx already running");
    }
    if (!fsExtra.pathExistsSync(this.cfg.nginxPath)) {
      throw new Error(`nginx binary not found: ${this.cfg.nginxPath}`);
    }
    fsExtra.ensureDirSync(this.cfg.prefixDir);
    fsExtra.ensureDirSync(node_path.join(this.cfg.prefixDir, "logs"));
    fsExtra.ensureDirSync(node_path.join(this.cfg.prefixDir, "temp"));
    fsExtra.ensureDirSync(node_path.join(this.cfg.prefixDir, "conf"));
    this.child = node_child_process.spawn(this.cfg.nginxPath, [
      "-p",
      toFwdSlash(this.cfg.prefixDir),
      "-c",
      "conf/nginx.conf",
      "-g",
      "daemon off;"
    ], {
      windowsHide: true,
      stdio: ["ignore", "pipe", "pipe"]
    });
    let stderrBuf = "";
    await new Promise((resolve, reject) => {
      const child = this.child;
      let settled = false;
      if (child.stderr) {
        child.stderr.on("data", (d) => {
          stderrBuf += d.toString();
        });
      }
      const onExit = (code) => {
        if (settled)
          return;
        settled = true;
        clearTimeout(timer);
        child.removeListener("error", onError);
        if (this.child === child)
          this.child = null;
        const detail = stderrBuf.trim();
        if (code !== 0 && code !== null) {
          reject(new Error(`nginx exited with code ${code}${detail ? `
${detail}` : ""}`));
        } else {
          reject(new Error(`nginx exited immediately after start${detail ? `
${detail}` : ""}`));
        }
      };
      const onError = (err) => {
        if (settled)
          return;
        settled = true;
        clearTimeout(timer);
        child.removeListener("exit", onExit);
        this.child = null;
        reject(err);
      };
      const timer = setTimeout(() => {
        if (settled)
          return;
        settled = true;
        child.removeListener("exit", onExit);
        child.removeListener("error", onError);
        resolve();
      }, 500);
      child.once("exit", onExit);
      child.once("error", onError);
    });
    const c = this.child;
    c.on("exit", () => {
      if (this.child === c)
        this.child = null;
    });
  }
  async stop() {
    if (!this.child)
      return;
    const c = this.child;
    const pid = c.pid;
    if (process.platform === "win32") {
      await new Promise((resolve) => {
        node_child_process.execFile("taskkill", ["/PID", String(pid), "/T", "/F"], () => {
          resolve();
        });
      });
      await this.waitForExit(c, 3e3);
    } else {
      c.kill("SIGTERM");
      await new Promise((resolve) => {
        const t = setTimeout(() => {
          if (this.child === c)
            this.child.kill("SIGKILL");
          resolve();
        }, 3e3);
        const onExit = () => {
          clearTimeout(t);
          if (this.child === c)
            this.child = null;
          resolve();
        };
        c.once("exit", onExit);
      });
    }
  }
  async reload() {
    if (!this.child)
      throw new Error("nginx not running, cannot reload");
    if (process.platform === "win32") {
      await this.sendSignal("reload");
    } else {
      const pid = this.child.pid;
      if (pid)
        process.kill(pid, "SIGHUP");
    }
  }
  /**
   * Windows 专用：执行 nginx -s <signal>，通过 pid 文件让 master 响应。
   * signal: 'reload' | 'stop' | 'quit'
   */
  async sendSignal(signal) {
    return new Promise((resolve) => {
      node_child_process.execFile(
        this.cfg.nginxPath,
        ["-p", toFwdSlash(this.cfg.prefixDir), "-c", "conf/nginx.conf", "-s", signal],
        () => resolve()
        // 不管成功失败都 resolve，reload/stop 的结果靠后续状态判断
      );
    });
  }
  /** 等待子进程退出，超时后强 kill 兜底 */
  async waitForExit(c, timeoutMs) {
    return new Promise((resolve) => {
      let done = false;
      const finish = () => {
        if (!done) {
          done = true;
          if (this.child === c)
            this.child = null;
          resolve();
        }
      };
      const t = setTimeout(() => {
        if (this.child === c) {
          try {
            c.kill("SIGKILL");
          } catch {
          }
        }
        finish();
      }, timeoutMs);
      c.once("exit", () => {
        clearTimeout(t);
        finish();
      });
    });
  }
  async test() {
    fsExtra.ensureDirSync(this.cfg.prefixDir);
    fsExtra.ensureDirSync(node_path.join(this.cfg.prefixDir, "logs"));
    fsExtra.ensureDirSync(node_path.join(this.cfg.prefixDir, "temp"));
    fsExtra.ensureDirSync(node_path.join(this.cfg.prefixDir, "conf"));
    return new Promise((resolve) => {
      node_child_process.execFile(this.cfg.nginxPath, ["-t", "-p", toFwdSlash(this.cfg.prefixDir), "-c", "conf/nginx.conf"], (err, stdout, stderr) => {
        const output = (stdout + stderr).trim();
        if (err)
          resolve({ ok: false, output });
        else resolve({ ok: true, output });
      });
    });
  }
  async writeConf(content) {
    fsExtra.outputFileSync(this.cfg.confPath, content);
  }
  async readConf() {
    if (!fsExtra.pathExistsSync(this.cfg.confPath))
      return "";
    return fsExtra.readFileSync(this.cfg.confPath, "utf-8");
  }
}
class SshClient {
  client;
  connected = false;
  connecting = null;
  disconnected = false;
  // 连接断开后标记，connect() 时据此重建 Client
  config;
  constructor(config) {
    this.config = {
      host: config.host,
      port: config.port,
      username: config.username,
      ...config.password ? { password: config.password } : {},
      ...config.privateKey ? { privateKey: config.privateKey } : {},
      readyTimeout: 1e4
    };
    this.client = new ssh2.Client();
    this.client.on("close", () => {
      this.connected = false;
      this.connecting = null;
      this.disconnected = true;
    });
    this.client.on("end", () => {
      this.connected = false;
    });
    this.client.on("error", () => {
      this.connected = false;
      this.connecting = null;
      this.disconnected = true;
    });
  }
  async connect() {
    if (this.connected)
      return;
    if (this.connecting)
      return this.connecting;
    if (this.disconnected) {
      this.client = new ssh2.Client();
      this.client.on("close", () => {
        this.connected = false;
        this.connecting = null;
        this.disconnected = true;
      });
      this.client.on("end", () => {
        this.connected = false;
      });
      this.client.on("error", () => {
        this.connected = false;
        this.connecting = null;
        this.disconnected = true;
      });
      this.disconnected = false;
    }
    this.connecting = new Promise((resolve, reject) => {
      this.client.once("ready", () => {
        this.connected = true;
        this.connecting = null;
        resolve();
      });
      this.client.once("error", (err) => {
        this.connecting = null;
        reject(err);
      });
      this.client.connect(this.config);
    });
    return this.connecting;
  }
  async exec(command) {
    await this.connect();
    return new Promise((resolve, reject) => {
      this.client.exec(command, (err, stream) => {
        if (err) {
          reject(err);
          return;
        }
        let stdout = "";
        let stderr = "";
        stream.on("data", (d) => {
          stdout += d.toString();
        });
        stream.stderr.on("data", (d) => {
          stderr += d.toString();
        });
        stream.on("close", (code) => {
          resolve({ stdout, stderr, code });
        });
      });
    });
  }
  async readFile(remotePath) {
    await this.connect();
    return new Promise((resolve, reject) => {
      this.client.sftp((err, sftp) => {
        if (err) {
          reject(err);
          return;
        }
        const chunks = [];
        const stream = sftp.createReadStream(remotePath);
        stream.on("data", (d) => chunks.push(d));
        stream.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
        stream.on("error", reject);
      });
    });
  }
  async writeFile(remotePath, content) {
    await this.connect();
    return new Promise((resolve, reject) => {
      this.client.sftp((err, sftp) => {
        if (err) {
          reject(err);
          return;
        }
        const stream = sftp.createWriteStream(remotePath);
        stream.on("finish", () => resolve());
        stream.on("error", reject);
        stream.end(content, "utf8");
      });
    });
  }
  close() {
    if (this.connected) {
      this.client.end();
      this.connected = false;
    }
  }
}
class RemoteNginxEngine {
  engineKind = "remote";
  supportsProcessControl = false;
  ssh;
  cfg;
  cachedStatus = "unknown";
  /**
   * @param cfg 远程引擎配置
   * @param sshClient 可选注入的 SshClient（测试用）。生产环境不传，内部 new SshClient。
   *                 解耦后测试可直接传 mock client，不必 mock 整个 @main/ssh/client 模块。
   */
  constructor(cfg, sshClient) {
    this.cfg = cfg;
    if (sshClient) {
      this.ssh = sshClient;
    } else {
      const password = cfg.passwordEncrypted ? decryptPassword(cfg.passwordEncrypted) : void 0;
      this.ssh = new SshClient({
        host: cfg.host,
        port: cfg.port,
        username: cfg.username,
        password
      });
    }
  }
  status() {
    return this.cachedStatus;
  }
  async refreshStatus() {
    try {
      const r = await this.ssh.exec("systemctl is-active nginx 2>/dev/null || pgrep -x nginx >/dev/null && echo active || echo inactive");
      const out = (r.stdout + r.stderr).trim();
      const lines = out.split("\n").map((l) => l.trim());
      if (lines.includes("active") || lines.includes("running")) {
        this.cachedStatus = "running";
      } else {
        this.cachedStatus = "stopped";
      }
    } catch {
      this.cachedStatus = "unknown";
    }
    return this.cachedStatus;
  }
  // remote 不支持进程级启停：远程 nginx 由 systemd/管理员管理。
  // 之前这里是假动作（只 refreshStatus），UI 会显示「已启动」误导用户。现在明示不支持。
  // UI 应根据 supportsProcessControl 隐藏启停按钮；若仍被调用则抛错而非静默成功。
  async start() {
    throw new Error("远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理");
  }
  async stop() {
    throw new Error("远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理");
  }
  async reload() {
    const r = await this.ssh.exec(this.cfg.reloadCmd);
    if (r.code !== 0) {
      throw new Error(`reload 失败 (code ${r.code}): ${r.stderr || r.stdout}`);
    }
  }
  async test() {
    const r = await this.ssh.exec(this.cfg.testCmd);
    const output = (r.stdout + r.stderr).trim();
    return { ok: r.code === 0, output };
  }
  async writeConf(content) {
    await this.ssh.writeFile(this.cfg.confPath, content);
  }
  async readConf() {
    return await this.ssh.readFile(this.cfg.confPath);
  }
  async getPid() {
    try {
      const r = await this.ssh.exec("cat /run/nginx.pid 2>/dev/null || pgrep -xo nginx");
      const out = r.stdout.trim();
      const n = parseInt(out, 10);
      return Number.isFinite(n) && n > 0 ? n : null;
    } catch {
      return null;
    }
  }
  close() {
    this.ssh.close();
  }
}
async function switchEngineToHost(hostId) {
  const host = getHost(hostId);
  if (!host)
    return { ok: false, error: "主机不存在" };
  const paths = resolvePaths();
  const wlEnabled = isWhitelistEnabled();
  if (wlEnabled && getEnabledWhitelistIps().length === 0) {
    return { ok: false, error: "白名单已开启但未配置任何 IP，请先添加 IP 或关闭白名单。" };
  }
  const wlIps = wlEnabled ? getEnabledWhitelistIps() : void 0;
  let engine;
  if (host.type === "local") {
    const nginxPath = host.localConfig?.nginxPath || node_path.join(paths.bundledNginxDir, process.platform === "win32" ? "nginx.exe" : "nginx");
    const prefixDir = node_path.join(paths.dataDir, "nginx-home");
    fsExtra.ensureDirSync(prefixDir);
    const confDir = node_path.join(prefixDir, "conf");
    fsExtra.ensureDirSync(confDir);
    const confPath = node_path.join(confDir, "nginx.conf");
    engine = new LocalNginxEngine({
      nginxPath,
      confPath,
      logDir: paths.nginxLogDir,
      tempDir: paths.tempDir,
      prefixDir
    });
    const bundledMime = node_path.join(paths.bundledNginxDir, "mime.types");
    if (fsExtra.pathExistsSync(bundledMime)) {
      try {
        fsExtra.copySync(bundledMime, node_path.join(confDir, "mime.types"));
      } catch {
      }
    }
    const mappings = listEnabledMappings(host.id);
    const conf = generateFullConf(mappings, {
      whitelistIps: wlIps
    });
    await engine.writeConf(conf);
  } else if (host.type === "remote" && host.remoteConfig) {
    engine = new RemoteNginxEngine({
      host: host.remoteConfig.host,
      port: host.remoteConfig.port,
      username: host.remoteConfig.username,
      passwordEncrypted: host.remoteConfig.passwordEncrypted,
      privateKeyPath: host.remoteConfig.keyPath,
      nginxPath: host.remoteConfig.nginxPath,
      confPath: host.remoteConfig.confPath,
      reloadCmd: host.remoteConfig.reloadCmd,
      testCmd: host.remoteConfig.testCmd
    });
    engine.refreshStatus().catch(() => {
    });
  } else {
    return { ok: false, error: "主机类型无效或缺少远程配置" };
  }
  setActiveEngine(engine, hostId);
  return { ok: true };
}
const EXPORT_VERSION = 2;
function stripInject(v) {
  if (typeof v !== "string")
    return "";
  return v.replace(/[;{}#\n\r]/g, "").trim();
}
function sanitizeImportedHost(h) {
  if (!h || typeof h !== "object" || typeof h.id !== "string" || !h.id)
    return null;
  let remote = null;
  if (h.remoteConfig && typeof h.remoteConfig === "object") {
    remote = {
      host: typeof h.remoteConfig.host === "string" ? h.remoteConfig.host.slice(0, 255) : "",
      port: isValidPort(h.remoteConfig.port) ? h.remoteConfig.port : 22,
      username: typeof h.remoteConfig.username === "string" ? h.remoteConfig.username.slice(0, 64) : "root",
      authType: "password",
      passwordEncrypted: null,
      // 导入文件不可信，凭据必须重填
      keyPath: null,
      // 危险命令字段重置为默认——reloadCmd/testCmd 会被 ssh.exec 原样执行
      reloadCmd: "nginx -s reload",
      testCmd: "nginx -t",
      nginxPath: "/usr/sbin/nginx",
      confPath: "/etc/nginx/nginx.conf"
    };
    if (!remote.host)
      return null;
  }
  let local = null;
  if (h.localConfig && typeof h.localConfig === "object") {
    local = {
      // nginxPath 留空 = 使用应用内置 nginx（hostEngineSwitch 对空值回退 bundled）
      nginxPath: "",
      confDir: "",
      logDir: ""
    };
  }
  return {
    id: h.id,
    name: stripInject(h.name).slice(0, 100) || "导入主机",
    type: h.type === "remote" ? "remote" : "local",
    isDefault: h.isDefault === 1 ? 1 : 0,
    localConfig: local,
    remoteConfig: remote,
    status: "unknown",
    createdAt: typeof h.createdAt === "number" ? h.createdAt : Date.now()
  };
}
function sanitizeImportedMapping(m) {
  if (!m || typeof m !== "object" || typeof m.id !== "string" || !m.id)
    return null;
  if (!isValidProtocol(m.protocol))
    return null;
  if (m.listenPort != null && !isValidPort(m.listenPort))
    return null;
  if (m.upstreamPort != null && !isValidPort(m.upstreamPort))
    return null;
  const proxyMode = isValidProxyMode(m.proxyMode) ? m.proxyMode : "standard";
  const proxyPassUrl = typeof m.proxyPassUrl === "string" && m.proxyPassUrl.trim() ? isValidProxyPassUrl(m.proxyPassUrl) ? m.proxyPassUrl.trim() : null : null;
  return {
    id: m.id,
    hostId: typeof m.hostId === "string" ? m.hostId : "",
    name: stripInject(m.name).slice(0, 100) || "导入映射",
    enabled: m.enabled === 1 ? 1 : m.enabled === 0 ? 0 : 1,
    protocol: m.protocol,
    listenPort: typeof m.listenPort === "number" ? m.listenPort : null,
    serverNames: stripInject(m.serverNames).slice(0, 500) || null,
    listenPath: stripInject(m.listenPath).slice(0, 200) || "/",
    upstreamHost: stripInject(m.upstreamHost).slice(0, 255),
    upstreamPort: typeof m.upstreamPort === "number" ? m.upstreamPort : null,
    tlsCertPath: stripInject(m.tlsCertPath).slice(0, 500) || null,
    tlsKeyPath: stripInject(m.tlsKeyPath).slice(0, 500) || null,
    redirectHttp: m.redirectHttp === 1 ? 1 : 0,
    extraDirectives: typeof m.extraDirectives === "string" ? m.extraDirectives.slice(0, 5e3) : null,
    proxyMode,
    proxyHeaders: proxyMode === "custom" && typeof m.proxyHeaders === "string" ? m.proxyHeaders.replace(/[{}#\r]/g, "").slice(0, 5e3) : null,
    proxyPassUrl,
    healthCheckEnabled: m.healthCheckEnabled === 1 ? 1 : 0,
    healthCheckIntervalSec: typeof m.healthCheckIntervalSec === "number" && m.healthCheckIntervalSec >= 5 ? Math.min(m.healthCheckIntervalSec, 3600) : 30,
    healthCheckTargetPath: stripInject(m.healthCheckTargetPath).slice(0, 500) || null,
    healthCheckExpectedStatus: typeof m.healthCheckExpectedStatus === "number" ? m.healthCheckExpectedStatus : null,
    healthStatus: "unknown",
    lastCheckedAt: null,
    sortOrder: typeof m.sortOrder === "number" ? m.sortOrder : 0,
    createdAt: typeof m.createdAt === "number" ? m.createdAt : Date.now(),
    updatedAt: Date.now()
  };
}
function exportData() {
  const db = getDb();
  const hosts = db.prepare("SELECT * FROM hosts").all();
  const mappings = db.prepare("SELECT * FROM mappings").all();
  const snapshots = db.prepare("SELECT * FROM config_snapshots").all();
  const logs = db.prepare("SELECT * FROM app_logs").all();
  const hostsOut = hosts.map(rowToHost).map((h) => ({
    ...h,
    // 导出文件不含 SSH 密码密文（Windows DPAPI 同用户任意进程可解密，落盘即泄露）
    remoteConfig: h.remoteConfig ? { ...h.remoteConfig, passwordEncrypted: null } : null
  }));
  return {
    version: EXPORT_VERSION,
    exportedAt: Date.now(),
    hosts: hostsOut,
    mappings: mappings.map(rowToMapping),
    snapshots: snapshots.map(rowToSnapshot),
    logs: logs.map(rowToLog)
  };
}
function importData(payload) {
  if (!payload || typeof payload !== "object")
    throw new Error("导入格式错误：非对象");
  if (payload.version !== EXPORT_VERSION && payload.version !== 1) {
    throw new Error(`导入版本不匹配：期望 ${EXPORT_VERSION}，得到 ${payload.version}`);
  }
  if (!Array.isArray(payload.hosts) || !Array.isArray(payload.mappings))
    throw new Error("导入格式错误：缺少 hosts/mappings 数组");
  const db = getDb();
  let hostsImported = 0;
  let mappingsImported = 0;
  let snapshotsImported = 0;
  let logsImported = 0;
  let hostsSkipped = 0;
  let mappingsSkipped = 0;
  const hosts = payload.hosts.map(sanitizeImportedHost);
  const hostIds = new Set(hosts.filter(Boolean).map((h) => h.id));
  const mappings = payload.mappings.map(sanitizeImportedMapping).filter((m) => m && hostIds.has(m.hostId));
  hostsSkipped = payload.hosts.length - hosts.filter(Boolean).length;
  mappingsSkipped = payload.mappings.length - mappings.length;
  db.transaction(() => {
    db.prepare("DELETE FROM app_logs").run();
    db.prepare("DELETE FROM config_snapshots").run();
    db.prepare("DELETE FROM mappings").run();
    db.prepare("DELETE FROM hosts").run();
    for (const h of hosts) {
      if (!h)
        continue;
      db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`).run(h.id, h.name, h.type, h.isDefault, h.localConfig ? JSON.stringify(h.localConfig) : null, h.remoteConfig ? JSON.stringify(h.remoteConfig) : null, h.status, h.createdAt);
      hostsImported++;
    }
    for (const m of mappings) {
      if (!m)
        continue;
      db.prepare(`INSERT INTO mappings (id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
        upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
        proxy_mode, proxy_headers,
        health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
        health_status, last_checked_at, sort_order, created_at, updated_at, proxy_pass_url)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`).run(m.id, m.hostId, m.name, m.enabled, m.protocol, m.listenPort, m.serverNames, m.listenPath, m.upstreamHost, m.upstreamPort, m.tlsCertPath, m.tlsKeyPath, m.redirectHttp, m.extraDirectives, m.proxyMode, m.proxyHeaders, m.healthCheckEnabled, m.healthCheckIntervalSec, m.healthCheckTargetPath, m.healthCheckExpectedStatus, m.healthStatus, m.lastCheckedAt, m.sortOrder, m.createdAt, m.updatedAt, m.proxyPassUrl);
      mappingsImported++;
    }
    if (Array.isArray(payload.snapshots)) {
      for (const s of payload.snapshots) {
        db.prepare(`INSERT INTO config_snapshots (id, host_id, content, reason, created_at) VALUES (?, ?, ?, ?, ?)`).run(s.id, s.hostId, s.content, s.reason, s.createdAt ?? Date.now());
        snapshotsImported++;
      }
    }
    if (Array.isArray(payload.logs)) {
      for (const l of payload.logs) {
        db.prepare(`INSERT INTO app_logs (id, host_id, level, source, message, timestamp) VALUES (?, ?, ?, ?, ?, ?)`).run(l.id, l.hostId, l.level, l.source, l.message, l.timestamp ?? Date.now());
        logsImported++;
      }
    }
  })();
  return { hostsImported, mappingsImported, snapshotsImported, logsImported, hostsSkipped, mappingsSkipped };
}
function rowToHost(r) {
  return {
    id: r.id,
    name: r.name,
    type: r.type,
    isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status,
    createdAt: r.created_at
  };
}
function rowToMapping(r) {
  return {
    id: r.id,
    hostId: r.host_id,
    name: r.name,
    enabled: r.enabled,
    protocol: r.protocol,
    listenPort: r.listen_port,
    serverNames: r.server_names,
    listenPath: r.listen_path,
    upstreamHost: r.upstream_host,
    upstreamPort: r.upstream_port,
    tlsCertPath: r.tls_cert_path,
    tlsKeyPath: r.tls_key_path,
    redirectHttp: r.redirect_http,
    extraDirectives: r.extra_directives,
    proxyMode: r.proxy_mode || "standard",
    proxyHeaders: r.proxy_headers,
    proxyPassUrl: r.proxy_pass_url,
    healthCheckEnabled: r.health_check_enabled,
    healthCheckIntervalSec: r.health_check_interval_sec,
    healthCheckTargetPath: r.health_check_target_path,
    healthCheckExpectedStatus: r.health_check_expected_status,
    healthStatus: r.health_status,
    lastCheckedAt: r.last_checked_at,
    sortOrder: r.sort_order,
    createdAt: r.created_at,
    updatedAt: r.updated_at
  };
}
function rowToSnapshot(r) {
  return {
    id: r.id,
    hostId: r.host_id,
    content: r.content,
    reason: r.reason,
    createdAt: r.created_at
  };
}
function rowToLog(r) {
  return {
    id: r.id,
    hostId: r.host_id,
    level: r.level,
    source: r.source,
    message: r.message,
    timestamp: r.timestamp
  };
}
const CHECK_TIMEOUT_MS = 3e3;
const LOOP_INTERVAL_MS = 3e4;
let loopTimer = null;
function checkTcp(host, port) {
  return new Promise((resolve) => {
    const socket = node_net.createConnection({ host, port }, () => {
      socket.destroy();
      resolve("healthy");
    });
    socket.setTimeout(CHECK_TIMEOUT_MS);
    socket.on("error", () => resolve("unhealthy"));
    socket.on("timeout", () => {
      socket.destroy();
      resolve("unhealthy");
    });
  });
}
function checkHttp(host, port, path, expectedStatus, useHttps = false) {
  const lib = useHttps ? https : http;
  return new Promise((resolve) => {
    const req = lib.get({
      hostname: host,
      port,
      path,
      timeout: CHECK_TIMEOUT_MS,
      // HTTPS upstream 通常自签证书，探活时忽略证书校验（只验证端口通不通 + 状态码）
      rejectUnauthorized: false
    }, (res) => {
      res.destroy();
      resolve(res.statusCode === expectedStatus ? "healthy" : "unhealthy");
    });
    req.on("error", () => resolve("unhealthy"));
    req.on("timeout", () => {
      req.destroy();
      resolve("unhealthy");
    });
  });
}
function checkMapping(m) {
  if (m.protocol === "tcp" || m.protocol === "udp") {
    return checkTcp(m.upstreamHost, m.upstreamPort);
  }
  const path = m.healthCheckTargetPath ?? "/";
  const expected = m.healthCheckExpectedStatus ?? 200;
  return checkHttp(m.upstreamHost, m.upstreamPort, path, expected, m.protocol === "https");
}
async function runOnce() {
  const host = getDefaultHost();
  if (!host)
    return;
  const mappings = listEnabledMappings(host.id).filter((m) => m.healthCheckEnabled === 1);
  const now = Date.now();
  await Promise.all(mappings.map(async (m) => {
    try {
      const status = await checkMapping(m);
      updateHealthStatus(m.id, status, now);
    } catch (e) {
      updateHealthStatus(m.id, "unknown", now);
      logger.warn(host.id, "app", `健康检查异常 ${m.name}: ${e?.message ?? e}`);
    }
  }));
}
function startHealthCheckLoop(intervalMs = LOOP_INTERVAL_MS) {
  stopHealthCheckLoop();
  runOnce().catch(() => {
  });
  loopTimer = setInterval(() => {
    runOnce().catch(() => {
    });
  }, intervalMs);
}
function stopHealthCheckLoop() {
  if (loopTimer) {
    clearInterval(loopTimer);
    loopTimer = null;
  }
}
function isPortAvailable(port) {
  if (port == null)
    return Promise.resolve(true);
  return new Promise((resolve) => {
    const server = node_net.createServer();
    server.once("error", () => resolve(false));
    server.once("listening", () => {
      server.close(() => resolve(true));
    });
    server.listen(port);
  });
}
const TEMPLATE_ID_RE = /^[\w-]{1,64}$/;
function isValidTemplateId(id) {
  return typeof id === "string" && TEMPLATE_ID_RE.test(id);
}
function getBuiltinDir() {
  const paths = resolvePaths();
  const packaged = node_path.join(paths.resourcesDir, "bundled", "templates");
  if (fsExtra.pathExistsSync(packaged))
    return packaged;
  const fromHere = node_path.join(__dirname, "builtin");
  if (fsExtra.pathExistsSync(fromHere))
    return fromHere;
  return node_path.join(__dirname, "..", "..", "..", "packages", "server", "src", "templates", "builtin");
}
function getCustomDir() {
  const paths = resolvePaths();
  const dir = node_path.join(paths.dataDir, "templates", "custom");
  fsExtra.ensureDirSync(dir);
  return dir;
}
function loadYamlFile(filePath, id, builtin) {
  try {
    const content = fsExtra.readFileSync(filePath, "utf-8");
    const raw = yaml.parse(content);
    if (!raw || !raw.mapping || !raw.mapping.protocol)
      return null;
    return {
      id: raw.id ?? id,
      name: raw.name ?? id,
      icon: raw.icon ?? "default",
      category: raw.category ?? "其他",
      description: raw.description ?? "",
      builtin,
      mapping: raw.mapping,
      extraDirectives: raw.extraDirectives
    };
  } catch {
    return null;
  }
}
function listBuiltinTemplates() {
  const dir = getBuiltinDir();
  if (!fsExtra.pathExistsSync(dir))
    return [];
  const files = fsExtra.readdirSync(dir).filter((f) => node_path.extname(f) === ".yml" || node_path.extname(f) === ".yaml");
  const list = [];
  for (const f of files) {
    const id = node_path.basename(f, node_path.extname(f));
    const tpl = loadYamlFile(node_path.join(dir, f), id, true);
    if (tpl)
      list.push(tpl);
  }
  return list.sort((a, b) => a.category.localeCompare(b.category, "zh") || a.name.localeCompare(b.name, "zh"));
}
function listCustomTemplates() {
  const dir = getCustomDir();
  if (!fsExtra.pathExistsSync(dir))
    return [];
  const files = fsExtra.readdirSync(dir).filter((f) => node_path.extname(f) === ".yml" || node_path.extname(f) === ".yaml");
  const list = [];
  for (const f of files) {
    const id = node_path.basename(f, node_path.extname(f));
    const tpl = loadYamlFile(node_path.join(dir, f), id, false);
    if (tpl)
      list.push(tpl);
  }
  return list.sort((a, b) => a.name.localeCompare(b.name, "zh"));
}
function listAllTemplates() {
  return [...listBuiltinTemplates(), ...listCustomTemplates()];
}
function saveCustomTemplate(tpl) {
  if (!isValidTemplateId(tpl.id)) {
    throw new Error("模板 id 非法：仅允许字母、数字、下划线、短横线（1-64 字符）");
  }
  const dir = getCustomDir();
  const data = {
    id: tpl.id,
    name: tpl.name,
    icon: tpl.icon,
    category: tpl.category,
    description: tpl.description,
    mapping: tpl.mapping,
    extraDirectives: tpl.extraDirectives
  };
  fsExtra.outputFileSync(node_path.join(dir, `${tpl.id}.yml`), yaml.stringify(data));
}
function deleteCustomTemplate(id) {
  if (!isValidTemplateId(id))
    return;
  const dir = getCustomDir();
  const file = node_path.join(dir, `${id}.yml`);
  if (fsExtra.pathExistsSync(file))
    fsExtra.removeSync(file);
}
function tailFile(filePath, maxLines) {
  let fd = null;
  try {
    const stat = node_fs.statSync(filePath);
    if (!stat.isFile() || stat.size === 0)
      return [];
    fd = node_fs.openSync(filePath, "r");
    const fileSize = stat.size;
    const CHUNK = 64 * 1024;
    const blocks = [];
    let position = fileSize;
    let newlineCount = 0;
    while (position > 0 && newlineCount <= maxLines) {
      const readSize = Math.min(CHUNK, position);
      position -= readSize;
      const buf = Buffer.alloc(readSize);
      node_fs.readSync(fd, buf, 0, readSize, position);
      blocks.unshift(buf);
      for (let i = 0; i < readSize; i++) {
        if (buf[i] === 10)
          newlineCount++;
      }
    }
    const content = Buffer.concat(blocks).toString("utf-8");
    const lines = content.split("\n").filter((l) => l.length > 0);
    return lines.slice(-maxLines);
  } catch {
    return [];
  } finally {
    if (fd !== null) {
      try {
        node_fs.closeSync(fd);
      } catch {
      }
    }
  }
}
function encryptPlainPassword(input) {
  if (input?.remoteConfig?.passwordEncrypted?.startsWith("PLAIN:")) {
    const plain = input.remoteConfig.passwordEncrypted.slice(6);
    return {
      ...input,
      remoteConfig: {
        ...input.remoteConfig,
        passwordEncrypted: plain ? encryptPassword(plain) : null
      }
    };
  }
  return input;
}
function registerIpcHandlers(ipcMain) {
  ipcMain.handle(API.HOST_LIST, () => listHosts());
  ipcMain.handle(API.HOST_CREATE, (_e, input) => {
    const r = createHost(encryptPlainPassword(input));
    logger.info(null, "app", `新增主机「${input.name}」· ${input.type === "local" ? "本地" : "远程SSH"}`);
    return r;
  });
  ipcMain.handle(API.HOST_UPDATE, (_e, id, patch) => {
    return updateHost(id, encryptPlainPassword(patch));
  });
  ipcMain.handle(API.HOST_DELETE, (_e, id) => {
    const h = listHosts().find((x) => x.id === id);
    deleteHost(id);
    logger.info(null, "app", `删除主机「${h?.name ?? id}」`);
    return true;
  });
  ipcMain.handle(API.HOST_SET_DEFAULT, (_e, id) => {
    setDefaultHost(id);
    return true;
  });
  ipcMain.handle(API.HOST_SWITCH, async (_e, id) => {
    try {
      const r = await switchEngineToHost(id);
      if (r.ok) {
        const h = listHosts().find((x) => x.id === id);
        logger.info(id, "app", `切换到主机「${h?.name ?? id}」`);
      }
      return r;
    } catch (e) {
      return { ok: false, error: e?.message ?? String(e) };
    }
  });
  ipcMain.handle(API.HOST_TEST_CONNECTION, async (_e, remoteConfig) => {
    const host = typeof remoteConfig?.host === "string" ? remoteConfig.host.slice(0, 255) : "";
    if (!host)
      return { ok: false, output: "缺少主机地址" };
    const port = isValidPort(remoteConfig?.port) ? Number(remoteConfig.port) : 22;
    const username = typeof remoteConfig?.username === "string" ? remoteConfig.username.slice(0, 64) : "root";
    try {
      let password;
      if (typeof remoteConfig.passwordEncrypted === "string" && remoteConfig.passwordEncrypted.startsWith("PLAIN:")) {
        password = remoteConfig.passwordEncrypted.slice(6);
      } else if (typeof remoteConfig.passwordEncrypted === "string" && remoteConfig.passwordEncrypted) {
        password = decryptPassword(remoteConfig.passwordEncrypted);
      }
      const client = new SshClient({ host, port, username, password });
      const r = await client.exec("echo ok");
      client.close();
      return { ok: r.code === 0, output: r.code === 0 ? "ok" : "认证失败或命令执行失败" };
    } catch {
      return { ok: false, output: "连接失败（地址不可达或超时）" };
    }
  });
  ipcMain.handle(API.MAPPING_LIST, (_e, hostId) => listMappings(hostId));
  ipcMain.handle(API.MAPPING_CREATE, (_e, input) => {
    if (input.protocol && !isValidProtocol(input.protocol))
      throw new Error(`非法协议: ${input.protocol}`);
    if (input.listenPort != null && !isValidPort(input.listenPort))
      throw new Error(`非法监听端口: ${input.listenPort}`);
    if (input.upstreamPort != null && !isValidPort(input.upstreamPort))
      throw new Error(`非法上游端口: ${input.upstreamPort}`);
    if (input.proxyPassUrl != null && input.proxyPassUrl.trim() && !isValidProxyPassUrl(input.proxyPassUrl))
      throw new Error("proxy_pass 自定义值非法：必须 http:// 或 https:// 开头，不含特殊字符");
    const r = createMapping(input);
    setEngineDirty(true);
    logger.info(input.hostId, "app", `新建映射「${input.name}」· ${(input.protocol || "").toUpperCase()}`);
    return r;
  });
  ipcMain.handle(API.MAPPING_UPDATE, (_e, id, patch) => {
    if (patch.protocol != null && !isValidProtocol(patch.protocol))
      throw new Error(`非法协议: ${patch.protocol}`);
    if (patch.listenPort != null && !isValidPort(patch.listenPort))
      throw new Error(`非法监听端口: ${patch.listenPort}`);
    if (patch.upstreamPort != null && !isValidPort(patch.upstreamPort))
      throw new Error(`非法上游端口: ${patch.upstreamPort}`);
    if (patch.proxyPassUrl != null && patch.proxyPassUrl.trim() && !isValidProxyPassUrl(patch.proxyPassUrl))
      throw new Error("proxy_pass 自定义值非法：必须 http:// 或 https:// 开头，不含特殊字符");
    const m = getMapping(id);
    updateMapping(id, patch);
    setEngineDirty(true);
    const name = patch.name ? `「${patch.name}」` : "";
    logger.info(m?.hostId ?? null, "app", `修改映射 ${name}`);
    return true;
  });
  ipcMain.handle(API.MAPPING_DELETE, (_e, id) => {
    const m = getMapping(id);
    deleteMapping(id);
    setEngineDirty(true);
    logger.info(m?.hostId ?? null, "app", `删除映射「${m?.name ?? id}」`);
    return true;
  });
  ipcMain.handle(API.ENGINE_START, async () => {
    const eng = getActiveEngine();
    if (!eng)
      return { status: "unknown", error: "没有活动的 nginx 引擎（未选中主机或主机非本地类型）" };
    if (!eng.supportsProcessControl)
      return { status: eng.status(), error: "远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理" };
    const host = getDefaultHost();
    try {
      await eng.start();
      logger.info(host?.id ?? null, "app", "引擎启动 · nginx 已开始运行");
      return { status: eng.status(), error: null };
    } catch (e) {
      logger.error(host?.id ?? null, "app", `引擎启动失败 · ${e?.message ?? String(e)}`);
      return { status: eng.status(), error: e?.message ?? String(e) };
    }
  });
  ipcMain.handle(API.ENGINE_STOP, async () => {
    const eng = getActiveEngine();
    if (!eng)
      return { status: "unknown", error: null };
    if (!eng.supportsProcessControl)
      return { status: eng.status(), error: "远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理" };
    const host = getDefaultHost();
    try {
      await eng.stop();
      logger.info(host?.id ?? null, "app", "引擎停止 · 所有转发已失效");
      return { status: eng.status(), error: null };
    } catch (e) {
      logger.error(host?.id ?? null, "app", `引擎停止失败 · ${e?.message ?? String(e)}`);
      return { status: eng.status(), error: e?.message ?? String(e) };
    }
  });
  ipcMain.handle(API.ENGINE_RELOAD, async () => {
    const eng = getActiveEngine();
    if (!eng)
      return { ok: false, output: "no active engine" };
    const hostId = getActiveHostId();
    if (!hostId)
      return { ok: false, output: "no active host" };
    const mappings = listEnabledMappings(hostId);
    const wlEnabled = isWhitelistEnabled();
    const wlIps = wlEnabled ? getEnabledWhitelistIps() : void 0;
    if (wlEnabled && (!wlIps || wlIps.length === 0))
      return { ok: false, output: "白名单已开启但未配置任何 IP。请先添加 IP 或关闭白名单后再应用。" };
    try {
      const result = await applyMappings(eng, mappings, { hostId, whitelistIps: wlIps });
      if (result.ok)
        setEngineDirty(false);
      return { ok: result.ok, output: result.output };
    } catch (e) {
      return { ok: false, output: e?.message ?? String(e) };
    }
  });
  ipcMain.handle(API.ENGINE_TEST, async () => {
    const eng = getActiveEngine();
    return eng ? eng.test() : { ok: false, output: "no engine" };
  });
  ipcMain.handle(API.ENGINE_STATUS, () => getActiveEngine()?.status() ?? "unknown");
  ipcMain.handle(API.ENGINE_DIRTY_GET, () => isEngineDirty());
  ipcMain.handle(API.ENGINE_PID_GET, async () => {
    const eng = getActiveEngine();
    if (!eng || !eng.getPid)
      return null;
    return await Promise.resolve(eng.getPid());
  });
  ipcMain.handle(API.ENGINE_KIND_GET, () => {
    const eng = getActiveEngine();
    if (!eng)
      return null;
    return { engineKind: eng.engineKind, supportsProcessControl: eng.supportsProcessControl };
  });
  ipcMain.handle(API.ENGINE_REFRESH_STATUS, async () => {
    await refreshActiveStatus();
    return getActiveEngine()?.status() ?? "unknown";
  });
  ipcMain.handle(API.TEMPLATE_LIST, () => listAllTemplates());
  ipcMain.handle(API.TEMPLATE_LIST_BUILTIN, () => listBuiltinTemplates());
  ipcMain.handle(API.TEMPLATE_SAVE_CUSTOM, (_e, tpl) => {
    saveCustomTemplate(tpl);
    return true;
  });
  ipcMain.handle(API.TEMPLATE_DELETE_CUSTOM, (_e, id) => {
    deleteCustomTemplate(id);
    return true;
  });
  ipcMain.handle(API.LOG_LIST, (_e, hostId, limit) => {
    return listLogs(hostId, clampInt(limit, 0, 5e3));
  });
  ipcMain.handle(API.LOG_CLEAR, () => {
    clearLogs();
    logger.info(null, "app", "操作日志已清空");
    return true;
  });
  ipcMain.handle(API.NGINX_LOG_TAIL, (_e, lines) => {
    const safeLines = clampInt(lines, 1, 1e4);
    const readRow = getDb().prepare("SELECT value FROM app_settings WHERE key = ?").get("nginx_log_read_enabled");
    if (readRow?.value === "0")
      return { access: [], error: [] };
    const paths = resolvePaths();
    const nginxLogsDir = node_path.join(paths.dataDir, "nginx-home", "logs");
    const result = { access: [], error: [] };
    try {
      const accLines = [];
      for (const f of ["access.log", "stream_access.log"]) {
        const p = node_path.join(nginxLogsDir, f);
        if (fsExtra.pathExistsSync(p))
          accLines.push(...tailFile(p, safeLines));
      }
      result.access = accLines;
    } catch {
    }
    try {
      const errorLog = node_path.join(nginxLogsDir, "error.log");
      if (fsExtra.pathExistsSync(errorLog))
        result.error = tailFile(errorLog, safeLines);
    } catch {
    }
    return result;
  });
  ipcMain.handle(API.NGINX_LOG_CLEAR, () => {
    try {
      const paths = resolvePaths();
      const nginxLogsDir = node_path.join(paths.dataDir, "nginx-home", "logs");
      for (const f of ["access.log", "error.log", "stream_access.log"]) {
        try {
          fsExtra.unlinkSync(node_path.join(nginxLogsDir, f));
        } catch {
        }
      }
      logger.info(null, "app", "Nginx 日志已清空");
      return true;
    } catch {
      return false;
    }
  });
  ipcMain.handle(API.NGINX_LOG_READ_ENABLED_GET, () => {
    const row = getDb().prepare("SELECT value FROM app_settings WHERE key = ?").get("nginx_log_read_enabled");
    return row?.value !== "0";
  });
  ipcMain.handle(API.NGINX_LOG_READ_ENABLED_SET, (_e, enabled) => {
    getDb().prepare("INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value").run("nginx_log_read_enabled", enabled ? "1" : "0");
    return true;
  });
  ipcMain.handle(API.SNAPSHOT_LIST, (_e, hostId) => listSnapshots(hostId, 50));
  ipcMain.handle(API.SNAPSHOT_GET, (_e, id) => getSnapshot(id));
  ipcMain.handle(API.SNAPSHOT_ROLLBACK, async (_e, snapshotId) => {
    const snap = getSnapshot(snapshotId);
    if (!snap)
      return { ok: false, output: "snapshot not found" };
    const eng = getActiveEngine();
    if (!eng)
      return { ok: false, output: "no active engine" };
    const result = await safeApplyConf(eng, snap.content);
    if (result.ok) {
      setEngineDirty(true);
      logger.info(snap.hostId, "app", `手动回滚到快照 ${snap.id} · DB 与 conf 已偏离，点应用会覆盖`);
    } else {
      logger.error(snap.hostId, "app", `回滚到快照 ${snap.id} 失败(${result.failStage ?? "unknown"}): ${result.output}`);
    }
    return result;
  });
  ipcMain.handle(API.HEALTH_CHECK_NOW, async (_e, mappingId) => {
    const m = getMapping(mappingId);
    if (!m)
      return "unknown";
    return await checkMapping(m);
  });
  ipcMain.handle(API.PORT_CHECK, (_e, port) => {
    if (!isValidPort(port))
      return false;
    return isPortAvailable(port);
  });
  ipcMain.handle(API.GET_LOCAL_IPS, () => {
    const os = require("node:os");
    const interfaces = os.networkInterfaces();
    const ips = [];
    for (const name of Object.keys(interfaces)) {
      const addrs = interfaces[name];
      if (!addrs)
        continue;
      for (const addr of addrs) {
        if (addr.family === "IPv4" && !addr.internal)
          ips.push({ interface: name, address: addr.address, family: addr.family });
      }
    }
    return ips;
  });
  ipcMain.handle(API.DATA_WIPE, async (_e, confirmToken) => {
    if (confirmToken !== "CONFIRM_WIPE")
      return { ok: false, output: "缺少确认 token，请通过界面操作" };
    try {
      const eng = getActiveEngine();
      if (eng && eng.status() === "running")
        await eng.stop();
      const paths = resolvePaths();
      closeDb();
      for (const sub of ["data.db", "data.db-shm", "data.db-wal", "nginx-conf", "logs", "snapshots", "temp", "templates", "nginx-home"]) {
        const p = node_path.join(paths.dataDir, sub);
        if (fsExtra.pathExistsSync(p))
          fsExtra.removeSync(p);
      }
      logger.warn(null, "app", "清空所有数据 · 数据库已重置");
      return { ok: true };
    } catch (e) {
      return { ok: false, output: e?.message ?? String(e) };
    }
  });
  ipcMain.handle(API.DATA_EXPORT, () => exportData());
  ipcMain.handle(API.DATA_IMPORT, (_e, payload) => {
    try {
      const result = importData(payload);
      setEngineDirty(true);
      logger.info(null, "app", `数据导入完成 · ${result.hostsImported} 主机 / ${result.mappingsImported} 映射`);
      return { ok: true, ...result };
    } catch (e) {
      return { ok: false, output: e?.message ?? String(e) };
    }
  });
  ipcMain.handle(API.WHITELIST_LIST, () => listWhitelist());
  ipcMain.handle(API.WHITELIST_ADD, (_e, ip, note) => {
    if (!isValidIpOrCidr(ip))
      throw new Error("IP 格式不正确，应为 IPv4(1.2.3.4) 或 CIDR(10.0.0.0/24)");
    const r = addWhitelist(ip, note);
    setEngineDirty(true);
    logger.info(null, "app", `白名单添加 ${ip}`);
    return r;
  });
  ipcMain.handle(API.WHITELIST_REMOVE, (_e, id) => {
    removeWhitelist(id);
    setEngineDirty(true);
    return true;
  });
  ipcMain.handle(API.WHITELIST_TOGGLE_IP, (_e, id, enabled) => {
    toggleWhitelist(id, enabled);
    setEngineDirty(true);
    return true;
  });
  ipcMain.handle(API.WHITELIST_GET_ENABLED, () => isWhitelistEnabled());
  ipcMain.handle(API.WHITELIST_SET_ENABLED, (_e, enabled) => {
    setWhitelistEnabled(enabled);
    setEngineDirty(true);
    logger.info(null, "app", enabled ? "白名单已开启 · 仅允许列表 IP 访问" : "白名单已关闭 · 全部允许");
    return true;
  });
}
function createWindow() {
  const win = new electron.BrowserWindow({
    width: 1100,
    height: 720,
    minWidth: 900,
    minHeight: 600,
    autoHideMenuBar: true,
    webPreferences: {
      preload: node_path.join(__dirname, "../preload/index.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false
    }
  });
  if (process.env.ELECTRON_RENDERER_URL) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL);
  } else {
    win.loadFile(node_path.join(__dirname, "../renderer/index.html"));
  }
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//.test(url))
      electron.shell.openExternal(url);
    return { action: "deny" };
  });
  win.webContents.on("will-navigate", (e, url) => {
    const devServer = process.env.ELECTRON_RENDERER_URL;
    const allowed = url.startsWith("file://") || (devServer ? url.startsWith(devServer) : false);
    if (!allowed) {
      e.preventDefault();
      if (/^https?:\/\//.test(url))
        electron.shell.openExternal(url);
    }
  });
}
electron.app.whenReady().then(async () => {
  try {
    if (process.platform === "win32" && electron.app.isPackaged) {
      const exeDir = node_path.dirname(electron.app.getPath("exe"));
      if (/[^\x00-\x7F]/.test(exeDir)) {
        electron.dialog.showErrorBox(
          "路径包含中文，无法启动",
          `本程序内置的 nginx 不支持包含中文的路径。

当前路径：
${exeDir}

解决方法：把整个程序文件夹复制或移动到纯英文路径下（例如 D:\\nginx-jump\\），再重新运行。`
        );
        electron.app.exit(1);
        return;
      }
    }
    const paths = resolvePaths();
    paths.ensureDataDirs();
    openDb();
    if (listHosts().length === 0) {
      const h = createHost({
        name: "本机网关",
        type: "local",
        isDefault: 1,
        localConfig: { nginxPath: "", confDir: "", logDir: "" },
        remoteConfig: null
      });
      setDefaultHost(h.id);
    }
    registerIpcHandlers(electron.ipcMain);
    const def = getDefaultHost();
    if (def) {
      const r = await switchEngineToHost(def.id);
      if (!r.ok)
        logger.warn(def.id, "app", `启动时切换默认主机失败：${r.error ?? "未知原因"}`);
    }
    createWindow();
    startHealthCheckLoop();
    electron.app.on("activate", () => {
      if (electron.BrowserWindow.getAllWindows().length === 0)
        createWindow();
    });
  } catch (err) {
    console.error("[startup] fatal:", err);
    electron.dialog.showErrorBox(
      "Nginx 跳板机启动失败",
      `启动过程中发生错误：

${err instanceof Error ? err.message : String(err)}

程序将退出。`
    );
    electron.app.exit(1);
  }
});
electron.app.on("window-all-closed", () => {
  electron.app.quit();
});
electron.app.on("before-quit", (e) => {
  const eng = getActiveEngine();
  if (eng && eng.status() === "running") {
    e.preventDefault();
    eng.stop().catch((err) => console.error("[exit] engine.stop failed:", err)).finally(() => electron.app.quit());
    return;
  }
  stopHealthCheckLoop();
  closeDb();
});
