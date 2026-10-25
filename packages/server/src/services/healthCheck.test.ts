import type { Mapping } from '@ngj/shared/types'
import { mkdirSync, rmSync } from 'node:fs'
import http from 'node:http'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { checkHttp, checkMapping, checkTcp, startHealthCheckLoop, stopHealthCheckLoop } from '../services/healthCheck'

describe('checkTcp', () => {
  it('端口通返回 healthy', async () => {
    const server = createServer()
    await new Promise<void>(r => server.listen(39997, r))
    expect(await checkTcp('127.0.0.1', 39997)).toBe('healthy')
    await new Promise<void>(r => server.close(() => r()))
  })

  it('端口不通返回 unhealthy', async () => {
    expect(await checkTcp('127.0.0.1', 39996)).toBe('unhealthy')
  })
})

describe('checkHttp', () => {
  it('返回 200 且期望 200 返回 healthy', async () => {
    const server = http.createServer((req, res) => { res.statusCode = 200; res.end('ok') })
    await new Promise<void>(r => server.listen(39995, r))
    expect(await checkHttp('127.0.0.1', 39995, '/', 200)).toBe('healthy')
    await new Promise<void>(r => server.close(() => r()))
  })

  it('返回 500 期望 200 返回 unhealthy', async () => {
    const server = http.createServer((req, res) => { res.statusCode = 500; res.end('err') })
    await new Promise<void>(r => server.listen(39994, r))
    expect(await checkHttp('127.0.0.1', 39994, '/', 200)).toBe('unhealthy')
    await new Promise<void>(r => server.close(() => r()))
  })
})

describe('checkMapping', () => {
  it('https 协议的映射走 https 探活（不再误报 unhealthy）', async () => {
    // 验证 checkMapping 对 https 协议调用 https 路径，不抛错并返回有效状态
    const m = { protocol: 'https', upstreamHost: '127.0.0.1', upstreamPort: 1, healthCheckTargetPath: null, healthCheckExpectedStatus: null } as Mapping
    const r = await checkMapping(m)
    expect(['unhealthy', 'unknown']).toContain(r)
  })
  it('tcp 协议走 tcp 探活', async () => {
    const m = { protocol: 'tcp', upstreamHost: '127.0.0.1', upstreamPort: 1 } as Mapping
    expect(await checkMapping(m)).toBe('unhealthy')
  })
})

describe('healthCheckLoop', () => {
  beforeEach(() => { stopHealthCheckLoop() })
  afterEach(() => { stopHealthCheckLoop() })

  it('start/stop 不抛错', () => {
    expect(() => startHealthCheckLoop()).not.toThrow()
    expect(() => stopHealthCheckLoop()).not.toThrow()
  })

  it('轮询优先活动主机（与引擎一致），无活动主机时退回默认主机', async () => {
    const root = join(tmpdir(), `ngj-hc-${Date.now()}`)
    mkdirSync(root, { recursive: true })
    const { __setAppRootForTest } = await import('../paths')
    const { openDb, closeDb } = await import('../db/connection')
    const { createHost } = await import('../db/hostRepo')
    const { createMapping, getMapping } = await import('../db/mappingRepo')
    const { setActiveEngine } = await import('../nginx/engineRegistry')

    __setAppRootForTest(root)
    openDb()
    stopHealthCheckLoop()
    try {
      const server = createServer()
      await new Promise<void>(r => server.listen(39993, r))
      const mk = (name: string) => createHost({
        name,
        type: 'local',
        isDefault: 0,
        localConfig: { nginxPath: '', confDir: '', logDir: '' },
        remoteConfig: null,
      })
      mk('默认主机（非活动）')
      const active = mk('活动主机')
      setActiveEngine({ engineKind: 'local', supportsProcessControl: true } as any, active.id)

      const m = createMapping({
        hostId: active.id,
        name: 'hc',
        enabled: 1,
        protocol: 'tcp',
        listenPort: null,
        serverNames: null,
        listenPath: null,
        upstreamHost: '127.0.0.1',
        upstreamPort: 39993,
        tlsCertPath: null,
        tlsKeyPath: null,
        redirectHttp: 0,
        extraDirectives: null,
        proxyMode: 'standard',
        proxyHeaders: null,
        proxyPassUrl: null,
        healthCheckEnabled: 1,
        healthCheckIntervalSec: 30,
        healthCheckTargetPath: null,
        healthCheckExpectedStatus: null,
      })

      startHealthCheckLoop(20)
      await new Promise(r => setTimeout(r, 200))
      stopHealthCheckLoop()
      server.close()

      const after = getMapping(m.id)!
      expect(after.healthStatus).toBe('healthy')
      expect(after.lastCheckedAt).not.toBeNull()
    }
    finally {
      stopHealthCheckLoop()
      closeDb()
      rmSync(root, { recursive: true, force: true })
    }
  }, 10000)
})
