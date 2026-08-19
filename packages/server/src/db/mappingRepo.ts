import type { HealthStatus, Mapping, MappingInput, Protocol } from '@ngj/shared/types'
import { randomUUID } from 'node:crypto'
import { getDb } from './connection'

interface MappingRow {
  id: string
  host_id: string
  name: string
  enabled: number
  protocol: Protocol
  listen_port: number | null
  server_names: string | null
  listen_path: string | null
  upstream_host: string
  upstream_port: number
  tls_cert_path: string | null
  tls_key_path: string | null
  redirect_http: number
  extra_directives: string | null
  proxy_mode: string
  proxy_headers: string | null
  proxy_pass_url: string | null
  health_check_enabled: number
  health_check_interval_sec: number
  health_check_target_path: string | null
  health_check_expected_status: number | null
  health_status: string
  last_checked_at: number | null
  sort_order: number
  created_at: number
  updated_at: number
}

function rowToMapping(r: MappingRow): Mapping {
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
    proxyMode: (r.proxy_mode || 'standard') as 'standard' | 'minimal' | 'custom',
    proxyHeaders: r.proxy_headers,
    proxyPassUrl: r.proxy_pass_url,
    healthCheckEnabled: r.health_check_enabled,
    healthCheckIntervalSec: r.health_check_interval_sec,
    healthCheckTargetPath: r.health_check_target_path,
    healthCheckExpectedStatus: r.health_check_expected_status,
    healthStatus: r.health_status as HealthStatus,
    lastCheckedAt: r.last_checked_at,
    sortOrder: r.sort_order,
    createdAt: r.created_at,
    updatedAt: r.updated_at,
  }
}

const COLUMNS = `id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
  upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
  proxy_mode, proxy_headers, proxy_pass_url,
  health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
  health_status, last_checked_at, sort_order, created_at, updated_at`

export function createMapping(input: MappingInput): Mapping {
  const db = getDb()
  const id = randomUUID()
  const now = Date.now()
  const maxOrder = (db.prepare('SELECT COALESCE(MAX(sort_order), -1) AS m FROM mappings WHERE host_id = ?').get(input.hostId) as { m: number }).m
  db.prepare(`INSERT INTO mappings (${COLUMNS})
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
    .run(id, input.hostId, input.name, input.enabled, input.protocol, input.listenPort, input.serverNames, input.listenPath, input.upstreamHost, input.upstreamPort, input.tlsCertPath, input.tlsKeyPath, input.redirectHttp, input.extraDirectives, input.proxyMode || 'standard', input.proxyHeaders ?? null, input.proxyPassUrl ?? null, input.healthCheckEnabled, input.healthCheckIntervalSec, input.healthCheckTargetPath, input.healthCheckExpectedStatus, 'unknown', null, maxOrder + 1, now, now)
  return getMapping(id)!
}

export function listMappings(hostId: string): Mapping[] {
  const rows = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE host_id = ? ORDER BY sort_order`).all(hostId) as MappingRow[]
  return rows.map(rowToMapping)
}

export function listEnabledMappings(hostId: string): Mapping[] {
  const rows = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE host_id = ? AND enabled = 1 ORDER BY sort_order`).all(hostId) as MappingRow[]
  return rows.map(rowToMapping)
}

export function getMapping(id: string): Mapping | undefined {
  const row = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE id = ?`).get(id) as MappingRow | undefined
  return row ? rowToMapping(row) : undefined
}

export function updateMapping(id: string, patch: Partial<MappingInput>): void {
  const cur = getMapping(id)
  if (!cur)
    throw new Error(`Mapping not found: ${id}`)
  const merged: Mapping = { ...cur, ...patch }
  getDb().prepare(`UPDATE mappings SET
    name=?, enabled=?, protocol=?, listen_port=?, server_names=?, listen_path=?,
    upstream_host=?, upstream_port=?, tls_cert_path=?, tls_key_path=?, redirect_http=?,
    extra_directives=?, proxy_mode=?, proxy_headers=?, proxy_pass_url=?,
    health_check_enabled=?, health_check_interval_sec=?,
    health_check_target_path=?, health_check_expected_status=?, updated_at=? WHERE id=?`).run(merged.name, merged.enabled, merged.protocol, merged.listenPort, merged.serverNames, merged.listenPath, merged.upstreamHost, merged.upstreamPort, merged.tlsCertPath, merged.tlsKeyPath, merged.redirectHttp, merged.extraDirectives, merged.proxyMode || 'standard', merged.proxyHeaders ?? null, merged.proxyPassUrl ?? null, merged.healthCheckEnabled, merged.healthCheckIntervalSec, merged.healthCheckTargetPath, merged.healthCheckExpectedStatus, Date.now(), id)
}

export function deleteMapping(id: string): void {
  getDb().prepare('DELETE FROM mappings WHERE id = ?').run(id)
}

/** 健康检查服务专用：只更新 healthStatus + lastCheckedAt，不动其他字段 */
export function updateHealthStatus(id: string, healthStatus: HealthStatus, lastCheckedAt: number): void {
  getDb().prepare(`UPDATE mappings SET health_status = ?, last_checked_at = ? WHERE id = ?`).run(healthStatus, lastCheckedAt, id)
}
