import { useAsyncAction } from '@renderer/composables/useAsyncAction'

import { beforeEach, describe, expect, it, vi } from 'vitest'

// mock feedback.toast，避免依赖全局 vue ref 状态
const toastMock = vi.fn()
vi.mock('@renderer/feedback', () => ({
  toast: (...args: any[]) => toastMock(...args),
}))

describe('useAsyncAction', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('成功：返回 true，busy 翻转 true→false', async () => {
    const { busy, run } = useAsyncAction()
    expect(busy.value).toBe(false)
    const action = vi.fn().mockResolvedValue(undefined)
    const ok = await run(action, '失败')
    expect(ok).toBe(true)
    expect(action).toHaveBeenCalledOnce()
    expect(busy.value).toBe(false)
    expect(toastMock).not.toHaveBeenCalled()
  })

  it('失败：返回 false，busy 复位，toast 带前缀+message', async () => {
    const { busy, run } = useAsyncAction()
    const action = vi.fn().mockRejectedValue(new Error('网络断了'))
    const ok = await run(action, '保存失败')
    expect(ok).toBe(false)
    expect(busy.value).toBe(false)
    expect(toastMock).toHaveBeenCalledWith('保存失败: 网络断了', 'err')
  })

  it('失败但 e.message 缺失：message 回退到「未知错误」', async () => {
    const { run } = useAsyncAction()
    const action = vi.fn().mockRejectedValue('字符串错误无 message')
    await run(action, '操作失败')
    expect(toastMock).toHaveBeenCalledWith('操作失败: 未知错误', 'err')
  })

  it('failMsg 不传：失败静默不 toast，但仍返回 false', async () => {
    const { run } = useAsyncAction()
    const action = vi.fn().mockRejectedValue(new Error('x'))
    const ok = await run(action)
    expect(ok).toBe(false)
    expect(toastMock).not.toHaveBeenCalled()
  })

  it('busy 期间重复调用被拒绝（返回 false，不执行 action）', async () => {
    const { busy: _busy, run } = useAsyncAction()
    let resolveFirst!: () => void
    const firstAction = vi.fn(() => new Promise<void>((r) => { resolveFirst = r }))
    const secondAction = vi.fn().mockResolvedValue(undefined)
    const p1 = run(firstAction, 'A')
    // busy 此时为 true（firstAction 还没 resolve）
    const ok2 = await run(secondAction, 'B')
    expect(ok2).toBe(false)
    expect(secondAction).not.toHaveBeenCalled()
    resolveFirst()
    const ok1 = await p1
    expect(ok1).toBe(true)
  })

  it('busy 在 action 执行期间为 true', async () => {
    const { busy, run } = useAsyncAction()
    let observedBusy = false
    const action = vi.fn().mockImplementation(async () => {
      observedBusy = busy.value
    })
    await run(action, 'x')
    expect(observedBusy).toBe(true)
  })
})
