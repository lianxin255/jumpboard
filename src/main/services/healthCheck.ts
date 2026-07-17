import { createConnection } from 'net'
import http from 'http'
import type { Mapping, HealthStatus } from '@shared/types'
import { listEnabledMappings, updateHealthStatus } from '../db/mappingRepo'
import { getDefaultHost } from '../db/hostRepo'
import { logger } from './logger'

const CHECK_TIMEOUT_MS = 3000
const LOOP_INTERVAL_MS = 30000
let loopTimer: NodeJS.Timeout | null = null

/** TCP 探活：尝试建立 TCP 连接 */
export function checkTcp(host: string, port: number): Promise<HealthStatus> {
  return new Promise((resolve) => {
    const socket = createConnection({ host, port }, () => {
      socket.destroy()
      resolve('healthy')
    })
    socket.setTimeout(CHECK_TIMEOUT_MS)
    socket.on('error', () => resolve('unhealthy'))
    socket.on('timeout', () => { socket.destroy(); resolve('unhealthy') })
  })
}

/** HTTP 探活：GET path，检查状态码 */
export function checkHttp(host: string, port: number, path: string, expectedStatus: number): Promise<HealthStatus> {
  return new Promise((resolve) => {
    const req = http.get({ hostname: host, port, path, timeout: CHECK_TIMEOUT_MS }, (res) => {
      res.destroy()
      resolve(res.statusCode === expectedStatus ? 'healthy' : 'unhealthy')
    })
    req.on('error', () => resolve('unhealthy'))
    req.on('timeout', () => { req.destroy(); resolve('unhealthy') })
  })
}

/** 对单条映射做健康检查，返回新状态 */
export function checkMapping(m: Mapping): Promise<HealthStatus> {
  if (m.protocol === 'tcp' || m.protocol === 'udp') {
    // TCP/UDP 都用 TCP 连接探测（UDP 无连接，nginx stream UDP 探活用 TCP 近似）
    return checkTcp(m.upstreamHost, m.upstreamPort)
  }
  // http/https
  const path = m.healthCheckTargetPath ?? '/'
  const expected = m.healthCheckExpectedStatus ?? 200
  return checkHttp(m.upstreamHost, m.upstreamPort, path, expected)
}

/** 轮询所有 enabled + healthCheckEnabled 的映射 */
async function runOnce(): Promise<void> {
  const host = getDefaultHost()
  if (!host) return
  const mappings = listEnabledMappings(host.id).filter(m => m.healthCheckEnabled === 1)
  const now = Date.now()
  for (const m of mappings) {
    try {
      const status = await checkMapping(m)
      updateHealthStatus(m.id, status, now)
    } catch (e: any) {
      updateHealthStatus(m.id, 'unknown', now)
      logger.warn(host.id, 'app', `健康检查异常 ${m.name}: ${e?.message ?? e}`)
    }
  }
}

/** 启动定时轮询 */
export function startHealthCheckLoop(intervalMs: number = LOOP_INTERVAL_MS): void {
  stopHealthCheckLoop()
  runOnce().catch(() => {})
  loopTimer = setInterval(() => { runOnce().catch(() => {}) }, intervalMs)
}

/** 停止轮询 */
export function stopHealthCheckLoop(): void {
  if (loopTimer) {
    clearInterval(loopTimer)
    loopTimer = null
  }
}
