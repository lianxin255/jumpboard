import type { NginxEngine } from '../nginx/types'
import { describe, expect, it, vi } from 'vitest'
import { safeApplyConf } from '../nginx/safeApply'

/** 构造 mock engine，可覆盖任意方法 */
function makeEngine(over: Partial<NginxEngine> = {}): NginxEngine {
  const base: NginxEngine = {
    engineKind: 'local',
    supportsProcessControl: true,
    start: vi.fn(),
    stop: vi.fn(),
    reload: vi.fn(),
    test: vi.fn().mockResolvedValue({ ok: true, output: 'test passed' }),
    status: vi.fn().mockReturnValue('running'),
    refreshStatus: vi.fn().mockResolvedValue('running'),
    writeConf: vi.fn(),
    readConf: vi.fn().mockResolvedValue('OLD_CONF'),
    ...over,
  } as NginxEngine
  return base
}

describe('safeApplyConf', () => {
  it('test 通过 + reload 成功 → ok，writeConf 写新 conf 一次', async () => {
    const eng = makeEngine()
    const r = await safeApplyConf(eng, 'NEW_CONF')
    expect(r.ok).toBe(true)
    expect(r.output).toContain('test passed')
    expect(eng.writeConf).toHaveBeenCalledWith('NEW_CONF')
    expect(eng.writeConf).toHaveBeenCalledTimes(1) // 不回滚
    expect(eng.reload).toHaveBeenCalledOnce()
  })

  it('test 失败 → 回滚旧 conf，不 reload', async () => {
    const eng = makeEngine({
      test: vi.fn().mockResolvedValue({ ok: false, output: 'invalid host' }),
    })
    const r = await safeApplyConf(eng, 'NEW_CONF')
    expect(r.ok).toBe(false)
    expect(r.output).toContain('invalid host')
    expect(eng.writeConf).toHaveBeenCalledTimes(2) // 写新 + 回滚旧
    expect(eng.writeConf).toHaveBeenNthCalledWith(2, 'OLD_CONF')
    expect(eng.reload).not.toHaveBeenCalled()
  })

  it('test 通过但 reload 失败 → 回滚旧 conf（修复 applyFlow 不回滚的 bug）', async () => {
    const eng = makeEngine({
      reload: vi.fn().mockRejectedValue(new Error('reload failed')),
    })
    const r = await safeApplyConf(eng, 'NEW_CONF')
    expect(r.ok).toBe(false)
    expect(r.output).toContain('reload failed')
    expect(eng.writeConf).toHaveBeenCalledTimes(2) // 写新 + 回滚旧（这是新增的回滚）
    expect(eng.writeConf).toHaveBeenNthCalledWith(2, 'OLD_CONF')
  })

  it('readConf 抛错 → 返回 ok:false，不写 conf', async () => {
    const eng = makeEngine({
      readConf: vi.fn().mockRejectedValue(new Error('SSH disconnected')),
    })
    const r = await safeApplyConf(eng, 'NEW_CONF')
    expect(r.ok).toBe(false)
    expect(r.output).toContain('SSH disconnected')
    expect(eng.writeConf).not.toHaveBeenCalled()
  })

  it('writeConf 抛错 → 返回 ok:false', async () => {
    const eng = makeEngine({
      writeConf: vi.fn().mockRejectedValue(new Error('permission denied')),
    })
    const r = await safeApplyConf(eng, 'NEW_CONF')
    expect(r.ok).toBe(false)
    expect(r.output).toContain('permission denied')
  })

  it('test 抛错（非返回 false，而是 reject） → 返回 ok:false', async () => {
    const eng = makeEngine({
      test: vi.fn().mockRejectedValue(new Error('nginx -t crashed')),
    })
    const r = await safeApplyConf(eng, 'NEW_CONF')
    expect(r.ok).toBe(false)
    expect(r.output).toContain('nginx -t crashed')
  })

  it('并发调用串行化：两次 safeApplyConf 不交错（互斥锁）', async () => {
    // 用执行顺序标记验证串行：第一次 reload 期间第二次不应开始
    const order: string[] = []
    const eng = makeEngine({
      writeConf: vi.fn().mockImplementation(async (c: string) => {
        order.push(`write:${c}`)
      }),
      test: vi.fn().mockImplementation(async () => {
        order.push('test')
        return { ok: true, output: 'ok' }
      }),
      reload: vi.fn().mockImplementation(async () => {
        order.push('reload-start')
        await new Promise(r => setTimeout(r, 20))
        order.push('reload-end')
      }),
    })
    await Promise.all([
      safeApplyConf(eng, 'CONF_A'),
      safeApplyConf(eng, 'CONF_B'),
    ])
    // 两次完整流程不交错：A 的 reload-end 必须在 B 的 write 之前
    const aReloadEnd = order.indexOf('reload-end')
    const bWrite = order.indexOf('write:CONF_B')
    expect(aReloadEnd).toBeLessThan(bWrite)
    expect(aReloadEnd).toBeGreaterThan(-1)
  })
})
