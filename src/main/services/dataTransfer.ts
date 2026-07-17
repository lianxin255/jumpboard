import { getDb } from '../db/connection'
import type { Host, Mapping } from '@shared/types'

const EXPORT_VERSION = 1

export interface ExportPayload {
  version: number
  exportedAt: number
  hosts: Host[]
  mappings: Mapping[]
}

export function exportData(): ExportPayload {
  const db = getDb()
  const hosts = db.prepare('SELECT * FROM hosts').all() as any[]
  const mappings = db.prepare('SELECT * FROM mappings').all() as any[]
  return {
    version: EXPORT_VERSION,
    exportedAt: Date.now(),
    hosts: hosts.map(rowToHost),
    mappings: mappings.map(rowToMapping)
  }
}

export interface ImportResult {
  hostsImported: number
  mappingsImported: number
}

export function importData(payload: Partial<ExportPayload>): ImportResult {
  if (!payload || typeof payload !== 'object') throw new Error('导入格式错误：非对象')
  if (payload.version !== EXPORT_VERSION) throw new Error(`导入版本不匹配：期望 ${EXPORT_VERSION}，得到 ${payload.version}`)
  if (!Array.isArray(payload.hosts) || !Array.isArray(payload.mappings)) throw new Error('导入格式错误：缺少 hosts/mappings 数组')

  const db = getDb()
  let hostsImported = 0
  let mappingsImported = 0
  db.transaction(() => {
    db.prepare('DELETE FROM mappings').run()
    db.prepare('DELETE FROM hosts').run()
    for (const h of payload.hosts!) {
      db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
        .run(h.id, h.name, h.type, h.isDefault ?? 0,
          h.localConfig ? JSON.stringify(h.localConfig) : null,
          h.remoteConfig ? JSON.stringify(h.remoteConfig) : null,
          h.status ?? 'unknown', h.createdAt ?? Date.now())
      hostsImported++
    }
    for (const m of payload.mappings!) {
      db.prepare(`INSERT INTO mappings (id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
        upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
        health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
        health_status, last_checked_at, sort_order, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
        .run(m.id, m.hostId, m.name, m.enabled ?? 1, m.protocol, m.listenPort, m.serverNames, m.listenPath,
          m.upstreamHost, m.upstreamPort, m.tlsCertPath, m.tlsKeyPath, m.redirectHttp ?? 0, m.extraDirectives,
          m.healthCheckEnabled ?? 0, m.healthCheckIntervalSec ?? 30, m.healthCheckTargetPath, m.healthCheckExpectedStatus,
          m.healthStatus ?? 'unknown', m.lastCheckedAt, m.sortOrder ?? 0, m.createdAt ?? Date.now(), m.updatedAt ?? Date.now())
      mappingsImported++
    }
  })()
  return { hostsImported, mappingsImported }
}

function rowToHost(r: any): Host {
  return {
    id: r.id, name: r.name, type: r.type, isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status, createdAt: r.created_at
  }
}

function rowToMapping(r: any): Mapping {
  return {
    id: r.id, hostId: r.host_id, name: r.name, enabled: r.enabled, protocol: r.protocol,
    listenPort: r.listen_port, serverNames: r.server_names, listenPath: r.listen_path,
    upstreamHost: r.upstream_host, upstreamPort: r.upstream_port,
    tlsCertPath: r.tls_cert_path, tlsKeyPath: r.tls_key_path, redirectHttp: r.redirect_http,
    extraDirectives: r.extra_directives,
    healthCheckEnabled: r.health_check_enabled, healthCheckIntervalSec: r.health_check_interval_sec,
    healthCheckTargetPath: r.health_check_target_path, healthCheckExpectedStatus: r.health_check_expected_status,
    healthStatus: r.health_status, lastCheckedAt: r.last_checked_at,
    sortOrder: r.sort_order, createdAt: r.created_at, updatedAt: r.updated_at
  }
}
