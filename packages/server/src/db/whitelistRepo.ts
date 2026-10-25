import { randomUUID } from 'node:crypto'
import { getDb } from './connection'

export interface WhitelistEntry {
  id: string
  ip: string
  note: string | null
  enabled: number // 0 | 1
  createdAt: number
}

interface WhitelistRow {
  id: string
  ip: string
  note: string | null
  enabled: number
  created_at: number
}

function rowToEntry(r: WhitelistRow): WhitelistEntry {
  return {
    id: r.id,
    ip: r.ip,
    note: r.note,
    enabled: r.enabled,
    createdAt: r.created_at,
  }
}

/** 列出所有白名单 IP */
export function listWhitelist(): WhitelistEntry[] {
  const rows = getDb().prepare('SELECT * FROM ip_whitelist ORDER BY created_at').all() as unknown as WhitelistRow[]
  return rows.map(rowToEntry)
}

/** 获取已启用的白名单 IP 列表（供 conf 生成用） */
export function getEnabledWhitelistIps(): string[] {
  const rows = getDb().prepare('SELECT ip FROM ip_whitelist WHERE enabled = 1').all() as { ip: string }[]
  return rows.map(r => r.ip)
}

/** 添加白名单 IP */
export function addWhitelist(ip: string, note?: string): WhitelistEntry {
  const entry: WhitelistEntry = {
    id: randomUUID(),
    ip,
    note: note ?? null,
    enabled: 1,
    createdAt: Date.now(),
  }
  getDb().prepare('INSERT INTO ip_whitelist (id, ip, note, enabled, created_at) VALUES (?, ?, ?, ?, ?)').run(entry.id, entry.ip, entry.note, entry.enabled, entry.createdAt)
  return entry
}

/** 删除白名单 IP */
export function removeWhitelist(id: string): void {
  getDb().prepare('DELETE FROM ip_whitelist WHERE id = ?').run(id)
}

/** 切换单条白名单的启用状态 */
export function toggleWhitelist(id: string, enabled: number): void {
  getDb().prepare('UPDATE ip_whitelist SET enabled = ? WHERE id = ?').run(enabled, id)
}

// ── app_settings：白名单全局开关 ──

/** 白名单功能是否启用（开启后只允许白名单 IP 访问，关闭=全部允许） */
export function isWhitelistEnabled(): boolean {
  const row = getDb().prepare('SELECT value FROM app_settings WHERE key = ?').get('whitelist_enabled') as unknown as { value: string } | undefined
  return row?.value === '1'
}

/** 设置白名单全局开关 */
export function setWhitelistEnabled(enabled: boolean): void {
  const db = getDb()
  db.prepare('INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?')
    .run('whitelist_enabled', enabled ? '1' : '0', enabled ? '1' : '0')
}
