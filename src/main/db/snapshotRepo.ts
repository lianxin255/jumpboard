import { randomUUID } from 'crypto'
import { getDb } from './connection'
import type { ConfigSnapshot } from '@shared/types'

interface SnapshotRow {
  id: string
  host_id: string
  content: string
  reason: string
  created_at: number
}

function rowToSnapshot(r: SnapshotRow): ConfigSnapshot {
  return {
    id: r.id,
    hostId: r.host_id,
    content: r.content,
    reason: r.reason,
    createdAt: r.created_at
  }
}

export function createSnapshot(input: { hostId: string; content: string; reason: string }): ConfigSnapshot {
  const id = randomUUID()
  const createdAt = Date.now()
  getDb().prepare(`INSERT INTO config_snapshots (id, host_id, content, reason, created_at)
    VALUES (?, ?, ?, ?, ?)`)
    .run(id, input.hostId, input.content, input.reason, createdAt)
  return { id, ...input, createdAt }
}

export function listSnapshots(hostId: string, limit: number): ConfigSnapshot[] {
  const rows = getDb().prepare(`SELECT * FROM config_snapshots WHERE host_id = ? ORDER BY created_at DESC LIMIT ?`)
    .all(hostId, limit) as SnapshotRow[]
  return rows.map(rowToSnapshot)
}

export function getSnapshot(id: string): ConfigSnapshot | undefined {
  const row = getDb().prepare(`SELECT * FROM config_snapshots WHERE id = ?`).get(id) as SnapshotRow | undefined
  return row ? rowToSnapshot(row) : undefined
}

/** 每 host 保留最近 N 条，删除超出的。返回删除条数。用 SQLite 窗口函数。 */
export function trimSnapshots(keepPerHost: number): number {
  const db = getDb()
  const result = db.prepare(`DELETE FROM config_snapshots WHERE id NOT IN (
    SELECT id FROM (
      SELECT id, host_id,
             ROW_NUMBER() OVER (PARTITION BY host_id ORDER BY created_at DESC) AS rn
      FROM config_snapshots
    ) WHERE rn <= ?
  )`).run(keepPerHost)
  return result.changes
}
