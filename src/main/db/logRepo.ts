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
  // ORDER BY timestamp DESC, rowid DESC：同 timestamp（毫秒精度会重复）时按插入顺序稳定排序
  const sql = hostId
    ? `SELECT * FROM app_logs WHERE host_id = ? ORDER BY timestamp DESC, rowid DESC LIMIT ?`
    : `SELECT * FROM app_logs ORDER BY timestamp DESC, rowid DESC LIMIT ?`
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

/**
 * 删除超出的旧日志，返回删除条数。保留最近 keepCount 条（全局，不分 host）。
 *
 * 用 rowid（隐式自增整数，有索引）而非 id（UUID 无序），比原版的 id NOT IN 快。
 * 曾尝试改 timestamp 范围删除（WHERE timestamp < 阈值），但 Date.now() 毫秒精度
 * 在快速连续写日志时会产生相同 timestamp，导致同 timestamp 的日志被多删或少删，
 * 无法精确保留 keepCount 条。rowid 方案精确且足够快（10000 条规模毫秒级）。
 */
export function trimLogs(keepCount: number): number {
  const db = getDb()
  // ORDER BY timestamp DESC, rowid DESC：同 timestamp 时按 rowid（插入顺序）降序，
  // 保证最新插入的优先保留。不加 rowid DESC 时 SQLite 对同 timestamp 的排序不稳定。
  const result = db.prepare(`DELETE FROM app_logs WHERE rowid NOT IN (
    SELECT rowid FROM app_logs ORDER BY timestamp DESC, rowid DESC LIMIT ?
  )`).run(keepCount)
  return result.changes
}

/** 清空全部操作日志 */
export function clearLogs(): void {
  getDb().prepare('DELETE FROM app_logs').run()
}
