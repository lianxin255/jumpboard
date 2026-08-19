import type { EngineTestResult, Mapping } from '@ngj/shared/types'
import type { Mock } from 'vitest'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createSnapshot } from '../db/snapshotRepo'
import { applyMappings } from '../nginx/applyFlow'
import { logger } from '../services/logger'

vi.mock('../db/snapshotRepo', () => ({
  createSnapshot: vi.fn(),
  trimSnapshots: vi.fn(),
}))
vi.mock('../services/logger', () => ({
  logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}))

function mapping(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1',
    hostId: 'h1',
    name: 't',
    enabled: 1,
    protocol: 'tcp',
    listenPort: 9000,
    serverNames: null,
    listenPath: null,
    upstreamHost: '10.0.0.1',
    upstreamPort: 3306,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: null,
    healthCheckEnabled: 0,
    healthCheckIntervalSec: 30,
    healthCheckTargetPath: null,
    healthCheckExpectedStatus: null,
    healthStatus: 'unknown',
    lastCheckedAt: null,
    sortOrder: 0,
    createdAt: 0,
    updatedAt: 0,
    ...over,
  } as Mapping
}

function makeEngine(opts: {
  testResult?: EngineTestResult
  testMock?: (conf: string) => Promise<EngineTestResult>
  writeConf?: Mock
  reload?: Mock
  readConf?: Mock
} = {}) {
  return {
    engineKind: 'local' as const,
    supportsProcessControl: true,
    test: opts.testMock ?? vi.fn().mockResolvedValue(opts.testResult ?? { ok: true, output: 'syntax is ok' }),
    writeConf: opts.writeConf ?? vi.fn().mockResolvedValue(undefined),
    reload: opts.reload ?? vi.fn().mockResolvedValue(undefined),
    readConf: opts.readConf ?? vi.fn().mockResolvedValue('old conf'),
    start: vi.fn(),
    stop: vi.fn(),
    status: vi.fn().mockReturnValue('running'),
    refreshStatus: vi.fn().mockResolvedValue('running'),
  }
}

describe('applyMappings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('test 通过 → writeConf + reload', async () => {
    const eng = makeEngine()
    const result = await applyMappings(eng as any, [mapping()], {})
    expect(result.ok).toBe(true)
    expect(eng.writeConf).toHaveBeenCalledOnce()
    expect(eng.reload).toHaveBeenCalledOnce()
  })

  it('test 失败 → 不 reload，回滚到旧 conf', async () => {
    const eng = makeEngine({ testResult: { ok: false, output: 'invalid host in upstream' } })
    const result = await applyMappings(eng as any, [mapping()], {})
    expect(result.ok).toBe(false)
    expect(result.output).toContain('invalid host in upstream')
    // writeConf 被调用两次：一次写新 conf，一次回滚旧 conf
    expect(eng.writeConf).toHaveBeenCalledTimes(2)
    expect(eng.reload).not.toHaveBeenCalled()
  })

  it('test 通过但 reload 抛错 → 返回错误且回滚旧 conf（修复原 applyFlow 不回滚的 bug）', async () => {
    const eng = makeEngine({ reload: vi.fn().mockRejectedValue(new Error('reload failed')) })
    const result = await applyMappings(eng as any, [mapping()], {})
    expect(result.ok).toBe(false)
    expect(result.output).toContain('reload failed')
    // reload 失败也回滚磁盘 conf（safeApplyConf 新增的行为）
    expect(eng.writeConf).toHaveBeenCalledTimes(2)
    expect(eng.writeConf).toHaveBeenNthCalledWith(2, 'old conf')
  })

  it('生成的 conf 通过 result.conf 暴露（调试用）', async () => {
    const eng = makeEngine()
    const result = await applyMappings(eng as any, [mapping()], {})
    expect(result.conf).toContain('stream {')
    expect(result.conf).toContain('listen 9000;')
  })

  it('成功时存配置快照（hostId 提供时）', async () => {
    const eng = makeEngine()
    await applyMappings(eng as any, [mapping()], { hostId: 'h1' })
    expect(createSnapshot).toHaveBeenCalledWith(expect.objectContaining({ hostId: 'h1', reason: 'reload' }))
    expect(logger.info).toHaveBeenCalled()
  })

  it('校验失败时写 warn 日志不存快照', async () => {
    const eng = makeEngine({ testResult: { ok: false, output: 'bad config' } })
    await applyMappings(eng as any, [mapping()], { hostId: 'h1' })
    expect(logger.warn).toHaveBeenCalled()
    expect(createSnapshot).not.toHaveBeenCalled()
  })
})
