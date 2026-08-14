import { addLog, trimLogs } from '../db/logRepo'
import type { LogLevel, LogSource } from '@shared/types'

const MAX_LOGS = 10000

/** 写一条日志。每 MAX_LOGS 条触发一次裁剪。写日志失败不应影响业务（吞错不抛）。 */
export function log(hostId: string | null, level: LogLevel, source: LogSource, message: string): void {
  try {
    addLog({ hostId, level, source, message })
    // 简易裁剪：每次写都检查太频繁，改成概率性裁剪（每 100 条检查一次）
    if (Math.random() < 0.01) trimLogs(MAX_LOGS)
  } catch {
    // DB 锁/磁盘满等导致写日志失败：吞掉，绝不让日志写入击穿 IPC handler
  }
}

export const logger = {
  info: (hostId: string | null, source: LogSource, msg: string) => log(hostId, 'info', source, msg),
  warn: (hostId: string | null, source: LogSource, msg: string) => log(hostId, 'warn', source, msg),
  error: (hostId: string | null, source: LogSource, msg: string) => log(hostId, 'error', source, msg),
}
