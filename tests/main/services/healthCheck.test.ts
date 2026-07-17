import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { checkTcp, checkHttp, startHealthCheckLoop, stopHealthCheckLoop } from '@main/services/healthCheck'
import { createServer } from 'net'
import http from 'http'

describe('checkTcp', () => {
  it('端口通返回 healthy', async () => {
    const server = createServer()
    await new Promise<void>((r) => server.listen(39997, r))
    expect(await checkTcp('127.0.0.1', 39997)).toBe('healthy')
    await new Promise<void>((r) => server.close(() => r()))
  })

  it('端口不通返回 unhealthy', async () => {
    expect(await checkTcp('127.0.0.1', 39996)).toBe('unhealthy')
  })
})

describe('checkHttp', () => {
  it('返回 200 且期望 200 返回 healthy', async () => {
    const server = http.createServer((req, res) => { res.statusCode = 200; res.end('ok') })
    await new Promise<void>((r) => server.listen(39995, r))
    expect(await checkHttp('127.0.0.1', 39995, '/', 200)).toBe('healthy')
    await new Promise<void>((r) => server.close(() => r()))
  })

  it('返回 500 期望 200 返回 unhealthy', async () => {
    const server = http.createServer((req, res) => { res.statusCode = 500; res.end('err') })
    await new Promise<void>((r) => server.listen(39994, r))
    expect(await checkHttp('127.0.0.1', 39994, '/', 200)).toBe('unhealthy')
    await new Promise<void>((r) => server.close(() => r()))
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
