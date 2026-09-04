import type { Host, HostType, LocalHostConfig, RemoteHostConfig } from '@ngj/shared/types'
import { randomUUID } from 'node:crypto'
import { getDb } from './connection'

interface HostRow {
  id: string
  name: string
  type: HostType
  is_default: number
  local_config: string | null
  remote_config: string | null
  status: string
  created_at: number
}

function rowToHost(r: HostRow): Host {
  return {
    id: r.id,
    name: r.name,
    type: r.type,
    isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status as Host['status'],
    createdAt: r.created_at,
  }
}

export interface CreateHostInput {
  name: string
  type: HostType
  isDefault: number
  localConfig: LocalHostConfig | null
  remoteConfig: RemoteHostConfig | null
}

export function createHost(input: CreateHostInput): Host {
  const db = getDb()
  const id = randomUUID()
  const now = Date.now()
  db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
              VALUES (?, ?, ?, ?, ?, ?, 'unknown', ?)`)
    .run(id, input.name, input.type, input.isDefault, input.localConfig ? JSON.stringify(input.localConfig) : null, input.remoteConfig ? JSON.stringify(input.remoteConfig) : null, now)
  return getHost(id)!
}

export function listHosts(): Host[] {
  const rows = getDb().prepare('SELECT * FROM hosts ORDER BY created_at').all() as HostRow[]
  return rows.map(rowToHost)
}

export function getHost(id: string): Host | undefined {
  const row = getDb().prepare('SELECT * FROM hosts WHERE id = ?').get(id) as HostRow | undefined
  return row ? rowToHost(row) : undefined
}

export function updateHost(id: string, patch: Partial<CreateHostInput>): void {
  const cur = getHost(id)
  if (!cur)
    throw new Error(`Host not found: ${id}`)
  const merged: Host = { ...cur, ...patch }
  getDb().prepare(`UPDATE hosts SET name=?, type=?, is_default=?, local_config=?, remote_config=? WHERE id=?`).run(merged.name, merged.type, merged.isDefault, merged.localConfig ? JSON.stringify(merged.localConfig) : null, merged.remoteConfig ? JSON.stringify(merged.remoteConfig) : null, id)
}

export function deleteHost(id: string): void {
  getDb().prepare('DELETE FROM hosts WHERE id = ?').run(id)
}

export function setDefaultHost(id: string): void {
  const db = getDb()
  db.transaction(() => {
    db.prepare('UPDATE hosts SET is_default = 0').run()
    db.prepare('UPDATE hosts SET is_default = 1 WHERE id = ?').run(id)
  })()
}

export function getDefaultHost(): Host | undefined {
  const row = getDb().prepare('SELECT * FROM hosts WHERE is_default = 1 ORDER BY created_at LIMIT 1').get() as HostRow | undefined
  return row ? rowToHost(row) : undefined
}

/** 无任何主机时补一个默认本地主机（应用首次启动 / DATA_WIPE 自愈共用） */
export function ensureDefaultHost(): Host | null {
  if (listHosts().length > 0)
    return null
  const h = createHost({
    name: '本机网关',
    type: 'local',
    isDefault: 1,
    localConfig: { nginxPath: '', confDir: '', logDir: '' },
    remoteConfig: null,
  })
  setDefaultHost(h.id)
  return h
}
