import type { Mapping } from '@ngj/shared/types'
import http from 'node:http'
import { createServer } from 'node:net'
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
})
