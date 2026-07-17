import { describe, it, expect, vi } from 'vitest'
import { applyMappings } from '@main/nginx/applyFlow'
import type { Mapping, EngineTestResult } from '@shared/types'

function mapping(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1', hostId: 'h1', name: 't', enabled: 1, protocol: 'tcp',
    listenPort: 9000, serverNames: null, listenPath: null,
    upstreamHost: '10.0.0.1', upstreamPort: 3306,
    tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: null,
    healthCheckEnabled: 0, healthCheckIntervalSec: 30,
    healthCheckTargetPath: null, healthCheckExpectedStatus: null,
    healthStatus: 'unknown', lastCheckedAt: null, sortOrder: 0,
    createdAt: 0, updatedAt: 0, ...over
  } as Mapping
}

function makeEngine(opts: {
  testResult?: EngineTestResult
  testMock?: (conf: string) => Promise<EngineTestResult>
  writeConf?: vi.Mock
  reload?: vi.Mock
  readConf?: vi.Mock
} = {}) {
  return {
    test: opts.testMock ?? vi.fn().mockResolvedValue(opts.testResult ?? { ok: true, output: 'syntax is ok' }),
    writeConf: opts.writeConf ?? vi.fn().mockResolvedValue(undefined),
    reload: opts.reload ?? vi.fn().mockResolvedValue(undefined),
    readConf: opts.readConf ?? vi.fn().mockResolvedValue('old conf'),
    start: vi.fn(),
    stop: vi.fn(),
    status: vi.fn().mockReturnValue('running')
  }
}

describe('applyMappings', () => {
  it('test 通过 → writeConf + reload', async () => {
    const eng = makeEngine()
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.ok).toBe(true)
    expect(eng.writeConf).toHaveBeenCalledOnce()
    expect(eng.reload).toHaveBeenCalledOnce()
  })

  it('test 失败 → 不 reload，回滚到旧 conf', async () => {
    const eng = makeEngine({ testResult: { ok: false, output: 'invalid host in upstream' } })
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.ok).toBe(false)
    expect(result.output).toContain('invalid host in upstream')
    // writeConf 被调用两次：一次写新 conf，一次回滚旧 conf
    expect(eng.writeConf).toHaveBeenCalledTimes(2)
    expect(eng.reload).not.toHaveBeenCalled()
  })

  it('test 通过但 reload 抛错 → 返回错误', async () => {
    const eng = makeEngine({ reload: vi.fn().mockRejectedValue(new Error('reload failed')) })
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.ok).toBe(false)
    expect(result.output).toContain('reload failed')
  })

  it('生成的 conf 通过 result.conf 暴露（调试用）', async () => {
    const eng = makeEngine()
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.conf).toContain('stream {')
    expect(result.conf).toContain('listen 9000;')
  })
})
