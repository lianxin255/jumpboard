import type { HealthStatus, Mapping } from '@ngj/shared/types'
import http from 'node:http'
import https from 'node:https'
import { createConnection } from 'node:net'
import { getDefaultHost } from '../db/hostRepo'
import { listEnabledMappings, updateHealthStatus } from '../db/mappingRepo'
import { getActiveHostId } from '../nginx/engineRegistry'
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

/** HTTP/HTTPS 探活：GET path，检查状态码。按 useHttps 选择模块 */
export function checkHttp(host: string, port: number, path: string, expectedStatus: number, useHttps = false): Promise<HealthStatus> {
  const lib = useHttps ? https : http
  return new Promise((resolve) => {
    const req = lib.get({
      hostname: host,
      port,
      path,
      timeout: CHECK_TIMEOUT_MS,
      // HTTPS upstream 通常自签证书，探活时忽略证书校验（只验证端口通不通 + 状态码）
      rejectUnauthorized: false,
    }, (res) => {
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
  // http/https：https 协议必须用 https 模块，否则明文打 443 必然握手失败
  const path = m.healthCheckTargetPath ?? '/'
  const expected = m.healthCheckExpectedStatus ?? 200
  return checkHttp(m.upstreamHost, m.upstreamPort, path, expected, m.protocol === 'https')
}

/** 轮询所有 enabled + healthCheckEnabled 的映射（并行，提速避免串行阻塞） */
async function runOnce(): Promise<void> {
  // 与活动引擎保持一致（用户当前在管理的主机），无活动引擎时退回默认主机
  const hostId = getActiveHostId() ?? getDefaultHost()?.id
  if (!hostId)
    return
  const mappings = listEnabledMappings(hostId).filter(m => m.healthCheckEnabled === 1)
  const now = Date.now()
  // 并行探活：串行时每条最多 3s，映射多会导致一轮远超间隔；并行则一轮≈最慢的一条
  await Promise.all(mappings.map(async (m) => {
    try {
      const status = await checkMapping(m)
      updateHealthStatus(m.id, status, now)
    }
    catch (e: any) {
      updateHealthStatus(m.id, 'unknown', now)
      logger.warn(hostId, 'app', `健康检查异常 ${m.name}: ${e?.message ?? e}`)
    }
  }))
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
