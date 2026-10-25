import type { ConfigSnapshot, Host, LogEntry, Mapping } from '@ngj/shared/types'
import { isValidPort, isValidProtocol, isValidProxyMode, isValidProxyPassUrl } from '@ngj/shared/validation'
import { getDb, withTransaction } from '../db/connection'

const EXPORT_VERSION = 2

/** conf 注入字符（与 renderers.ts sanitize 一致）：name/listenPath 等自由文本入 conf 前剔除 */
function stripInject(v: unknown): string {
  if (typeof v !== 'string')
    return ''
  return v.replace(/[;{}#\n\r]/g, '').trim()
}

/**
 * 导入的 host 消毒（防恶意备份注入）：
 * - remoteConfig 的 reloadCmd/testCmd 重置为默认（它们会被 ssh.exec 直接执行，导入文件不可信）
 * - nginxPath/confPath/localConfig.nginxPath 重置（防 spawn 任意本地二进制 / SFTP 覆盖任意远程文件）
 * - passwordEncrypted 剔除（导出已不带；若恶意文件带也拒收，防 DPAPI 同用户解密攻击链）
 * - host/port/username 收敛为合法标量
 */
function sanitizeImportedHost(h: any): any | null {
  if (!h || typeof h !== 'object' || typeof h.id !== 'string' || !h.id)
    return null
  let remote: any = null
  if (h.remoteConfig && typeof h.remoteConfig === 'object') {
    remote = {
      host: typeof h.remoteConfig.host === 'string' ? h.remoteConfig.host.slice(0, 255) : '',
      port: isValidPort(h.remoteConfig.port) ? h.remoteConfig.port : 22,
      username: typeof h.remoteConfig.username === 'string' ? h.remoteConfig.username.slice(0, 64) : 'root',
      authType: 'password',
      passwordEncrypted: null, // 导入文件不可信，凭据必须重填
      keyPath: null,
      // 危险命令字段重置为默认——reloadCmd/testCmd 会被 ssh.exec 原样执行
      reloadCmd: 'nginx -s reload',
      testCmd: 'nginx -t',
      nginxPath: '/usr/sbin/nginx',
      confPath: '/etc/nginx/nginx.conf',
    }
    if (!remote.host)
      return null
  }
  let local: any = null
  if (h.localConfig && typeof h.localConfig === 'object') {
    local = {
      // nginxPath 留空 = 使用应用内置 nginx（hostEngineSwitch 对空值回退 bundled）
      nginxPath: '',
      confDir: '',
      logDir: '',
    }
  }
  return {
    id: h.id,
    name: stripInject(h.name).slice(0, 100) || '导入主机',
    type: h.type === 'remote' ? 'remote' : 'local',
    isDefault: h.isDefault === 1 ? 1 : 0,
    localConfig: local,
    remoteConfig: remote,
    status: 'unknown',
    createdAt: typeof h.createdAt === 'number' ? h.createdAt : Date.now(),
  }
}

/**
 * 导入的 mapping 消毒：protocol/port/proxyMode/proxyPassUrl 走同一套 validation（与 handler 一致），
 * name/listenPath/serverNames/tls 路径剔除注入字符。非法记录返回 null（跳过）。
 */
function sanitizeImportedMapping(m: any): any | null {
  if (!m || typeof m !== 'object' || typeof m.id !== 'string' || !m.id)
    return null
  if (!isValidProtocol(m.protocol))
    return null
  if (m.listenPort != null && !isValidPort(m.listenPort))
    return null
  if (m.upstreamPort != null && !isValidPort(m.upstreamPort))
    return null
  const proxyMode = isValidProxyMode(m.proxyMode) ? m.proxyMode : 'standard'
  const proxyPassUrl = (typeof m.proxyPassUrl === 'string' && m.proxyPassUrl.trim())
    ? (isValidProxyPassUrl(m.proxyPassUrl) ? m.proxyPassUrl.trim() : null)
    : null
  return {
    id: m.id,
    hostId: typeof m.hostId === 'string' ? m.hostId : '',
    name: stripInject(m.name).slice(0, 100) || '导入映射',
    enabled: m.enabled === 1 ? 1 : (m.enabled === 0 ? 0 : 1),
    protocol: m.protocol,
    listenPort: typeof m.listenPort === 'number' ? m.listenPort : null,
    serverNames: stripInject(m.serverNames).slice(0, 500) || null,
    listenPath: stripInject(m.listenPath).slice(0, 200) || '/',
    upstreamHost: stripInject(m.upstreamHost).slice(0, 255),
    upstreamPort: typeof m.upstreamPort === 'number' ? m.upstreamPort : null,
    tlsCertPath: stripInject(m.tlsCertPath).slice(0, 500) || null,
    tlsKeyPath: stripInject(m.tlsKeyPath).slice(0, 500) || null,
    redirectHttp: m.redirectHttp === 1 ? 1 : 0,
    extraDirectives: typeof m.extraDirectives === 'string' ? m.extraDirectives.slice(0, 5000) : null,
    proxyMode,
    proxyHeaders: proxyMode === 'custom' && typeof m.proxyHeaders === 'string'
      ? m.proxyHeaders.replace(/[{}#\r]/g, '').slice(0, 5000)
      : null,
    proxyPassUrl,
    healthCheckEnabled: m.healthCheckEnabled === 1 ? 1 : 0,
    healthCheckIntervalSec: typeof m.healthCheckIntervalSec === 'number' && m.healthCheckIntervalSec >= 5 ? Math.min(m.healthCheckIntervalSec, 3600) : 30,
    healthCheckTargetPath: stripInject(m.healthCheckTargetPath).slice(0, 500) || null,
    healthCheckExpectedStatus: typeof m.healthCheckExpectedStatus === 'number' ? m.healthCheckExpectedStatus : null,
    healthStatus: 'unknown',
    lastCheckedAt: null,
    sortOrder: typeof m.sortOrder === 'number' ? m.sortOrder : 0,
    createdAt: typeof m.createdAt === 'number' ? m.createdAt : Date.now(),
    updatedAt: Date.now(),
  }
}

export interface ExportPayload {
  version: number
  exportedAt: number
  hosts: Host[]
  mappings: Mapping[]
  snapshots?: ConfigSnapshot[]
  logs?: LogEntry[]
}

export function exportData(): ExportPayload {
  const db = getDb()
  const hosts = db.prepare('SELECT * FROM hosts').all() as any[]
  const mappings = db.prepare('SELECT * FROM mappings').all() as any[]
  const snapshots = db.prepare('SELECT * FROM config_snapshots').all() as any[]
  const logs = db.prepare('SELECT * FROM app_logs').all() as any[]
  const hostsOut = hosts.map(rowToHost).map(h => ({
    ...h,
    // 导出文件不含 SSH 密码密文（Windows DPAPI 同用户任意进程可解密，落盘即泄露）
    remoteConfig: h.remoteConfig ? { ...h.remoteConfig, passwordEncrypted: null } : null,
  }))
  return {
    version: EXPORT_VERSION,
    exportedAt: Date.now(),
    hosts: hostsOut,
    mappings: mappings.map(rowToMapping),
    snapshots: snapshots.map(rowToSnapshot),
    logs: logs.map(rowToLog),
  }
}

export interface ImportResult {
  hostsImported: number
  mappingsImported: number
  snapshotsImported: number
  logsImported: number
  /** 因校验不过被跳过的条数（恶意/损坏的导入内容） */
  hostsSkipped?: number
  mappingsSkipped?: number
}

/**
 * 导入数据：外部文件，字段可能不全（缺少的由 sanitize 补默认值）。
 * 用 any 入参而不是 Partial<Mapping>：导入数据来自不可信的外部文件，
 * sanitizeImported* 函数逐条校验+补全，类型安全由运行时检查保证而非编译期。
 */
export function importData(payload: any): ImportResult {
  if (!payload || typeof payload !== 'object')
    throw new Error('导入格式错误：非对象')
  // 兼容 v1（无 snapshots/logs 字段）：v1 文件只含 hosts/mappings
  if (payload.version !== EXPORT_VERSION && payload.version !== 1) {
    throw new Error(`导入版本不匹配：期望 ${EXPORT_VERSION}，得到 ${payload.version}`)
  }
  if (!Array.isArray(payload.hosts) || !Array.isArray(payload.mappings))
    throw new Error('导入格式错误：缺少 hosts/mappings 数组')

  const db = getDb()
  let hostsImported = 0; let mappingsImported = 0; let snapshotsImported = 0; let logsImported = 0
  let hostsSkipped = 0; let mappingsSkipped = 0
  // 导入前逐条消毒：恶意备份文件可注入 reloadCmd（RCE）/nginxPath（本地执行）/conf 注入。
  // 非法记录跳过不中断（导入是全量替换，失败一条不应毁掉全部）。
  const hosts = (payload.hosts as any[]).map(sanitizeImportedHost)
  const hostIds = new Set(hosts.filter(Boolean).map((h: any) => h!.id))
  const mappings = (payload.mappings as any[])
    .map(sanitizeImportedMapping)
    // hostId 不在导入的主机集合里的 mapping 无法生效，跳过
    .filter((m: any) => m && hostIds.has(m.hostId))
  hostsSkipped = payload.hosts!.length - hosts.filter(Boolean).length
  mappingsSkipped = payload.mappings!.length - mappings.length

  withTransaction(db, () => {
    // 清空所有业务表（导入是全量替换语义）。按外键依赖顺序删。
    db.prepare('DELETE FROM app_logs').run()
    db.prepare('DELETE FROM config_snapshots').run()
    db.prepare('DELETE FROM mappings').run()
    db.prepare('DELETE FROM hosts').run()

    for (const h of hosts) {
      if (!h)
        continue
      db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
        .run(h.id, h.name, h.type, h.isDefault, h.localConfig ? JSON.stringify(h.localConfig) : null, h.remoteConfig ? JSON.stringify(h.remoteConfig) : null, h.status, h.createdAt)
      hostsImported++
    }
    for (const m of mappings) {
      if (!m)
        continue
      db.prepare(`INSERT INTO mappings (id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
        upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
        proxy_mode, proxy_headers,
        health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
        health_status, last_checked_at, sort_order, created_at, updated_at, proxy_pass_url)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
        .run(m.id, m.hostId, m.name, m.enabled, m.protocol, m.listenPort, m.serverNames, m.listenPath, m.upstreamHost, m.upstreamPort, m.tlsCertPath, m.tlsKeyPath, m.redirectHttp, m.extraDirectives, m.proxyMode, m.proxyHeaders, m.healthCheckEnabled, m.healthCheckIntervalSec, m.healthCheckTargetPath, m.healthCheckExpectedStatus, m.healthStatus, m.lastCheckedAt, m.sortOrder, m.createdAt, m.updatedAt, m.proxyPassUrl)
      mappingsImported++
    }
    // v2 新增：恢复快照和日志（v1 文件没有这两个字段，跳过）
    if (Array.isArray(payload.snapshots)) {
      for (const s of payload.snapshots) {
        db.prepare(`INSERT INTO config_snapshots (id, host_id, content, reason, created_at) VALUES (?, ?, ?, ?, ?)`)
          .run(s.id, s.hostId, s.content, s.reason, s.createdAt ?? Date.now())
        snapshotsImported++
      }
    }
    if (Array.isArray(payload.logs)) {
      for (const l of payload.logs) {
        db.prepare(`INSERT INTO app_logs (id, host_id, level, source, message, timestamp) VALUES (?, ?, ?, ?, ?, ?)`)
          .run(l.id, l.hostId, l.level, l.source, l.message, l.timestamp ?? Date.now())
        logsImported++
      }
    }
  })
  return { hostsImported, mappingsImported, snapshotsImported, logsImported, hostsSkipped, mappingsSkipped }
}

function rowToHost(r: any): Host {
  return {
    id: r.id,
    name: r.name,
    type: r.type,
    isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status,
    createdAt: r.created_at,
  }
}

function rowToMapping(r: any): Mapping {
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
    proxyMode: r.proxy_mode || 'standard',
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
    updatedAt: r.updated_at,
  }
}

function rowToSnapshot(r: any): ConfigSnapshot {
  return {
    id: r.id,
    hostId: r.host_id,
    content: r.content,
    reason: r.reason,
    createdAt: r.created_at,
  }
}

function rowToLog(r: any): LogEntry {
  return {
    id: r.id,
    hostId: r.host_id,
    level: r.level,
    source: r.source,
    message: r.message,
    timestamp: r.timestamp,
  }
}
