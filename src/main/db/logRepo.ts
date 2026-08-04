import { randomUUID } from 'crypto'
import { getDb } from './connection'
import type { LogEntry, LogLevel, LogSource } from '@shared/types'

interface LogRow {
  id: string
  host_id: string | null
  level: string
  source: string
  message: string
  timestamp: number
}

function rowToLog(r: LogRow): LogEntry {
  return {
    id: r.id,
    hostId: r.host_id,
    level: r.level as LogLevel,
    source: r.source as LogSource,
    message: r.message,
    timestamp: r.timestamp
  }
}

export function addLog(input: { hostId: string | null; level: LogLevel; source: LogSource; message: string }): LogEntry {
  const id = randomUUID()
  const timestamp = Date.now()
  getDb().prepare(`INSERT INTO app_logs (id, host_id, level, source, message, timestamp)
    VALUES (?, ?, ?, ?, ?, ?)`)
    .run(id, input.hostId, input.level, input.source, input.message, timestamp)
  return { id, ...input, timestamp }
}

export function listLogs(hostId: string | null, limit: number): LogEntry[] {
  const sql = hostId
    ? `SELECT * FROM app_logs WHERE host_id = ? ORDER BY timestamp DESC LIMIT ?`
    : `SELECT * FROM app_logs ORDER BY timestamp DESC LIMIT ?`
  const rows = hostId
    ? getDb().prepare(sql).all(hostId, limit) as LogRow[]
    : getDb().prepare(sql).all(limit) as LogRow[]
  return rows.map(rowToLog)
}

export function listLogsByLevel(hostId: string | null, level: LogLevel, limit: number): LogEntry[] {
  const sql = hostId
    ? `SELECT * FROM app_logs WHERE host_id = ? AND level = ? ORDER BY timestamp DESC LIMIT ?`
    : `SELECT * FROM app_logs WHERE level = ? ORDER BY timestamp DESC LIMIT ?`
  const rows = hostId
    ? getDb().prepare(sql).all(hostId, level, limit) as LogRow[]
    : getDb().prepare(sql).all(level, limit) as LogRow[]
  return rows.map(rowToLog)
}

/** 删除超出的旧日志，返回删除条数。保留最近 N 条（全局，不分 host） */
export function trimLogs(keepCount: number): number {
  const db = getDb()
  const result = db.prepare(`DELETE FROM app_logs WHERE id NOT IN (
    SELECT id FROM app_logs ORDER BY timestamp DESC LIMIT ?
  )`).run(keepCount)
  return result.changes
}

/** 清空全部操作日志 */
export function clearLogs(): void {
  getDb().prepare('DELETE FROM app_logs').run()
}
